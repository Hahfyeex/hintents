// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"net/http"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

// Middleware represents a function that wraps an http.RoundTripper
// Middleware can intercept, modify, or observe HTTP requests and responses
type Middleware func(http.RoundTripper) http.RoundTripper

// MiddlewareChain represents a chain of middleware to be applied
type MiddlewareChain struct {
	middlewares []Middleware
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain(middlewares ...Middleware) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,
	}
}

// Apply applies all middleware in the chain to the given transport
// Middleware is applied in order: first middleware wraps the transport,
// second middleware wraps the first, and so on
func (mc *MiddlewareChain) Apply(transport http.RoundTripper) http.RoundTripper {
	result := transport
	// Apply in reverse order so first middleware is outermost
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		result = mc.middlewares[i](result)
	}
	return result
}

// Append adds middleware to the end of the chain
func (mc *MiddlewareChain) Append(middleware Middleware) {
	mc.middlewares = append(mc.middlewares, middleware)
}

// Prepend adds middleware to the beginning of the chain
func (mc *MiddlewareChain) Prepend(middleware Middleware) {
	mc.middlewares = append([]Middleware{middleware}, mc.middlewares...)
}

// RequestContext holds contextual information about an HTTP request
type RequestContext struct {
	StartTime time.Time
	Method    string
	URL       string
	Headers   http.Header
}

// ResponseContext holds contextual information about an HTTP response
type ResponseContext struct {
	StatusCode int
	Duration   time.Duration
	Error      error
}

// LoggingMiddleware creates middleware that logs HTTP requests and responses
func LoggingMiddleware() Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &loggingTransport{next: next}
	}
}

type loggingTransport struct {
	next http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	
	logger.Logger.Debug("RPC request started",
		"method", req.Method,
		"url", req.URL.String())

	resp, err := t.next.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		logger.Logger.Error("RPC request failed",
			"method", req.Method,
			"url", req.URL.String(),
			"duration", duration,
			"error", err)
		return resp, err
	}

	logger.Logger.Debug("RPC request completed",
		"method", req.Method,
		"url", req.URL.String(),
		"status", resp.StatusCode,
		"duration", duration)

	return resp, nil
}

// MetricsMiddleware creates middleware that collects request metrics
func MetricsMiddleware(collector MetricsCollector) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &metricsTransport{
			next:      next,
			collector: collector,
		}
	}
}

type metricsTransport struct {
	next      http.RoundTripper
	collector MetricsCollector
}

func (t *metricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.next.RoundTrip(req)
	duration := time.Since(start)

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}

	t.collector.RecordRequest(req.Method, req.URL.String(), statusCode, duration, err)
	return resp, err
}

// MetricsCollector defines the interface for collecting request metrics
type MetricsCollector interface {
	RecordRequest(method, url string, statusCode int, duration time.Duration, err error)
}

// HeaderMiddleware creates middleware that adds custom headers to requests
func HeaderMiddleware(headers map[string]string) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &headerTransport{
			next:    next,
			headers: headers,
		}
	}
}

type headerTransport struct {
	next    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone request to avoid modifying original
	reqCopy := req.Clone(req.Context())
	for key, value := range t.headers {
		reqCopy.Header.Set(key, value)
	}
	return t.next.RoundTrip(reqCopy)
}

// TimeoutMiddleware creates middleware that enforces request timeouts
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &timeoutTransport{
			next:    next,
			timeout: timeout,
		}
	}
}

type timeoutTransport struct {
	next    http.RoundTripper
	timeout time.Duration
}

func (t *timeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
	defer cancel()
	
	reqWithTimeout := req.Clone(ctx)
	return t.next.RoundTrip(reqWithTimeout)
}

// RateLimitMiddleware creates middleware that enforces rate limiting
func RateLimitMiddleware(limiter RateLimiter) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &rateLimitTransport{
			next:    next,
			limiter: limiter,
		}
	}
}

type rateLimitTransport struct {
	next    http.RoundTripper
	limiter RateLimiter
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return t.next.RoundTrip(req)
}

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	Wait(ctx context.Context) error
}

// CircuitBreakerMiddleware creates middleware that implements circuit breaker pattern
func CircuitBreakerMiddleware(breaker CircuitBreaker) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &circuitBreakerTransport{
			next:    next,
			breaker: breaker,
		}
	}
}

type circuitBreakerTransport struct {
	next    http.RoundTripper
	breaker CircuitBreaker
}

func (t *circuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.breaker.Allow() {
		return nil, ErrCircuitOpen
	}

	resp, err := t.next.RoundTrip(req)
	
	if err != nil || (resp != nil && resp.StatusCode >= 500) {
		t.breaker.RecordFailure()
	} else {
		t.breaker.RecordSuccess()
	}

	return resp, err
}

// CircuitBreaker defines the interface for circuit breaker pattern
type CircuitBreaker interface {
	Allow() bool
	RecordSuccess()
	RecordFailure()
}

// CachingMiddleware creates middleware that caches responses
func CachingMiddleware(cache ResponseCache) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return &cachingTransport{
			next:  next,
			cache: cache,
		}
	}
}

type cachingTransport struct {
	next  http.RoundTripper
	cache ResponseCache
}

func (t *cachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only cache GET requests
	if req.Method != http.MethodGet {
		return t.next.RoundTrip(req)
	}

	cacheKey := req.URL.String()
	
	// Check cache
	if cached, ok := t.cache.Get(cacheKey); ok {
		logger.Logger.Debug("Cache hit", "url", cacheKey)
		return cached, nil
	}

	// Execute request
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// Cache successful responses
	if resp.StatusCode == http.StatusOK {
		t.cache.Set(cacheKey, resp)
	}

	return resp, nil
}

// ResponseCache defines the interface for response caching
type ResponseCache interface {
	Get(key string) (*http.Response, bool)
	Set(key string, resp *http.Response)
}

// Custom errors
var (
	ErrCircuitOpen = &CircuitOpenError{}
)

// CircuitOpenError represents a circuit breaker open state
type CircuitOpenError struct{}

func (e *CircuitOpenError) Error() string {
	return "circuit breaker is open"
}
