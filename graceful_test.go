package graceful

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDefault(t *testing.T) {
	testRouterConstructor(t, func() (*Graceful, error) {
		return Default()
	}, "http://localhost:8080/example")
}

func TestWithAddr(t *testing.T) {
	testRouterConstructor(t, func() (*Graceful, error) {
		return Default(WithAddr(":8081"))
	}, "http://localhost:8081/example")
}

func TestCycle(t *testing.T) {
	router, err := Default()
	assert.NoError(t, err)
	assert.NotNil(t, router)

	router.GET("/example", func(c *gin.Context) { c.String(http.StatusOK, "it worked") })

	for i := 0; i < 10; i++ {
		ctxEnd, cancelEnd := context.WithCancel(context.Background())
		ctxService, cancelService := context.WithCancel(context.Background())

		go func(_ context.Context, cancelEnd context.CancelFunc) {
			assert.ErrorIs(t, router.RunWithContext(ctxService), context.Canceled)
			cancelEnd()
		}(ctxEnd, cancelEnd)

		testRequest(t, "http://localhost:8080/example")

		cancelService()
		<-ctxEnd.Done()
	}
}

func TestSimpleSignal(t *testing.T) {
	router, err := Default()
	assert.NoError(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	router.GET("/example", func(c *gin.Context) {
		time.Sleep(20 * time.Second)
		c.String(http.StatusOK, "it worked")
	})

	start := time.Now()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, router.RunWithContext(context.Background()))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Second)
		assert.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		testRequest(t, "http://localhost:8080/example")
	}()

	<-ctx.Done()

	assert.NoError(t, router.Shutdown(context.Background()))
	assert.GreaterOrEqual(t, time.Since(start).Seconds(), 20.0)

	wg.Wait()
}

func TestSimpleSleep(t *testing.T) {
	router, err := Default()
	assert.NoError(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	router.GET("/example", func(c *gin.Context) {
		time.Sleep(20 * time.Second)
		c.String(http.StatusOK, "it worked")
	})

	start := time.Now()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, router.RunWithContext(context.Background()))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		testRequest(t, "http://localhost:8080/example")
	}()

	time.Sleep(5 * time.Second)

	assert.NoError(t, router.Shutdown(context.Background()))
	assert.GreaterOrEqual(t, time.Since(start).Seconds(), 20.0)

	wg.Wait()
}

func TestSimpleCycle(t *testing.T) {
	router, err := Default()
	assert.NoError(t, err)
	assert.NotNil(t, router)

	router.GET("/example", func(c *gin.Context) { c.String(http.StatusOK, "it worked") })

	for i := 0; i < 10; i++ {
		assert.NoError(t, router.Start())
		testRequest(t, "http://localhost:8080/example")
		assert.NoError(t, router.Stop())
	}
}

func TestWithTLS(t *testing.T) {
	testRouterConstructor(t, func() (*Graceful, error) {
		return Default(WithTLS(":8443", "./testdata/certificate/cert.pem", "./testdata/certificate/key.pem"))
	}, "https://localhost:8443/example")
}

func TestWithFd(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	assert.NoError(t, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.NoError(t, err)
	defer listener.Close()
	socketFile, err := listener.File()
	if isWindows() {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	defer socketFile.Close()

	testRouterConstructor(t, func() (*Graceful, error) {
		return Default(WithFd(socketFile.Fd()))
	}, fmt.Sprintf("http://localhost:%d/example", listener.Addr().(*net.TCPAddr).Port))
}

func TestWithListener(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	assert.NoError(t, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.NoError(t, err)
	defer listener.Close()

	testRouterConstructor(t, func() (*Graceful, error) {
		return Default(WithListener(listener))
	}, fmt.Sprintf("http://localhost:%d/example", listener.Addr().(*net.TCPAddr).Port))
}

func TestWithServer(t *testing.T) {
	cert, err := tls.LoadX509KeyPair("./testdata/certificate/cert.pem", "./testdata/certificate/key.pem")
	assert.NoError(t, err)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	testRouterConstructor(t, func() (*Graceful, error) {
		return Default(
			WithServer(&http.Server{
				Addr:              ":8811",
				ReadHeaderTimeout: 10 * time.Second,
			}),
			WithServer(&http.Server{
				Addr:              ":9443",
				TLSConfig:         tlsConfig,
				ReadHeaderTimeout: 10 * time.Second,
			}),
		)
	}, "http://localhost:8811/example", "https://localhost:9443/example")
}

func TestWithAll(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	assert.NoError(t, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.NoError(t, err)
	defer listener.Close()

	testRouterConstructor(t, func() (*Graceful, error) {
		return Default(WithAddr(":8080"),
			WithTLS(":8443", "./testdata/certificate/cert.pem", "./testdata/certificate/key.pem"),
			WithListener(listener),
			WithServer(&http.Server{
				Addr:              ":8811",
				ReadHeaderTimeout: 10 * time.Second,
			}),
		)
	},
		"http://localhost:8080/example",
		"https://localhost:8443/example",
		fmt.Sprintf("http://localhost:%d/example", listener.Addr().(*net.TCPAddr).Port),
		"http://localhost:8811/example",
	)
}

func TestWithContext(t *testing.T) {
	router, err := Default()
	assert.Nil(t, err)
	assert.NotNil(t, router)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		router.GET("/example", func(c *gin.Context) { c.String(http.StatusOK, "it worked") })
		err := router.RunWithContext(ctx)
		if assert.Error(t, err) {
			assert.Equal(t, context.Canceled, err)
		}
	}()

	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)
	testRequest(t, "http://localhost:8080/example")

	cancel()
	<-ctx.Done()

	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/example", nil)
	assert.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{}}
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	assert.Error(t, err)

	err = router.Shutdown(context.Background())
	assert.NoError(t, err)

	router.Close()
}

func TestRunAddr(t *testing.T) {
	testRouterRun(t, func(g *Graceful) error {
		return g.Run(":8088")
	}, "http://localhost:8088/example")
}

func TestRunTLS(t *testing.T) {
	testRouterRun(t, func(g *Graceful) error {
		return g.RunTLS(":8443", "./testdata/certificate/cert.pem", "./testdata/certificate/key.pem")
	}, "https://localhost:8443/example")
}

func TestRunFd(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	assert.NoError(t, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.NoError(t, err)
	defer listener.Close()
	socketFile, err := listener.File()
	if isWindows() {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	defer socketFile.Close()

	testRouterRun(t, func(g *Graceful) error {
		return g.RunFd(socketFile.Fd())
	}, fmt.Sprintf("http://localhost:%d/example", listener.Addr().(*net.TCPAddr).Port))
}

func TestRunListener(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	assert.NoError(t, err)
	listener, err := net.ListenTCP("tcp", addr)
	assert.NoError(t, err)
	defer listener.Close()

	testRouterRun(t, func(g *Graceful) error {
		return g.RunListener(listener)
	}, fmt.Sprintf("http://localhost:%d/example", listener.Addr().(*net.TCPAddr).Port))
}

func TestWithUnix(t *testing.T) {
	unixTestSocket := filepath.Join(os.TempDir(), fmt.Sprintf("graceful-%d.sock", time.Now().UnixNano()))
	defer os.Remove(unixTestSocket)

	router, err := Default(WithUnix(unixTestSocket))
	assert.Nil(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	go func() {
		router.GET("/example", func(c *gin.Context) { c.String(http.StatusOK, "it worked") })
		assert.NoError(t, router.Run())
	}()

	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	var d net.Dialer
	c, err := d.DialContext(context.Background(), "unix", unixTestSocket)
	assert.NoError(t, err)

	fmt.Fprint(c, "GET /example HTTP/1.0\r\n\r\n")
	scanner := bufio.NewScanner(c)
	var response string
	for scanner.Scan() {
		response += scanner.Text()
	}
	assert.Contains(t, response, "HTTP/1.0 200", "should get a 200")
	assert.Contains(t, response, "it worked", "resp body should match")

	err = router.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestRunUnix(t *testing.T) {
	unixTestSocket := filepath.Join(os.TempDir(), fmt.Sprintf("graceful-%d.sock", time.Now().UnixNano()))
	defer os.Remove(unixTestSocket)

	router, err := Default()
	assert.Nil(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	go func() {
		router.GET("/example", func(c *gin.Context) { c.String(http.StatusOK, "it worked") })
		assert.NoError(t, router.RunUnix(unixTestSocket))
	}()

	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	var d net.Dialer
	c, err := d.DialContext(context.Background(), "unix", unixTestSocket)
	assert.NoError(t, err)

	fmt.Fprint(c, "GET /example HTTP/1.0\r\n\r\n")
	scanner := bufio.NewScanner(c)
	var response string
	for scanner.Scan() {
		response += scanner.Text()
	}
	assert.Contains(t, response, "HTTP/1.0 200", "should get a 200")
	assert.Contains(t, response, "it worked", "resp body should match")

	err = router.Shutdown(context.Background())
	assert.NoError(t, err)
}

func testRouterConstructor(t *testing.T, constructor func() (*Graceful, error), urls ...string) {
	router, err := constructor()
	assert.Nil(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		router.GET("/example", func(c *gin.Context) { c.String(http.StatusOK, "it worked") })
		assert.NoError(t, router.RunWithContext(ctx))
		cancel()
	}()

	testRequest(t, urls...)

	err = router.Shutdown(context.Background())
	assert.NoError(t, err)

	<-ctx.Done()
}

func testRouterRun(t *testing.T, run func(*Graceful) error, urls ...string) {
	router, err := Default()
	assert.NoError(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	go func() {
		router.GET("/example", func(c *gin.Context) { c.String(http.StatusOK, "it worked") })
		assert.NoError(t, run(router))
	}()

	testRequest(t, urls...)

	err = router.Shutdown(context.Background())
	assert.NoError(t, err)
}

func testRequest(t *testing.T, urls ...string) {
	// Open the PEM file
	file, err := os.Open("testdata/certificate/cert.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	// Create a new empty certificate pool
	caCertPool := x509.NewCertPool()

	// Load the PEM file into the certificate pool
	pemData, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	caCertPool.AppendCertsFromPEM(pemData)

	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	if len(urls) == 0 {
		t.Fatal("url cannot be empty")
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12, // Fix for G402: TLS MinVersion too low
		},
	}
	client := &http.Client{Transport: tr}

	for _, url := range urls {
		req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
		assert.NoError(t, err)
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		body, ioerr := io.ReadAll(resp.Body)
		assert.NoError(t, ioerr)

		responseStatus := "200 OK"
		responseBody := "it worked"

		assert.Equal(t, responseStatus, resp.Status, "should get a "+responseStatus)
		if responseStatus == "200 OK" {
			assert.Equal(t, responseBody, string(body), "resp body should match")
		}
	}
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func TestWithShutdownTimeout(t *testing.T) {
	// Test with custom timeout - verify it doesn't error
	customTimeout := 5 * time.Second
	router, err := Default(WithShutdownTimeout(customTimeout))
	assert.NoError(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	// Verify the timeout was set by checking the internal field
	assert.Equal(t, customTimeout, router.shutdownTimeout)

	// Test with default timeout
	router2, err := Default()
	assert.NoError(t, err)
	assert.NotNil(t, router2)
	defer router2.Close()

	// Should be zero, meaning it will use DefaultShutdownTimeout
	assert.Equal(t, time.Duration(0), router2.shutdownTimeout)
}

func TestShutdownTimeoutInAction(t *testing.T) {
	// Test that custom timeout can be set and basic shutdown works
	router, err := Default(WithShutdownTimeout(1 * time.Second))
	assert.NoError(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	router.GET("/example", func(c *gin.Context) {
		c.String(http.StatusOK, "it worked")
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = router.RunWithContext(context.Background())
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Make a quick request
	testRequest(t, "http://localhost:8080/example")

	// Shutdown should work normally with custom timeout
	err = router.Shutdown(context.Background())
	assert.NoError(t, err)

	wg.Wait()
}

func TestWithServerTimeouts(t *testing.T) {
	// Test with custom server timeouts
	customRead := 10 * time.Second
	customWrite := 20 * time.Second
	customIdle := 30 * time.Second

	router, err := Default(
		WithAddr(":8088"),
		WithServerTimeouts(customRead, customWrite, customIdle),
	)
	assert.NoError(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	// Verify the timeouts were set by checking the internal fields
	assert.Equal(t, customRead, router.readTimeout)
	assert.Equal(t, customWrite, router.writeTimeout)
	assert.Equal(t, customIdle, router.idleTimeout)

	// Start the server to verify it works with custom timeouts
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = router.RunWithContext(context.Background())
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify the server was created with correct timeouts
	router.lock.Lock()
	assert.Equal(t, 1, len(router.servers))
	srv := router.servers[0]
	assert.Equal(t, customRead, srv.ReadTimeout)
	assert.Equal(t, customWrite, srv.WriteTimeout)
	assert.Equal(t, customIdle, srv.IdleTimeout)
	assert.Equal(t, DefaultReadHeaderTimeout, srv.ReadHeaderTimeout)
	router.lock.Unlock()

	// Shutdown
	err = router.Shutdown(context.Background())
	assert.NoError(t, err)

	wg.Wait()
}

func TestDefaultServerTimeouts(t *testing.T) {
	// Test with default timeouts (no WithServerTimeouts option)
	router, err := Default(WithAddr(":8089"))
	assert.NoError(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	// Should be zero, meaning it will use default values
	assert.Equal(t, time.Duration(0), router.readTimeout)
	assert.Equal(t, time.Duration(0), router.writeTimeout)
	assert.Equal(t, time.Duration(0), router.idleTimeout)

	// Start the server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = router.RunWithContext(context.Background())
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify the server was created with default timeouts
	router.lock.Lock()
	assert.Equal(t, 1, len(router.servers))
	srv := router.servers[0]
	assert.Equal(t, DefaultReadTimeout, srv.ReadTimeout)
	assert.Equal(t, DefaultWriteTimeout, srv.WriteTimeout)
	assert.Equal(t, DefaultIdleTimeout, srv.IdleTimeout)
	assert.Equal(t, DefaultReadHeaderTimeout, srv.ReadHeaderTimeout)
	router.lock.Unlock()

	// Shutdown
	err = router.Shutdown(context.Background())
	assert.NoError(t, err)

	wg.Wait()
}

func TestPartialServerTimeouts(t *testing.T) {
	// Test with partial custom timeouts (some zero values)
	customRead := 25 * time.Second

	router, err := Default(
		WithAddr(":8090"),
		WithServerTimeouts(customRead, 0, 0), // Only set read timeout
	)
	assert.NoError(t, err)
	assert.NotNil(t, router)
	defer router.Close()

	// Verify only the read timeout was set
	assert.Equal(t, customRead, router.readTimeout)
	assert.Equal(t, time.Duration(0), router.writeTimeout)
	assert.Equal(t, time.Duration(0), router.idleTimeout)

	// Start the server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = router.RunWithContext(context.Background())
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify the server was created with correct timeouts
	router.lock.Lock()
	assert.Equal(t, 1, len(router.servers))
	srv := router.servers[0]
	assert.Equal(t, customRead, srv.ReadTimeout)
	assert.Equal(t, DefaultWriteTimeout, srv.WriteTimeout) // Should use default
	assert.Equal(t, DefaultIdleTimeout, srv.IdleTimeout)   // Should use default
	router.lock.Unlock()

	// Shutdown
	err = router.Shutdown(context.Background())
	assert.NoError(t, err)

	wg.Wait()
}
