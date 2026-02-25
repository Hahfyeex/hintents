// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_DefaultOptions(t *testing.T) {
	client, err := NewClient()
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, Mainnet, client.Network)
	assert.False(t, client.CacheEnabled)
}

func TestNewClient_WithNetwork(t *testing.T) {
	tests := []struct {
		name    string
		network Network
	}{
		{"testnet", Testnet},
		{"mainnet", Mainnet},
		{"futurenet", Futurenet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(WithNetwork(tt.network))
			require.NoError(t, err)
			assert.Equal(t, tt.network, client.Network)
		})
	}
}

func TestNewClient_WithHorizonURL(t *testing.T) {
	customURL := "https://custom-horizon.example.com"
	client, err := NewClient(
		WithNetwork(Testnet),
		WithHorizonURL(customURL),
	)
	
	require.NoError(t, err)
	assert.Equal(t, customURL, client.HorizonURL)
}

func TestNewClient_WithHorizonURL_Empty(t *testing.T) {
	_, err := NewClient(
		WithNetwork(Testnet),
		WithHorizonURL(""),
	)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "horizon URL cannot be empty")
}

func TestNewClient_WithSorobanURL(t *testing.T) {
	customURL := "https://custom-soroban.example.com"
	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanURL(customURL),
	)
	
	require.NoError(t, err)
	assert.Equal(t, customURL, client.SorobanURL)
}

func TestNewClient_WithAltURLs(t *testing.T) {
	altURLs := []string{
		"https://rpc1.example.com",
		"https://rpc2.example.com",
		"https://rpc3.example.com",
	}
	
	client, err := NewClient(
		WithNetwork(Testnet),
		WithAltURLs(altURLs),
	)
	
	require.NoError(t, err)
	assert.Equal(t, altURLs, client.AltURLs)
}

func TestNewClient_WithAltURLs_Empty(t *testing.T) {
	_, err := NewClient(
		WithNetwork(Testnet),
		WithAltURLs([]string{}),
	)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alternative URLs cannot be empty")
}

func TestNewClient_WithToken(t *testing.T) {
	token := "test-token-123"
	client, err := NewClient(
		WithNetwork(Testnet),
		WithToken(token),
	)
	
	require.NoError(t, err)
	assert.Equal(t, token, client.token)
}

func TestNewClient_WithCache(t *testing.T) {
	client, err := NewClient(
		WithNetwork(Testnet),
		WithCache(true),
	)
	
	require.NoError(t, err)
	assert.True(t, client.CacheEnabled)
}

func TestNewClient_WithNetworkConfig(t *testing.T) {
	config := NetworkConfig{
		Name:              "custom",
		HorizonURL:        "https://custom-horizon.example.com",
		NetworkPassphrase: "Custom Network ; January 2025",
		SorobanRPCURL:     "https://custom-soroban.example.com",
	}
	
	client, err := NewClient(WithNetworkConfig(config))
	
	require.NoError(t, err)
	assert.Equal(t, "custom", client.Network)
	assert.Equal(t, config.HorizonURL, client.HorizonURL)
	assert.Equal(t, config.SorobanRPCURL, client.SorobanURL)
}

func TestNewClient_WithNetworkConfig_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		config NetworkConfig
		errMsg string
	}{
		{
			name: "empty name",
			config: NetworkConfig{
				HorizonURL:        "https://example.com",
				NetworkPassphrase: "Test",
			},
			errMsg: "network name cannot be empty",
		},
		{
			name: "empty horizon URL",
			config: NetworkConfig{
				Name:              "test",
				NetworkPassphrase: "Test",
			},
			errMsg: "horizon URL cannot be empty",
		},
		{
			name: "empty passphrase",
			config: NetworkConfig{
				Name:       "test",
				HorizonURL: "https://example.com",
			},
			errMsg: "network passphrase cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(WithNetworkConfig(tt.config))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	_, err := NewClient(
		WithNetwork(Testnet),
		WithHTTPClient(customClient),
	)
	
	require.NoError(t, err)
}

func TestNewClient_WithHTTPClient_Nil(t *testing.T) {
	_, err := NewClient(
		WithNetwork(Testnet),
		WithHTTPClient(nil),
	)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP client cannot be nil")
}

func TestNewClient_WithMiddleware(t *testing.T) {
	middleware := LoggingMiddleware()
	
	client, err := NewClient(
		WithNetwork(Testnet),
		WithMiddleware(middleware),
	)
	
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithMiddleware_Nil(t *testing.T) {
	_, err := NewClient(
		WithNetwork(Testnet),
		WithMiddleware(nil),
	)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "middleware cannot be nil")
}

func TestNewClient_WithMiddlewares(t *testing.T) {
	middlewares := []Middleware{
		LoggingMiddleware(),
		HeaderMiddleware(map[string]string{"X-Test": "value"}),
	}
	
	client, err := NewClient(
		WithNetwork(Testnet),
		WithMiddlewares(middlewares...),
	)
	
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithMiddlewares_ContainsNil(t *testing.T) {
	middlewares := []Middleware{
		LoggingMiddleware(),
		nil,
	}
	
	_, err := NewClient(
		WithNetwork(Testnet),
		WithMiddlewares(middlewares...),
	)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "middleware cannot be nil")
}

func TestNewClient_WithRetryConfig(t *testing.T) {
	config := RetryConfig{
		MaxRetries:         5,
		InitialBackoff:     2 * time.Second,
		MaxBackoff:         30 * time.Second,
		JitterFraction:     0.2,
		StatusCodesToRetry: []int{429, 500, 502, 503, 504},
	}
	
	client, err := NewClient(
		WithNetwork(Testnet),
		WithRetryConfig(config),
	)
	
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_MultipleOptions(t *testing.T) {
	client, err := NewClient(
		WithNetwork(Testnet),
		WithToken("test-token"),
		WithCache(true),
		WithMiddleware(LoggingMiddleware()),
		WithAltURLs([]string{"https://rpc1.example.com", "https://rpc2.example.com"}),
	)
	
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, Testnet, client.Network)
	assert.Equal(t, "test-token", client.token)
	assert.True(t, client.CacheEnabled)
	assert.Len(t, client.AltURLs, 2)
}

func TestValidateNetworkConfig(t *testing.T) {
	validConfig := NetworkConfig{
		Name:              "test",
		HorizonURL:        "https://example.com",
		NetworkPassphrase: "Test Network",
	}
	
	err := ValidateNetworkConfig(validConfig)
	assert.NoError(t, err)
}

func TestBuildTransport(t *testing.T) {
	cfg := &clientConfig{
		token:       "test-token",
		middlewares: []Middleware{LoggingMiddleware()},
	}
	
	transport := buildTransport(cfg)
	assert.NotNil(t, transport)
}

func TestBuildTransport_NoToken(t *testing.T) {
	cfg := &clientConfig{
		middlewares: []Middleware{},
	}
	
	transport := buildTransport(cfg)
	assert.NotNil(t, transport)
}

func TestBuildTransport_CustomRetryConfig(t *testing.T) {
	retryConfig := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 2 * time.Second,
	}
	
	cfg := &clientConfig{
		retryConfig: &retryConfig,
	}
	
	transport := buildTransport(cfg)
	assert.NotNil(t, transport)
}
