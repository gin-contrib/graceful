package main

import (
	"context"
	"log"
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

	// Create router with lifecycle hooks
	router, err := graceful.Default(
		// BeforeShutdown hooks - called before server shutdown begins
		graceful.WithBeforeShutdown(func(ctx context.Context) error {
			log.Println("HOOK: Before shutdown - Notifying load balancer...")
			// Simulate notifying load balancer to stop sending traffic
			time.Sleep(100 * time.Millisecond)
			log.Println("HOOK: Load balancer notified")
			return nil
		}),
		graceful.WithBeforeShutdown(func(ctx context.Context) error {
			log.Println("HOOK: Before shutdown - Stopping background workers...")
			// Simulate stopping background workers
			time.Sleep(50 * time.Millisecond)
			log.Println("HOOK: Background workers stopped")
			return nil
		}),

		// AfterShutdown hooks - called after all servers have shut down
		graceful.WithAfterShutdown(func(ctx context.Context) error {
			log.Println("HOOK: After shutdown - Closing database connections...")
			// Simulate closing database connections
			time.Sleep(100 * time.Millisecond)
			log.Println("HOOK: Database connections closed")
			return nil
		}),
		graceful.WithAfterShutdown(func(ctx context.Context) error {
			log.Println("HOOK: After shutdown - Flushing metrics...")
			// Simulate flushing metrics to external service
			time.Sleep(50 * time.Millisecond)
			log.Println("HOOK: Metrics flushed")
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	defer router.Close()

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Welcome! Try stopping the server with Ctrl+C to see lifecycle hooks in action.")
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	log.Println("Server starting on :8080")
	log.Println("Send SIGTERM (Ctrl+C) to trigger graceful shutdown with lifecycle hooks")

	go func() {
		if err := router.RunWithContext(context.Background()); err != nil && err != context.Canceled {
			panic(err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutdown signal received, starting graceful shutdown...")

	if err := router.Shutdown(context.Background()); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
