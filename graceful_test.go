package graceful

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

		go func(ctx context.Context, cancel context.CancelFunc) {
			assert.ErrorIs(t, router.RunWithContext(ctxService), context.Canceled)
			cancelEnd()
		}(ctxEnd, cancelEnd)

		testRequest(t, "http://localhost:8080/example")

		cancelService()
		<-ctxEnd.Done()
	}
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
		)
	},
		"http://localhost:8080/example",
		"https://localhost:8443/example",
		fmt.Sprintf("http://localhost:%d/example", listener.Addr().(*net.TCPAddr).Port))
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

	client := &http.Client{Transport: &http.Transport{}}
	_, err = client.Get("http://localhost:8080/example")
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

	c, err := net.Dial("unix", unixTestSocket)
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

	c, err := net.Dial("unix", unixTestSocket)
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
	// have to wait for the goroutine to start and run the server
	// otherwise the main thread will complete
	time.Sleep(5 * time.Millisecond)

	if len(urls) == 0 {
		t.Fatal("url cannot be empty")
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}

	for _, url := range urls {
		resp, err := client.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		body, ioerr := io.ReadAll(resp.Body)
		assert.NoError(t, ioerr)

		var responseStatus = "200 OK"
		var responseBody = "it worked"

		assert.Equal(t, responseStatus, resp.Status, "should get a "+responseStatus)
		if responseStatus == "200 OK" {
			assert.Equal(t, responseBody, string(body), "resp body should match")
		}
	}
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}
