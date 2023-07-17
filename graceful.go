package graceful

import (
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// Graceful is a wrapper around a gin.Engine that provides graceful shutdown
type Graceful struct {
	*gin.Engine

	lock           sync.Mutex
	servers        []*http.Server
	listenAndServe []listenAndServe
	cleanup        []cleanup
}

type listenAndServe func() error
type cleanup func()

// Default returns a Graceful gin instance with the Logger and Recovery middleware already attached.
func Default(opts ...Option) (*Graceful, error) {
	return New(gin.Default(), opts...)
}

// New returns a Graceful gin instance from the given gin.Engine.
func New(router *gin.Engine, opts ...Option) (*Graceful, error) {
	g := &Graceful{
		Engine: router,
	}

	for _, o := range opts {
		if err := g.apply(o); err != nil {
			g.Close()
			return nil, err
		}
	}

	return g, nil
}

// Run attaches the router to a http.Server and starts listening and serving HTTP requests.
func (g *Graceful) Run(addr ...string) error {
	for _, a := range addr {
		if err := g.apply(WithAddr(a)); err != nil {
			return err
		}
	}

	return g.RunWithContext(context.Background())
}

// RunTLS attaches the router to a http.Server and starts listening and serving HTTPS (secure) requests.
func (g *Graceful) RunTLS(addr, certFile, keyFile string) error {
	if err := g.apply(WithTLS(addr, certFile, keyFile)); err != nil {
		return err
	}

	return g.RunWithContext(context.Background())
}

// RunUnix attaches the router to a http.Server and starts listening and serving HTTP requests
// through the specified unix socket (i.e. a file).
func (g *Graceful) RunUnix(file string) error {
	if err := g.apply(WithUnix(file)); err != nil {
		return err
	}

	return g.RunWithContext(context.Background())
}

// RunFd attaches the router to a http.Server and starts listening and serving HTTP requests
// through the specified file descriptor.
func (g *Graceful) RunFd(fd uintptr) error {
	if err := g.apply(WithFd(fd)); err != nil {
		return err
	}

	return g.RunWithContext(context.Background())
}

// RunListener attaches the router to a http.Server and starts listening and serving HTTP requests
// through the specified net.Listener
func (g *Graceful) RunListener(listener net.Listener) error {
	if err := g.apply(WithListener(listener)); err != nil {
		return err
	}

	return g.RunWithContext(context.Background())
}

// RunWithContext attaches the router to the configured http.Server (fallback to configuring one on
// :8080 if none are configured) and starts listening and serving HTTP requests. If the passed
// context is canceled, the server is gracefully shut down
func (g *Graceful) RunWithContext(ctx context.Context) error {
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		<-ctx.Done()
		g.Shutdown(ctx)
	}()
	defer cancel(nil)

	g.lock.Lock()

	if len(g.listenAndServe) == 0 {
		if err := g.apply(WithAddr(":8080")); err != nil {
			return err
		}
	}

	for _, srv := range g.listenAndServe {
		wg.Add(1)
		go func(srv listenAndServe) {
			defer wg.Done()
			if err := srv(); err != nil && err != http.ErrServerClosed {
				cancel(err)
				g.Shutdown(ctx)
			}
		}(srv)
	}
	g.lock.Unlock()

	wg.Wait()
	return ctx.Err()
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
func (g *Graceful) Shutdown(ctx context.Context) error {
	var err error

	g.lock.Lock()
	defer g.lock.Unlock()

	for _, srv := range g.servers {
		if e := srv.Shutdown(ctx); e != nil {
			e = err
		}
	}
	g.servers = nil

	return err
}

// Close shutdown the Graceful instance and close all underlying http.Servers.
func (g *Graceful) Close() {
	g.Shutdown(context.Background())

	g.lock.Lock()
	defer g.lock.Unlock()

	for _, c := range g.cleanup {
		c()
	}

	g.cleanup = nil
	g.listenAndServe = nil
	g.servers = nil
}

func (g *Graceful) apply(o Option) error {
	srv, cleanup, err := o.apply(g)
	if err != nil {
		return err
	}
	g.listenAndServe = append(g.listenAndServe, srv)
	g.cleanup = append(g.cleanup, cleanup)
	return nil
}

func (g *Graceful) appendHttpServer() *http.Server {
	srv := &http.Server{Handler: g.Engine}

	g.lock.Lock()
	defer g.lock.Unlock()
	g.servers = append(g.servers, srv)

	return srv
}

func donothing() {}
