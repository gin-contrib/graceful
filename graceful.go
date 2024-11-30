// graceful provides a wrapper around the gin.Engine to enable graceful shutdown of HTTP servers.
// It allows for starting, stopping, and shutting down servers with various configurations, such as
// listening on TCP addresses, Unix sockets, file descriptors, or custom net.Listeners.
package graceful

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// Graceful wraps a gin.Engine and provides methods to start, stop, and gracefully shut down HTTP servers.
type Graceful struct {
	*gin.Engine

	started context.Context
	stop    context.CancelFunc
	err     chan error

	lock           sync.Mutex
	servers        []*http.Server
	listenAndServe []listenAndServe
	cleanup        []cleanup
}

// ErrAlreadyStarted is returned when trying to start a router that has already been started.
var ErrAlreadyStarted = errors.New("already started router")

// ErrNotStarted is returned when trying to stop a router that has not been started.
var ErrNotStarted = errors.New("router not started")

// listenAndServe is a function type that starts an HTTP server and returns an error if it fails.
type listenAndServe func() error

// cleanup is a function type that performs cleanup operations.
type cleanup func()

var donothing cleanup = func() {}

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

// Run attaches the router to an http.Server and starts listening and serving HTTP requests.
func (g *Graceful) Run(addr ...string) error {
	for _, a := range addr {
		if err := g.apply(WithAddr(a)); err != nil {
			return err
		}
	}

	return g.RunWithContext(context.Background())
}

// RunTLS attaches the router to an http.Server and starts listening and serving HTTPS (secure) requests.
func (g *Graceful) RunTLS(addr, certFile, keyFile string) error {
	if err := g.apply(WithTLS(addr, certFile, keyFile)); err != nil {
		return err
	}

	return g.RunWithContext(context.Background())
}

// RunUnix attaches the router to an http.Server and starts listening and serving HTTP requests
// through the specified Unix socket (i.e., a file).
func (g *Graceful) RunUnix(file string) error {
	if err := g.apply(WithUnix(file)); err != nil {
		return err
	}

	return g.RunWithContext(context.Background())
}

// RunFd attaches the router to an http.Server and starts listening and serving HTTP requests
// through the specified file descriptor.
func (g *Graceful) RunFd(fd uintptr) error {
	if err := g.apply(WithFd(fd)); err != nil {
		return err
	}

	return g.RunWithContext(context.Background())
}

// RunListener attaches the router to an http.Server and starts listening and serving HTTP requests
// through the specified net.Listener.
func (g *Graceful) RunListener(listener net.Listener) error {
	if err := g.apply(WithListener(listener)); err != nil {
		return err
	}

	return g.RunWithContext(context.Background())
}

// RunWithContext attaches the router to the configured http.Server (fallback to configuring one on
// :8080 if none are configured) and starts listening and serving HTTP requests. If the passed
// context is canceled, the server is gracefully shut down.
func (g *Graceful) RunWithContext(ctx context.Context) error {
	if err := g.ensureAtLeastDefaultServer(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		_ = g.Shutdown(ctx)
	}()
	defer cancel()

	eg := errgroup.Group{}

	g.lock.Lock()

	for _, srv := range g.listenAndServe {
		safeCopy := srv
		eg.Go(func() error {
			if err := safeCopy(); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		})
	}

	g.lock.Unlock()

	if err := waitWithContext(ctx, &eg); err != nil {
		return err
	}
	return g.Shutdown(ctx)
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
func (g *Graceful) Shutdown(ctx context.Context) error {
	var err error

	g.lock.Lock()
	defer g.lock.Unlock()

	for _, srv := range g.servers {
		if e := srv.Shutdown(ctx); e != nil {
			err = e
		}
	}
	g.servers = nil

	return err
}

// Start will start the Graceful instance and all underlying http.Servers in a separate
// goroutine and return right away. You must call Stop and not Shutdown if you use Start.
func (g *Graceful) Start() error {
	g.lock.Lock()
	defer g.lock.Unlock()

	if g.started != nil {
		return ErrAlreadyStarted
	}

	g.err = make(chan error)
	ctxStarted, cancel := context.WithCancel(context.Background())
	ctx, cancelStop := context.WithCancel(context.Background())
	go func() {
		err := g.RunWithContext(ctx)
		cancel()
		g.err <- err
	}()

	g.stop = cancelStop
	g.started = ctxStarted

	return nil
}

// Stop will stop the Graceful instance previously started with Start. It
// will return once the instance has been stopped.
func (g *Graceful) Stop() error {
	resetStartedState := func() (context.Context, context.CancelFunc, chan error, error) {
		g.lock.Lock()
		defer g.lock.Unlock()

		if g.started == nil {
			return nil, nil, nil, ErrNotStarted
		}

		stop := g.stop
		started := g.started
		chErr := g.err
		g.stop = nil
		g.started = nil

		return started, stop, chErr, nil
	}
	started, stop, chErr, err := resetStartedState()
	if err != nil {
		return err
	}

	stop()
	err = <-chErr
	<-started.Done()

	if !errors.Is(err, context.Canceled) {
		return err
	}

	err = started.Err()
	if errors.Is(err, context.Canceled) {
		err = nil
	}

	return err
}

// Close gracefully shuts down the server.
// It first shuts down the server using the Shutdown method,
// then it performs any cleanup operations registered with the server.
// Finally, it resets the server's internal state.
func (g *Graceful) Close() {
	_ = g.Shutdown(context.Background())

	g.lock.Lock()
	defer g.lock.Unlock()

	for _, c := range g.cleanup {
		c()
	}

	g.cleanup = nil
	g.listenAndServe = nil
	g.servers = nil
}

// apply applies the given option to the Graceful instance.
// It creates a new server, applies the option to it, and adds the server and cleanup function to the Graceful instance.
// If an error occurs during the application of the option, it returns the error.
func (g *Graceful) apply(o Option) error {
	srv, cleanup, err := o.apply(g)
	if err != nil {
		return err
	}
	g.listenAndServe = append(g.listenAndServe, srv)
	g.cleanup = append(g.cleanup, cleanup)
	return nil
}

// appendHTTPServer appends a new HTTP server to the list of servers managed by the Graceful instance.
// It returns the newly created http.Server.
func (g *Graceful) appendHTTPServer() *http.Server {
	srv := &http.Server{
		Handler:           g.Engine,
		ReadHeaderTimeout: time.Second * 5, // Set a reasonable ReadHeaderTimeout value
	}

	g.lock.Lock()
	defer g.lock.Unlock()
	g.servers = append(g.servers, srv)

	return srv
}

// appendExistHTTPServer appends an existing HTTP server to the list of servers managed by the Graceful instance.
// This allows for customization of the http.Server, and srv.Handler will be set to the current g.Engine.
func (g *Graceful) appendExistHTTPServer(srv *http.Server) {
	srv.Handler = g.Engine

	g.lock.Lock()
	defer g.lock.Unlock()
	g.servers = append(g.servers, srv)
}

// ensureAtLeastDefaultServer ensures that there is at least one server running with the default address ":8080".
// If no server is running, it creates a new server with the default address and starts it.
// It returns an error if there was a problem creating or starting the server.
func (g *Graceful) ensureAtLeastDefaultServer() error {
	g.lock.Lock()
	defer g.lock.Unlock()

	if len(g.listenAndServe) == 0 {
		if err := g.apply(WithAddr(":8080")); err != nil {
			return err
		}
	}
	return nil
}

// waitWithContext waits for the completion of the errgroup.Group and returns any error encountered.
// If the context is canceled before the errgroup.Group completes, it returns the context error.
// If the errgroup.Group completes successfully or the context is not canceled, it returns nil.
func waitWithContext(ctx context.Context, eg *errgroup.Group) error {
	if err := eg.Wait(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
