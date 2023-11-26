# graceful

[![Run Tests](https://github.com/gin-contrib/graceful/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/gin-contrib/graceful/actions/workflows/go.yml)
[![codecov](https://codecov.io/gh/gin-contrib/graceful/branch/master/graph/badge.svg)](https://codecov.io/gh/gin-contrib/graceful)
[![Go Report Card](https://goreportcard.com/badge/github.com/gin-contrib/graceful)](https://goreportcard.com/report/github.com/gin-contrib/graceful)
[![GoDoc](https://godoc.org/github.com/gin-contrib/graceful?status.svg)](https://godoc.org/github.com/gin-contrib/graceful)

Gin wrapper to enable graceful termination when shutting down a process

## Example

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
