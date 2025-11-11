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

- **New(router *gin.Engine, opts ...Option) \*Graceful, error**:  
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

Various options (see code for `Option` interface/implementations) allow configuration of servers, including custom addresses, TLS, Unix sockets, file descriptors, and listeners.

---

## License

This project is licensed under the [MIT License](LICENSE).
