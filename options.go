package graceful

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
)

// Option specifies instrumentation configuration options.
type Option interface {
	apply(*Graceful) (listenAndServe, cleanup, error)
}

var _ Option = (*optionFunc)(nil)

type optionFunc func(*Graceful) (listenAndServe, cleanup, error)

// apply applies the option function to the Graceful instance.
// It returns the listenAndServe function, cleanup function, and an error, if any.
func (o optionFunc) apply(g *Graceful) (listenAndServe, cleanup, error) {
	return o(g)
}

// WithAddr configure a http.Server to listen on the given address.
func WithAddr(addr string) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		return func() error {
			srv := g.appendHTTPServer()
			srv.Addr = addr

			return srv.ListenAndServe()
		}, donothing, nil
	})
}

// WithTLS configure a http.Server to listen on the given address and serve HTTPS requests.
func WithTLS(addr string, certFile string, keyFile string) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		return func() error {
			srv := g.appendHTTPServer()
			srv.Addr = addr
			g.lock.Lock()
			g.servers = append(g.servers, srv)
			g.lock.Unlock()

			return srv.ListenAndServeTLS(certFile, keyFile)
		}, donothing, nil
	})
}

// WithServer configure an existing http.Server to serve HTTP or HTTPS requests.
// This allows for a more complete customization of the http.Server,
// and srv Handler will be set to the current gin.Engine.
// If srv contains TLSConfig, ListenAndServeTLS will be used;
// otherwise, ListenAndServe will be used.
func WithServer(srv *http.Server) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		if srv == nil {
			return nil, donothing, errors.New("nil http server")
		}
		return func() error {
			g.appendExistHTTPServer(srv)
			if srv.TLSConfig == nil {
				return srv.ListenAndServe()
			} else {
				return srv.ListenAndServeTLS("", "")
			}
		}, donothing, nil
	})
}

// WithUnix configure a http.Server to listen on the given unix socket file.
func WithUnix(file string) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		var lc net.ListenConfig
		listener, err := lc.Listen(context.Background(), "unix", file)
		if err != nil {
			return nil, donothing, err
		}

		return listen(g, listener, func() {
			os.Remove(file)
			listener.Close()
		})
	})
}

// WithFd configure a http.Server to listen on the given file descriptor.
func WithFd(fd uintptr) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		f := os.NewFile(fd, fmt.Sprintf("fd@%d", fd))
		listener, err := net.FileListener(f)
		if err != nil {
			return nil, donothing, err
		}

		return listen(g, listener, func() {
			listener.Close()
			f.Close()
		})
	})
}

// WithListener configure a http.Server to listen on the given net.Listener.
func WithListener(l net.Listener) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		return listen(g, l, donothing)
	})
}

func listen(g *Graceful, l net.Listener, close cleanup) (listenAndServe, cleanup, error) {
	return func() error {
			srv := g.appendHTTPServer()

			return srv.Serve(l)
		}, func() {
			close()
		}, nil
}
