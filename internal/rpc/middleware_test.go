// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddlewareChain_Apply(t *testing.T) {
	callOrder := []string{}

	middleware1 := func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			callOrder = append(callOrder, "middleware1-before")
			resp, err := next.RoundTrip(req)
			callOrder = append(callOrder, "middleware1-after")
			return resp, err
		})
	}

	middleware2 := func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			callOrder = append(callOrder, "middleware2-before")
			resp, err := next.RoundTrip(req)
			callOrder = append(callOrder, "middleware2-after")
			return resp, err
		})
	}

	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		callOrder = append(callOrder, "base")
		return &http.Response{StatusCode: 200}, nil
	})

	chain := NewMiddlewareChain(middleware1, middleware2)
	transport := chain.Apply(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	require.NoError(t, err)

	expected := []string{
		"middleware1-before",
		"middleware2-before",
		"base",
		"middleware2-after",
		"middleware1-after",
	}
	assert.Equal(t, expected, callOrder)
}

func TestMiddlewareChain_Append(t *testing.T) {
	chain := NewMiddlewareChain()
	
	middleware1 := func(next http.RoundTripper) http.RoundTripper {
		return next
	}
	
	chain.Append(middleware1)
	assert.Len(t, chain.middlewares, 1)
}

func TestMiddlewareChain_Prepend(t *testing.T) {
	middleware1 := func(next http.RoundTripper) http.RoundTripper {
		return next
	}
	middleware2 := func(next http.RoundTripper) http.RoundTripper {
		return next
	}

	chain := NewMiddlewareChain(middleware1)
	chain.Prepend(middleware2)
	
	assert.Len(t, chain.middlewares, 2)
}

func TestLoggingMiddleware(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	baseTransport := http.DefaultTransport
	middleware := LoggingMiddleware()
	transport := middleware(baseTransport)

	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestLoggingMiddleware_Error(t *testing.T) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("connection failed")
	})

	middleware := LoggingMiddleware()
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	
	assert.Error(t, err)
}

func TestHeaderMiddleware(t *testing.T) {
	headers := map[string]string{
		"X-Custom-Header": "test-value",
		"X-API-Key":       "secret-key",
	}

	receivedHeaders := make(http.Header)
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		receivedHeaders = req.Header.Clone()
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := HeaderMiddleware(headers)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	
	require.NoError(t, err)
	assert.Equal(t, "test-value", receivedHeaders.Get("X-Custom-Header"))
	assert.Equal(t, "secret-key", receivedHeaders.Get("X-API-Key"))
}

func TestTimeoutMiddleware(t *testing.T) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// Simulate slow response
		select {
		case <-time.After(200 * time.Millisecond):
			return &http.Response{StatusCode: 200}, nil
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	})

	middleware := TimeoutMiddleware(50 * time.Millisecond)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestTimeoutMiddleware_Success(t *testing.T) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := TimeoutMiddleware(1 * time.Second)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp, err := transport.RoundTrip(req)
	
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestMetricsMiddleware(t *testing.T) {
	collector := &mockMetricsCollector{}
	
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := MetricsMiddleware(collector)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	_, err := transport.RoundTrip(req)
	
	require.NoError(t, err)
	assert.Equal(t, 1, collector.requestCount)
	assert.Equal(t, "GET", collector.lastMethod)
	assert.Equal(t, "http://example.com/test", collector.lastURL)
	assert.Equal(t, 200, collector.lastStatusCode)
}

func TestMetricsMiddleware_Error(t *testing.T) {
	collector := &mockMetricsCollector{}
	
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("network error")
	})

	middleware := MetricsMiddleware(collector)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	
	assert.Error(t, err)
	assert.Equal(t, 1, collector.requestCount)
	assert.NotNil(t, collector.lastError)
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := &mockRateLimiter{
		shouldWait: false,
	}
	
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := RateLimitMiddleware(limiter)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	
	require.NoError(t, err)
	assert.Equal(t, 1, limiter.waitCalls)
}

func TestRateLimitMiddleware_Blocked(t *testing.T) {
	limiter := &mockRateLimiter{
		shouldWait: true,
		waitError:  errors.New("rate limit exceeded"),
	}
	
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := RateLimitMiddleware(limiter)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	
	assert.Error(t, err)
	assert.Equal(t, "rate limit exceeded", err.Error())
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	breaker := &mockCircuitBreaker{
		allowRequests: true,
	}
	
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := CircuitBreakerMiddleware(breaker)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	resp, err := transport.RoundTrip(req)
	
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, breaker.successCount)
}

func TestCircuitBreakerMiddleware_Open(t *testing.T) {
	breaker := &mockCircuitBreaker{
		allowRequests: false,
	}
	
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := CircuitBreakerMiddleware(breaker)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	
	assert.Error(t, err)
	assert.Equal(t, ErrCircuitOpen, err)
}

func TestCircuitBreakerMiddleware_Failure(t *testing.T) {
	breaker := &mockCircuitBreaker{
		allowRequests: true,
	}
	
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("connection failed")
	})

	middleware := CircuitBreakerMiddleware(breaker)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	_, err := transport.RoundTrip(req)
	
	assert.Error(t, err)
	assert.Equal(t, 1, breaker.failureCount)
}

// Helper types and functions

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type mockMetricsCollector struct {
	requestCount   int
	lastMethod     string
	lastURL        string
	lastStatusCode int
	lastDuration   time.Duration
	lastError      error
}

func (m *mockMetricsCollector) RecordRequest(method, url string, statusCode int, duration time.Duration, err error) {
	m.requestCount++
	m.lastMethod = method
	m.lastURL = url
	m.lastStatusCode = statusCode
	m.lastDuration = duration
	m.lastError = err
}

type mockRateLimiter struct {
	waitCalls  int
	shouldWait bool
	waitError  error
}

func (m *mockRateLimiter) Wait(ctx context.Context) error {
	m.waitCalls++
	if m.shouldWait && m.waitError != nil {
		return m.waitError
	}
	return nil
}

type mockCircuitBreaker struct {
	allowRequests bool
	successCount  int
	failureCount  int
}

func (m *mockCircuitBreaker) Allow() bool {
	return m.allowRequests
}

func (m *mockCircuitBreaker) RecordSuccess() {
	m.successCount++
}

func (m *mockCircuitBreaker) RecordFailure() {
	m.failureCount++
}
