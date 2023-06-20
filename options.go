package graceful

import (
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

func (o optionFunc) apply(g *Graceful) (listenAndServe, cleanup, error) {
	return o(g)
}

// WithAddr configure a http.Server to listen on the given address.
func WithAddr(addr string) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		srv := &http.Server{Addr: addr, Handler: g.Engine}
		g.servers = append(g.servers, srv)

		return func() error {
			return srv.ListenAndServe()
		}, donothing, nil
	})
}

// WithTLS configure a http.Server to listen on the given address and serve HTTPS requests.
func WithTLS(addr string, certFile string, keyFile string) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		srv := &http.Server{Addr: addr, Handler: g.Engine}
		g.servers = append(g.servers, srv)

		return func() error {
			return srv.ListenAndServeTLS(certFile, keyFile)
		}, donothing, nil
	})
}

// WithUnix configure a http.Server to listen on the given unix socket file.
func WithUnix(file string) Option {
	return optionFunc(func(g *Graceful) (listenAndServe, cleanup, error) {
		listener, err := net.Listen("unix", file)
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
		f := os.NewFile(uintptr(fd), fmt.Sprintf("fd@%d", fd))
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
	srv := &http.Server{Handler: g.Engine}
	g.servers = append(g.servers, srv)

	return func() error {
			return srv.Serve(l)
		}, func() {
			close()
		}, nil
}
