// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkMiddlewareChain_Single(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := LoggingMiddleware()
	chain := NewMiddlewareChain(middleware)
	transport := chain.Apply(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

func BenchmarkMiddlewareChain_Multiple(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middlewares := []Middleware{
		LoggingMiddleware(),
		HeaderMiddleware(map[string]string{"X-Test": "value"}),
		MetricsMiddleware(&mockMetricsCollector{}),
	}

	chain := NewMiddlewareChain(middlewares...)
	transport := chain.Apply(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

func BenchmarkLoggingMiddleware(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := LoggingMiddleware()
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

func BenchmarkHeaderMiddleware(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	headers := map[string]string{
		"X-Custom-Header": "value1",
		"X-API-Key":       "value2",
		"X-Request-ID":    "value3",
	}

	middleware := HeaderMiddleware(headers)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

func BenchmarkMetricsMiddleware(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	collector := &mockMetricsCollector{}
	middleware := MetricsMiddleware(collector)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

func BenchmarkTimeoutMiddleware(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	middleware := TimeoutMiddleware(5 * time.Second)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	limiter := &mockRateLimiter{shouldWait: false}
	middleware := RateLimitMiddleware(limiter)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

func BenchmarkCircuitBreakerMiddleware(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	breaker := &mockCircuitBreaker{allowRequests: true}
	middleware := CircuitBreakerMiddleware(breaker)
	transport := middleware(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

func BenchmarkNewClient_WithMiddleware(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewClient(
			WithNetwork(Testnet),
			WithMiddleware(LoggingMiddleware()),
		)
	}
}

func BenchmarkNewClient_WithMultipleMiddlewares(b *testing.B) {
	middlewares := []Middleware{
		LoggingMiddleware(),
		HeaderMiddleware(map[string]string{"X-Test": "value"}),
		MetricsMiddleware(&mockMetricsCollector{}),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewClient(
			WithNetwork(Testnet),
			WithMiddlewares(middlewares...),
		)
	}
}

func BenchmarkMiddlewareOverhead_NoMiddleware(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		baseTransport.RoundTrip(req)
	}
}

func BenchmarkMiddlewareOverhead_WithMiddleware(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	chain := NewMiddlewareChain(
		LoggingMiddleware(),
		HeaderMiddleware(map[string]string{"X-Test": "value"}),
		MetricsMiddleware(&mockMetricsCollector{}),
	)
	transport := chain.Apply(baseTransport)

	req := httptest.NewRequest("GET", "http://example.com", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.RoundTrip(req)
	}
}

// Benchmark with real HTTP server
func BenchmarkMiddleware_RealHTTP(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	chain := NewMiddlewareChain(
		LoggingMiddleware(),
		HeaderMiddleware(map[string]string{"X-Test": "value"}),
	)
	transport := chain.Apply(http.DefaultTransport)

	client := &http.Client{Transport: transport}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := client.Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
	}
}

// Benchmark parallel requests
func BenchmarkMiddleware_Parallel(b *testing.B) {
	baseTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200}, nil
	})

	chain := NewMiddlewareChain(
		LoggingMiddleware(),
		MetricsMiddleware(&mockMetricsCollector{}),
	)
	transport := chain.Apply(baseTransport)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest("GET", "http://example.com", nil)
		for pb.Next() {
			transport.RoundTrip(req)
		}
	})
}

// Mock rate limiter that doesn't actually wait
type noWaitRateLimiter struct{}

func (n *noWaitRateLimiter) Wait(ctx context.Context) error {
	return nil
}
