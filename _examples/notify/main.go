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

	go func() {
		if err := router.RunWithContext(context.Background()); err != nil && err != context.Canceled {
			panic(err)
		}
	}()

	<-ctx.Done()

	if err := router.Shutdown(context.Background()); err != nil {
		panic(err)
	}
}
