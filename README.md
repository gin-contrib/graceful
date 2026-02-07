# graceful

English | [繁體中文](README.zh-tw.md) | [简体中文](README.zh-cn.md)

![Run Tests](https://github.com/gin-contrib/graceful/actions/workflows/go.yml/badge.svg?branch=master)
[![Trivy Security Scan](https://github.com/gin-contrib/graceful/actions/workflows/trivy-scan.yml/badge.svg)](https://github.com/gin-contrib/graceful/actions/workflows/trivy-scan.yml)
![codecov](https://codecov.io/gh/gin-contrib/graceful/branch/master/graph/badge.svg)
![Go Report Card](https://goreportcard.com/badge/github.com/gin-contrib/graceful)
![GoDoc](https://godoc.org/github.com/gin-contrib/graceful?status.svg)

**graceful** is a wrapper for Gin that provides robust and flexible graceful shutdown capabilities for HTTP servers. It allows you to start, stop, and smoothly shut down servers, supporting various listen mechanisms including TCP, Unix sockets, file descriptors, or custom listeners.

- [graceful](#graceful)
  - [Features](#features)
  - [Installation](#installation)
  - [Usage Example](#usage-example)
  - [API Overview](#api-overview)
    - [Graceful Type](#graceful-type)
    - [Server Start Methods](#server-start-methods)
    - [Shutdown and Cleanup Methods](#shutdown-and-cleanup-methods)
    - [Options](#options)
  - [Lifecycle Hooks Example](#lifecycle-hooks-example)
  - [License](#license)

## Features

- Seamless graceful shutdown for Gin HTTP servers
- Supports TCP, TLS, Unix sockets, file descriptors, and custom net.Listener
- Thread-safe start/stop routines and context-based cancellation
- Ensures all active connections are properly handled before terminating
- Simple API with customizable options

## Installation

```bash
go get github.com/gin-contrib/graceful
```

## Usage Example

```go
package main

import (
  "context"
  "net/http"
  "os/signal"
  "syscall"

  "github.com/gin-contrib/graceful"
  "github.com/gin-gonic/gin"
)

func main() {
  ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
  defer stop()

  router, err := graceful.Default()
  if err != nil {
    panic(err)
  }
  defer router.Close()

  router.GET("/", func(c *gin.Context) {
    c.String(http.StatusOK, "Welcome Gin Server")
  })

  if err := router.RunWithContext(ctx); err != nil && err != context.Canceled {
    panic(err)
  }
}
```

## API Overview

### Graceful Type

The main type is:

```go
type Graceful struct {
  *gin.Engine
  // Internal synchronization/context fields
}
```

### Server Start Methods

- **Default(opts ...Option) \*Graceful, error**:  
  Creates a `Graceful` instance with default Gin middleware.

- **New(router \*gin.Engine, opts ...Option) \*Graceful, error**:  
  Wraps an existing `gin.Engine`.

- **Run(addr ...string) error**:  
  Starts HTTP server(s) on TCP address(es).

- **RunTLS(addr, certFile, keyFile string) error**:  
  Starts HTTPS server on the given address.

- **RunUnix(file string) error**:  
  Starts server on the specified Unix socket.

- **RunFd(fd uintptr) error**:  
  Starts server using the provided file descriptor.

- **RunListener(listener net.Listener) error**:  
  Starts server on a custom `net.Listener`.

- **RunWithContext(ctx context.Context) error**:  
  Starts server(s) with lifecycle bound to a context. Recommended for handling shutdown signals.

### Shutdown and Cleanup Methods

- **Shutdown(ctx context.Context) error**:  
  Gracefully shuts down server(s), waiting for active connections.

- **Start() error**:  
  Starts server(s) in a goroutine. You must call `Stop()` to terminate.

- **Stop() error**:  
  Stops a running Graceful instance previously started with `Start()`.

- **Close()**:  
  Cleans up internal state and shuts down any running server(s).

### Options

Various options allow configuration of servers:

- **WithAddr(addr string)**:
  Configure HTTP server to listen on the given TCP address.

- **WithTLS(addr, certFile, keyFile string)**:
  Configure HTTPS server with TLS certificates.

- **WithUnix(file string)**:
  Configure server to listen on Unix socket.

- **WithFd(fd uintptr)**:
  Configure server to listen on file descriptor.

- **WithListener(listener net.Listener)**:
  Configure server to use custom listener.

- **WithServer(srv \*http.Server)**:
  Configure with an existing `http.Server` for full customization.

- **WithShutdownTimeout(timeout time.Duration)**:
  Configure graceful shutdown timeout (default: 30 seconds).

- **WithServerTimeouts(readTimeout, writeTimeout, idleTimeout time.Duration)**:
  Configure HTTP server timeouts:
  - `ReadTimeout`: Complete request read timeout including body (default: 15 seconds)
  - `WriteTimeout`: Response write timeout (default: 30 seconds)
  - `IdleTimeout`: Keep-alive idle connection timeout (default: 60 seconds)

- **WithBeforeShutdown(hook Hook)**:
  Register a hook to be called before server shutdown begins. Multiple hooks can be registered and will execute in registration order. Hooks receive the shutdown context and can return errors, which are collected but do not prevent shutdown from proceeding.

- **WithAfterShutdown(hook Hook)**:
  Register a hook to be called after all servers have shut down. Multiple hooks can be registered and will execute in registration order. Hooks receive the shutdown context and can return errors, which are collected and returned but do not affect server shutdown.

Example with custom timeouts:

```go
router, err := graceful.Default(
  graceful.WithShutdownTimeout(10 * time.Second),
  graceful.WithServerTimeouts(10*time.Second, 15*time.Second, 30*time.Second),
)
```

## Lifecycle Hooks Example

Lifecycle hooks allow you to execute custom logic during graceful shutdown. Use `WithBeforeShutdown` for cleanup that should happen before servers stop (like deregistering from service discovery), and `WithAfterShutdown` for cleanup after servers have stopped (like closing database connections).

```go
package main

import (
  "context"
  "log"
  "net/http"
  "os/signal"
  "syscall"

  "github.com/gin-contrib/graceful"
  "github.com/gin-gonic/gin"
)

func main() {
  ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
  defer stop()

  router, err := graceful.Default(
    // BeforeShutdown: called before server shutdown begins
    graceful.WithBeforeShutdown(func(ctx context.Context) error {
      log.Println("Notifying load balancer...")
      // Deregister from load balancer
      return nil
    }),
    graceful.WithBeforeShutdown(func(ctx context.Context) error {
      log.Println("Stopping background workers...")
      // Stop accepting new background jobs
      return nil
    }),

    // AfterShutdown: called after all servers have shut down
    graceful.WithAfterShutdown(func(ctx context.Context) error {
      log.Println("Closing database connections...")
      // Close database pool
      return nil
    }),
    graceful.WithAfterShutdown(func(ctx context.Context) error {
      log.Println("Flushing metrics...")
      // Send final metrics
      return nil
    }),
  )
  if err != nil {
    panic(err)
  }
  defer router.Close()

  router.GET("/", func(c *gin.Context) {
    c.String(http.StatusOK, "Welcome Gin Server")
  })

  if err := router.RunWithContext(ctx); err != nil && err != context.Canceled {
    panic(err)
  }
}
```

When shutdown is triggered, hooks execute in this order:

1. All `BeforeShutdown` hooks (in registration order)
2. Server graceful shutdown
3. All `AfterShutdown` hooks (in registration order)

See the [hooks example](_examples/hooks) for a complete working example.

---

## License

This project is licensed under the [MIT License](LICENSE).
