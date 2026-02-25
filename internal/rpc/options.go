// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"fmt"
	"net/http"
	"time"

	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
)

// ClientOption is a functional option for configuring the RPC Client
type ClientOption func(*clientConfig) error

// clientConfig holds the configuration for creating a Client
type clientConfig struct {
	network        Network
	horizonURL     string
	sorobanURL     string
	altURLs        []string
	token          string
	cacheEnabled   bool
	networkConfig  *NetworkConfig
	httpClient     *http.Client
	middlewares    []Middleware
	retryConfig    *RetryConfig
}

// WithNetwork sets the network for the client
func WithNetwork(network Network) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.network = network
		return nil
	}
}

// WithHorizonURL sets a custom Horizon URL
func WithHorizonURL(url string) ClientOption {
	return func(cfg *clientConfig) error {
		if url == "" {
			return fmt.Errorf("horizon URL cannot be empty")
		}
		cfg.horizonURL = url
		return nil
	}
}

// WithSorobanURL sets a custom Soroban RPC URL
func WithSorobanURL(url string) ClientOption {
	return func(cfg *clientConfig) error {
		if url == "" {
			return fmt.Errorf("soroban URL cannot be empty")
		}
		cfg.sorobanURL = url
		return nil
	}
}

// WithAltURLs sets alternative URLs for failover
func WithAltURLs(urls []string) ClientOption {
	return func(cfg *clientConfig) error {
		if len(urls) == 0 {
			return fmt.Errorf("alternative URLs cannot be empty")
		}
		cfg.altURLs = urls
		return nil
	}
}

// WithToken sets the authentication token
func WithToken(token string) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.token = token
		return nil
	}
}

// WithCache enables or disables caching
func WithCache(enabled bool) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.cacheEnabled = enabled
		return nil
	}
}

// WithNetworkConfig sets a custom network configuration
func WithNetworkConfig(config NetworkConfig) ClientOption {
	return func(cfg *clientConfig) error {
		if err := ValidateNetworkConfig(config); err != nil {
			return err
		}
		cfg.networkConfig = &config
		return nil
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(cfg *clientConfig) error {
		if client == nil {
			return fmt.Errorf("HTTP client cannot be nil")
		}
		cfg.httpClient = client
		return nil
	}
}

// WithMiddleware adds a middleware to the client
func WithMiddleware(middleware Middleware) ClientOption {
	return func(cfg *clientConfig) error {
		if middleware == nil {
			return fmt.Errorf("middleware cannot be nil")
		}
		cfg.middlewares = append(cfg.middlewares, middleware)
		return nil
	}
}

// WithMiddlewares adds multiple middlewares to the client
func WithMiddlewares(middlewares ...Middleware) ClientOption {
	return func(cfg *clientConfig) error {
		for _, m := range middlewares {
			if m == nil {
				return fmt.Errorf("middleware cannot be nil")
			}
		}
		cfg.middlewares = append(cfg.middlewares, middlewares...)
		return nil
	}
}

// WithRetryConfig sets a custom retry configuration
func WithRetryConfig(config RetryConfig) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.retryConfig = &config
		return nil
	}
}

// NewClient creates a new RPC client with the given options
func NewClient(opts ...ClientOption) (*Client, error) {
	cfg := &clientConfig{
		network:      Mainnet,
		cacheEnabled: false,
		middlewares:  []Middleware{},
	}

	// Apply all options
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Use custom network config if provided
	if cfg.networkConfig != nil {
		return buildClientFromNetworkConfig(cfg)
	}

	// Build client from network type
	return buildClientFromNetwork(cfg)
}

// buildClientFromNetworkConfig creates a client from a custom network configuration
func buildClientFromNetworkConfig(cfg *clientConfig) (*Client, error) {
	transport := buildTransport(cfg)

	httpClient := &http.Client{
		Transport: transport,
	}

	horizonClient := &horizonclient.Client{
		HorizonURL: cfg.networkConfig.HorizonURL,
		HTTP:       httpClient,
	}

	sorobanURL := cfg.networkConfig.SorobanRPCURL
	if sorobanURL == "" {
		sorobanURL = cfg.networkConfig.HorizonURL
	}

	altURLs := cfg.altURLs
	if len(altURLs) == 0 {
		altURLs = []string{cfg.networkConfig.HorizonURL}
	}

	return &Client{
		Horizon:      horizonClient,
		HorizonURL:   cfg.networkConfig.HorizonURL,
		Network:      "custom",
		SorobanURL:   sorobanURL,
		AltURLs:      altURLs,
		token:        cfg.token,
		Config:       *cfg.networkConfig,
		CacheEnabled: cfg.cacheEnabled,
		failures:     make(map[string]int),
		lastFailure:  make(map[string]time.Time),
	}, nil
}

// buildClientFromNetwork creates a client from a predefined network type
func buildClientFromNetwork(cfg *clientConfig) (*Client, error) {
	var networkConfig NetworkConfig

	switch cfg.network {
	case Testnet:
		networkConfig = TestnetConfig
	case Mainnet:
		networkConfig = MainnetConfig
	case Futurenet:
		networkConfig = FuturenetConfig
	default:
		return nil, fmt.Errorf("unsupported network: %s", cfg.network)
	}

	// Override with custom URLs if provided
	if cfg.horizonURL != "" {
		networkConfig.HorizonURL = cfg.horizonURL
	}
	if cfg.sorobanURL != "" {
		networkConfig.SorobanRPCURL = cfg.sorobanURL
	}

	transport := buildTransport(cfg)

	httpClient := &http.Client{
		Transport: transport,
	}

	horizonClient := &horizonclient.Client{
		HorizonURL: networkConfig.HorizonURL,
		HTTP:       httpClient,
	}

	altURLs := cfg.altURLs
	if len(altURLs) == 0 {
		altURLs = []string{networkConfig.HorizonURL}
	}

	return &Client{
		Horizon:      horizonClient,
		HorizonURL:   networkConfig.HorizonURL,
		Network:      cfg.network,
		SorobanURL:   networkConfig.SorobanRPCURL,
		AltURLs:      altURLs,
		token:        cfg.token,
		Config:       networkConfig,
		CacheEnabled: cfg.cacheEnabled,
		failures:     make(map[string]int),
		lastFailure:  make(map[string]time.Time),
	}, nil
}

// buildTransport constructs the HTTP transport with all middleware applied
func buildTransport(cfg *clientConfig) http.RoundTripper {
	var baseTransport http.RoundTripper = http.DefaultTransport

	// Apply authentication if token is provided
	if cfg.token != "" {
		baseTransport = &authTransport{
			token:     cfg.token,
			transport: baseTransport,
		}
	}

	// Apply retry logic
	retryConfig := DefaultRetryConfig()
	if cfg.retryConfig != nil {
		retryConfig = *cfg.retryConfig
	}
	baseTransport = NewRetryTransport(retryConfig, baseTransport)

	// Apply custom middlewares
	if len(cfg.middlewares) > 0 {
		chain := NewMiddlewareChain(cfg.middlewares...)
		baseTransport = chain.Apply(baseTransport)
	}

	return baseTransport
}

// ValidateNetworkConfig validates a network configuration
func ValidateNetworkConfig(config NetworkConfig) error {
	if config.Name == "" {
		return fmt.Errorf("network name cannot be empty")
	}
	if config.HorizonURL == "" {
		return fmt.Errorf("horizon URL cannot be empty")
	}
	if config.NetworkPassphrase == "" {
		return fmt.Errorf("network passphrase cannot be empty")
	}
	return nil
}
