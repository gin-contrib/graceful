package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/graceful"
	"github.com/gin-gonic/gin"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create a graceful router with custom timeouts
	// - ShutdownTimeout: wait up to 10 seconds for active connections to close during shutdown
	// - ServerTimeouts: configure HTTP server timeouts
	//   * ReadTimeout: 10 seconds - complete request read timeout including body
	//   * WriteTimeout: 15 seconds - response write timeout
	//   * IdleTimeout: 30 seconds - keep-alive idle connection timeout
	router, err := graceful.Default(
		graceful.WithShutdownTimeout(10*time.Second),
		graceful.WithServerTimeouts(10*time.Second, 15*time.Second, 30*time.Second),
	)
	if err != nil {
		panic(err)
	}
	defer router.Close()

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Welcome Gin Server")
	})

	router.GET("/slow", func(c *gin.Context) {
		// Simulate a slow endpoint that takes 5 seconds
		time.Sleep(5 * time.Second)
		c.String(http.StatusOK, "Slow response completed")
	})

	go func() {
		if err := router.RunWithContext(context.Background()); err != nil && err != context.Canceled {
			panic(err)
		}
	}()

	<-ctx.Done()

	// The shutdown will use the configured 10-second timeout
	// to wait for active connections (like /slow) to complete
	if err := router.Shutdown(context.Background()); err != nil {
		panic(err)
	}
}
