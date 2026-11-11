// Package kentikapi is a thin, service-agnostic adapter around the generated Kentik v6 gRPC clients from
// github.com/kentik/api-schema-public. It owns connection setup (endpoint selection, TLS), per-RPC
// authentication, per-attempt timeouts and retries, so that Terraform resources only deal with generated
// request and response types.
//
// The package deliberately knows nothing about Terraform, so it can be shared between providers.
package kentikapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// DefaultEndpoint is the gRPC endpoint of the Kentik US cluster.
	DefaultEndpoint = "grpc.api.kentik.com:443"
	// EUEndpoint is the gRPC endpoint of the Kentik EU cluster.
	EUEndpoint = "grpc.api.kentik.eu:443"

	// DefaultTimeout bounds a single RPC attempt.
	DefaultTimeout = 60 * time.Second
	// DefaultUserAgent is used when Config.UserAgent is empty.
	DefaultUserAgent = "kentikapi-go"
)

// Region selects one of the Kentik clusters.
type Region string

const (
	RegionUS Region = "US"
	RegionEU Region = "EU"
)

// Config configures a Client. Email and Token are required.
type Config struct {
	// Email and Token are the Kentik API credentials, sent as X-CH-Auth-Email and X-CH-Auth-API-Token.
	Email string
	Token string

	// Endpoint is "host:port" of the gRPC API. A "https://" prefix is accepted and ignored; an "http://"
	// prefix selects a plaintext connection (only for local testing). When empty, the endpoint is
	// derived from Region.
	Endpoint string
	// Region selects the default endpoint when Endpoint is empty. Defaults to RegionUS.
	Region Region
	// Insecure disables TLS. Only meant for local fakes.
	Insecure bool

	// UserAgent is prepended to the gRPC user agent.
	UserAgent string
	// Timeout bounds each RPC attempt. Zero means DefaultTimeout; a negative value disables it.
	Timeout time.Duration
	// Retry configures retries of transient failures.
	Retry RetryConfig

	// UnaryInterceptors are run after the built-in retry and timeout interceptors, i.e. once per attempt.
	UnaryInterceptors []grpc.UnaryClientInterceptor
	// DialOptions are appended to the options built from this Config (for example a bufconn dialer in tests).
	DialOptions []grpc.DialOption
}

// Client holds a gRPC connection to the Kentik API. Create service clients from Conn, or use the typed
// accessors in services.go.
type Client struct {
	conn     *grpc.ClientConn
	endpoint string
}

// New validates cfg and creates a Client. The connection is established lazily on the first RPC.
func New(_ context.Context, cfg Config) (*Client, error) {
	if cfg.Email == "" || cfg.Token == "" {
		return nil, errors.New("kentikapi: Email and Token are required")
	}

	endpoint, plaintext, err := ResolveEndpoint(cfg.Endpoint, cfg.Region)
	if err != nil {
		return nil, err
	}
	plaintext = plaintext || cfg.Insecure

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}

	var transport credentials.TransportCredentials
	if plaintext {
		transport = insecure.NewCredentials()
	} else {
		transport = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}

	interceptors := append([]grpc.UnaryClientInterceptor{
		retryInterceptor(cfg.Retry.withDefaults()),
		timeoutInterceptor(timeout),
	}, cfg.UnaryInterceptors...)

	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(transport),
		grpc.WithPerRPCCredentials(newAuthCredentials(cfg.Email, cfg.Token, !plaintext)),
		grpc.WithUserAgent(ua),
		grpc.WithChainUnaryInterceptor(interceptors...),
	}, cfg.DialOptions...)

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("kentikapi: create gRPC client for %q: %w", endpoint, err)
	}
	return &Client{conn: conn, endpoint: endpoint}, nil
}

// Conn returns the underlying connection, for creating clients of any Kentik gRPC service.
func (c *Client) Conn() *grpc.ClientConn { return c.conn }

// Endpoint returns the resolved "host:port" the client connects to.
func (c *Client) Endpoint() string { return c.endpoint }

// Close closes the underlying connection.
func (c *Client) Close() error { return c.conn.Close() }

func timeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
