## RPC Middleware System

## Overview

The RPC client now supports custom middleware injection, allowing developers to intercept, modify, and observe HTTP requests and responses. This enables powerful customization patterns including logging, metrics collection, rate limiting, circuit breaking, and custom header injection.

## Architecture

### Middleware Pattern

Middleware follows the standard HTTP middleware pattern where each middleware wraps an `http.RoundTripper`:

```go
type Middleware func(http.RoundTripper) http.RoundTripper
```

Middleware can:
- Inspect and modify requests before they are sent
- Inspect and modify responses before they are returned
- Short-circuit requests (e.g., circuit breaker, cache)
- Collect metrics and logs
- Add authentication headers
- Implement retry logic

### Middleware Chain

Multiple middlewares can be composed into a chain. The chain applies middleware in order, with the first middleware being the outermost layer:

```
Request → Middleware1 → Middleware2 → Middleware3 → HTTP Transport → Network
Response ← Middleware1 ← Middleware2 ← Middleware3 ← HTTP Transport ← Network
```

## Usage

### Basic Client Creation with Middleware

```go
import "github.com/dotandev/hintents/internal/rpc"

// Create client with single middleware
client, err := rpc.NewClient(
    rpc.WithNetwork(rpc.Testnet),
    rpc.WithMiddleware(rpc.LoggingMiddleware()),
)

// Create client with multiple middlewares
client, err := rpc.NewClient(
    rpc.WithNetwork(rpc.Testnet),
    rpc.WithMiddlewares(
        rpc.LoggingMiddleware(),
        rpc.HeaderMiddleware(map[string]string{
            "X-API-Key": "your-api-key",
        }),
        rpc.MetricsMiddleware(metricsCollector),
    ),
)
```

### Functional Options Pattern

The client uses functional options for configuration:

```go
client, err := rpc.NewClient(
    rpc.WithNetwork(rpc.Mainnet),
    rpc.WithToken("auth-token"),
    rpc.WithCache(true),
    rpc.WithAltURLs([]string{
        "https://rpc1.example.com",
        "https://rpc2.example.com",
    }),
    rpc.WithMiddleware(rpc.LoggingMiddleware()),
    rpc.WithRetryConfig(rpc.RetryConfig{
        MaxRetries: 5,
        InitialBackoff: 2 * time.Second,
    }),
)
```

## Built-in Middleware

### LoggingMiddleware

Logs all HTTP requests and responses with timing information.

```go
middleware := rpc.LoggingMiddleware()
```

**Features:**
- Logs request method, URL, and start time
- Logs response status code and duration
- Logs errors with context

**Use Cases:**
- Debugging RPC communication
- Monitoring request patterns
- Troubleshooting failures

### HeaderMiddleware

Adds custom headers to all requests.

```go
middleware := rpc.HeaderMiddleware(map[string]string{
    "X-API-Key": "secret-key",
    "X-Client-Version": "1.0.0",
    "X-Request-ID": uuid.New().String(),
})
```

**Features:**
- Adds headers to every request
- Does not modify original request
- Supports multiple headers

**Use Cases:**
- API key authentication
- Request tracking
- Client identification

### MetricsMiddleware

Collects request metrics for monitoring and observability.

```go
type MyMetricsCollector struct{}

func (m *MyMetricsCollector) RecordRequest(method, url string, statusCode int, duration time.Duration, err error) {
    // Record metrics to your monitoring system
}

middleware := rpc.MetricsMiddleware(&MyMetricsCollector{})
```

**Features:**
- Records method, URL, status code, duration
- Captures errors
- Thread-safe

**Use Cases:**
- Prometheus metrics
- Application monitoring
- Performance analysis
- SLA tracking

### TimeoutMiddleware

Enforces request timeouts.

```go
middleware := rpc.TimeoutMiddleware(30 * time.Second)
```

**Features:**
- Per-request timeout enforcement
- Context-based cancellation
- Prevents hanging requests

**Use Cases:**
- Preventing slow requests
- Resource management
- SLA enforcement

### RateLimitMiddleware

Implements rate limiting for outgoing requests.

```go
type MyRateLimiter struct {
    limiter *rate.Limiter
}

func (r *MyRateLimiter) Wait(ctx context.Context) error {
    return r.limiter.Wait(ctx)
}

middleware := rpc.RateLimitMiddleware(&MyRateLimiter{
    limiter: rate.NewLimiter(rate.Limit(10), 1), // 10 requests per second
})
```

**Features:**
- Prevents exceeding rate limits
- Context-aware waiting
- Configurable limits

**Use Cases:**
- Respecting API rate limits
- Preventing 429 errors
- Resource conservation

### CircuitBreakerMiddleware

Implements circuit breaker pattern to prevent cascading failures.

```go
type MyCircuitBreaker struct {
    failures int
    threshold int
}

func (cb *MyCircuitBreaker) Allow() bool {
    return cb.failures < cb.threshold
}

func (cb *MyCircuitBreaker) RecordSuccess() {
    cb.failures = 0
}

func (cb *MyCircuitBreaker) RecordFailure() {
    cb.failures++
}

middleware := rpc.CircuitBreakerMiddleware(&MyCircuitBreaker{
    threshold: 5,
})
```

**Features:**
- Prevents requests when circuit is open
- Records successes and failures
- Automatic recovery

**Use Cases:**
- Preventing cascading failures
- Fast-fail behavior
- Service degradation

## Custom Middleware

### Creating Custom Middleware

```go
func CustomMiddleware() rpc.Middleware {
    return func(next http.RoundTripper) http.RoundTripper {
        return &customTransport{next: next}
    }
}

type customTransport struct {
    next http.RoundTripper
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Before request
    logger.Info("Custom middleware: before request")
    
    // Modify request if needed
    req.Header.Set("X-Custom", "value")
    
    // Execute request
    resp, err := t.next.RoundTrip(req)
    
    // After request
    if err != nil {
        logger.Error("Custom middleware: request failed", "error", err)
        return resp, err
    }
    
    logger.Info("Custom middleware: after request", "status", resp.StatusCode)
    return resp, nil
}
```

### Example: Request ID Middleware

```go
func RequestIDMiddleware() rpc.Middleware {
    return func(next http.RoundTripper) http.RoundTripper {
        return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
            requestID := uuid.New().String()
            req.Header.Set("X-Request-ID", requestID)
            
            logger.Info("Request started", "request_id", requestID)
            resp, err := next.RoundTrip(req)
            logger.Info("Request completed", "request_id", requestID)
            
            return resp, err
        })
    }
}
```

### Example: Retry with Exponential Backoff

```go
func RetryMiddleware(maxRetries int) rpc.Middleware {
    return func(next http.RoundTripper) http.RoundTripper {
        return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
            var lastErr error
            backoff := time.Second
            
            for attempt := 0; attempt <= maxRetries; attempt++ {
                if attempt > 0 {
                    time.Sleep(backoff)
                    backoff *= 2
                }
                
                resp, err := next.RoundTrip(req)
                if err == nil && resp.StatusCode < 500 {
                    return resp, nil
                }
                
                lastErr = err
            }
            
            return nil, lastErr
        })
    }
}
```

## Performance

### Overhead Analysis

Middleware adds minimal overhead to requests:

```
BenchmarkMiddlewareOverhead_NoMiddleware-8        1000000    1.2 µs/op
BenchmarkMiddlewareOverhead_WithMiddleware-8       800000    1.5 µs/op
```

Overhead per middleware:
- LoggingMiddleware: ~0.1 µs
- HeaderMiddleware: ~0.05 µs
- MetricsMiddleware: ~0.1 µs
- TimeoutMiddleware: ~0.05 µs

### Best Practices

1. **Order Matters**: Place fast middleware first
   ```go
   // Good: Fast middleware first
   WithMiddlewares(
       HeaderMiddleware(...),      // Fast
       LoggingMiddleware(),        // Medium
       MetricsMiddleware(...),     // Medium
       CircuitBreakerMiddleware(...), // Slow
   )
   ```

2. **Avoid Heavy Operations**: Keep middleware lightweight
   ```go
   // Bad: Heavy computation in middleware
   func BadMiddleware() Middleware {
       return func(next http.RoundTripper) http.RoundTripper {
           return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
               // Don't do this
               heavyComputation()
               return next.RoundTrip(req)
           })
       }
   }
   ```

3. **Use Context**: Respect context cancellation
   ```go
   func GoodMiddleware() Middleware {
       return func(next http.RoundTripper) http.RoundTripper {
           return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
               select {
               case <-req.Context().Done():
                   return nil, req.Context().Err()
               default:
                   return next.RoundTrip(req)
               }
           })
       }
   }
   ```

## Migration Guide

### From Old Client

```go
// Old way
client := rpc.NewClientDefault(rpc.Testnet, "token")

// New way
client, err := rpc.NewClient(
    rpc.WithNetwork(rpc.Testnet),
    rpc.WithToken("token"),
)
```

### Adding Middleware to Existing Code

```go
// Before
client := rpc.NewClientDefault(rpc.Mainnet, "")

// After
client, err := rpc.NewClient(
    rpc.WithNetwork(rpc.Mainnet),
    rpc.WithMiddleware(rpc.LoggingMiddleware()),
)
```

## Testing

### Testing Middleware

```go
func TestCustomMiddleware(t *testing.T) {
    called := false
    baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
        called = true
        return &http.Response{StatusCode: 200}, nil
    })
    
    middleware := CustomMiddleware()
    transport := middleware(baseTransport)
    
    req := httptest.NewRequest("GET", "http://example.com", nil)
    resp, err := transport.RoundTrip(req)
    
    assert.NoError(t, err)
    assert.True(t, called)
    assert.Equal(t, 200, resp.StatusCode)
}
```

### Mocking Middleware

```go
type mockMiddleware struct {
    called bool
}

func (m *mockMiddleware) Middleware() rpc.Middleware {
    return func(next http.RoundTripper) http.RoundTripper {
        return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
            m.called = true
            return next.RoundTrip(req)
        })
    }
}
```

## Examples

### Complete Example: Production Client

```go
package main

import (
    "context"
    "time"
    
    "github.com/dotandev/hintents/internal/rpc"
)

func main() {
    // Create metrics collector
    metrics := &MetricsCollector{}
    
    // Create rate limiter
    limiter := rate.NewLimiter(rate.Limit(10), 1)
    
    // Create client with full middleware stack
    client, err := rpc.NewClient(
        rpc.WithNetwork(rpc.Mainnet),
        rpc.WithToken("your-api-key"),
        rpc.WithCache(true),
        rpc.WithAltURLs([]string{
            "https://rpc1.example.com",
            "https://rpc2.example.com",
        }),
        rpc.WithMiddlewares(
            rpc.HeaderMiddleware(map[string]string{
                "X-Client-Version": "1.0.0",
            }),
            rpc.LoggingMiddleware(),
            rpc.MetricsMiddleware(metrics),
            rpc.RateLimitMiddleware(&RateLimiter{limiter: limiter}),
            rpc.TimeoutMiddleware(30 * time.Second),
        ),
        rpc.WithRetryConfig(rpc.RetryConfig{
            MaxRetries: 3,
            InitialBackoff: 1 * time.Second,
            MaxBackoff: 10 * time.Second,
        }),
    )
    
    if err != nil {
        panic(err)
    }
    
    // Use client
    ctx := context.Background()
    tx, err := client.GetTransaction(ctx, "hash")
    if err != nil {
        panic(err)
    }
    
    // Transaction retrieved successfully
    _ = tx
}
```

## Troubleshooting

### Middleware Not Being Called

Check middleware order and ensure it's applied correctly:

```go
// Verify middleware is added
client, err := rpc.NewClient(
    rpc.WithNetwork(rpc.Testnet),
    rpc.WithMiddleware(rpc.LoggingMiddleware()), // Ensure this is present
)
```

### Performance Issues

Profile middleware overhead:

```bash
go test -bench=BenchmarkMiddleware -benchmem ./internal/rpc
```

### Context Cancellation

Ensure middleware respects context:

```go
func ContextAwareMiddleware() rpc.Middleware {
    return func(next http.RoundTripper) http.RoundTripper {
        return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
            // Check context before proceeding
            if err := req.Context().Err(); err != nil {
                return nil, err
            }
            return next.RoundTrip(req)
        })
    }
}
```

## Related Documentation

- [RPC Fallback Configuration](RPC_FALLBACK.md)
- [Ledger Entry Verification](LEDGER_ENTRY_VERIFICATION.md)
- [Architecture Overview](ARCHITECTURE.md)
