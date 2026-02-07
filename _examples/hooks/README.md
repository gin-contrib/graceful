# Lifecycle Hooks Example

This example demonstrates how to use lifecycle hooks (`WithBeforeShutdown` and `WithAfterShutdown`) to execute custom logic during graceful shutdown.

## Usage

```bash
go run main.go
```

Then send a request:

```bash
curl http://localhost:8080/
```

To see the hooks in action, stop the server with `Ctrl+C` (SIGTERM).

## Expected Output

When you stop the server, you should see output like:

```txt
Shutdown signal received, starting graceful shutdown...
HOOK: Before shutdown - Notifying load balancer...
HOOK: Load balancer notified
HOOK: Before shutdown - Stopping background workers...
HOOK: Background workers stopped
HOOK: After shutdown - Closing database connections...
HOOK: Database connections closed
HOOK: After shutdown - Flushing metrics...
HOOK: Metrics flushed
Server stopped gracefully
```

## Hook Execution Order

1. **BeforeShutdown hooks** - Execute in registration order before server shutdown begins
   - Notify load balancer to stop sending traffic
   - Stop background workers

2. **Server Shutdown** - All HTTP servers shut down gracefully

3. **AfterShutdown hooks** - Execute in registration order after servers have stopped
   - Close database connections
   - Flush metrics to external service

## Use Cases

- **BeforeShutdown**: Deregister from service discovery, notify load balancers, stop accepting new work
- **AfterShutdown**: Close database connections, flush buffers, cleanup resources, send final metrics
