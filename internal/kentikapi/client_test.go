package kentikapi_test

import (
	"context"
	"testing"
	"time"

	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kentik/terraform-provider-kentik-synthetics/internal/fakesynthetics"
	"github.com/kentik/terraform-provider-kentik-synthetics/internal/kentikapi"
)

func newClient(t *testing.T, fake *fakesynthetics.Server, mutate func(*kentikapi.Config)) *kentikapi.Client {
	t.Helper()
	cfg := kentikapi.Config{
		Email:       "john@example.com",
		Token:       "secret-token",
		Endpoint:    fake.Endpoint(),
		Insecure:    true,
		Retry:       kentikapi.RetryConfig{MinDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond},
		DialOptions: fake.DialOptions(),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	c, err := kentikapi.New(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestNew_RequiresCredentials(t *testing.T) {
	t.Parallel()
	_, err := kentikapi.New(context.Background(), kentikapi.Config{Email: "a@example.com"})
	require.Error(t, err)
}

func TestClient_SendsAuthMetadata(t *testing.T) {
	t.Parallel()
	fake := fakesynthetics.Start(t)
	c := newClient(t, fake, nil)

	_, err := c.SyntheticsAdmin().ListTests(context.Background(), &syntheticspb.ListTestsRequest{})
	require.NoError(t, err)

	calls := fake.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "ListTests", calls[0].Method)
	assert.Equal(t, "john@example.com", calls[0].Email)
	assert.Equal(t, "secret-token", calls[0].Token)
}

func TestClient_RetriesTransientErrors(t *testing.T) {
	t.Parallel()
	for _, code := range []codes.Code{codes.Unavailable, codes.ResourceExhausted} {
		t.Run(code.String(), func(t *testing.T) {
			t.Parallel()
			fake := fakesynthetics.Start(t)
			c := newClient(t, fake, nil)
			fake.FailNext("ListAgents", status.Error(code, "try again"), status.Error(code, "try again"))

			_, err := c.SyntheticsAdmin().ListAgents(context.Background(), &syntheticspb.ListAgentsRequest{})
			require.NoError(t, err)
			assert.Equal(t, 3, fake.CallCount("ListAgents"))
		})
	}
}

func TestClient_GivesUpAfterMaxAttempts(t *testing.T) {
	t.Parallel()
	fake := fakesynthetics.Start(t)
	c := newClient(t, fake, func(cfg *kentikapi.Config) { cfg.Retry.MaxAttempts = 2 })
	fake.FailNext("ListAgents",
		status.Error(codes.Unavailable, "1"), status.Error(codes.Unavailable, "2"), status.Error(codes.Unavailable, "3"))

	_, err := c.SyntheticsAdmin().ListAgents(context.Background(), &syntheticspb.ListAgentsRequest{})
	require.Equal(t, codes.Unavailable, status.Code(err))
	assert.Equal(t, 2, fake.CallCount("ListAgents"))
}

func TestClient_DoesNotRetryPermanentErrors(t *testing.T) {
	t.Parallel()
	fake := fakesynthetics.Start(t)
	c := newClient(t, fake, nil)

	_, err := c.SyntheticsAdmin().GetTest(context.Background(), &syntheticspb.GetTestRequest{Id: "missing"})
	require.Equal(t, codes.NotFound, status.Code(err))
	assert.Equal(t, 1, fake.CallCount("GetTest"))
}

func TestClient_RunsUserInterceptorsPerAttempt(t *testing.T) {
	t.Parallel()
	fake := fakesynthetics.Start(t)
	attempts := 0
	c := newClient(t, fake, func(cfg *kentikapi.Config) {
		cfg.UnaryInterceptors = []grpc.UnaryClientInterceptor{
			func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
				invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
			) error {
				attempts++
				return invoker(ctx, method, req, reply, cc, opts...)
			},
		}
	})
	fake.FailNext("ListTests", status.Error(codes.Unavailable, "x"))

	_, err := c.SyntheticsAdmin().ListTests(context.Background(), &syntheticspb.ListTestsRequest{})
	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestClient_RespectsContextCancellation(t *testing.T) {
	t.Parallel()
	fake := fakesynthetics.Start(t)
	c := newClient(t, fake, func(cfg *kentikapi.Config) {
		cfg.Retry = kentikapi.RetryConfig{MaxAttempts: 10, MinDelay: time.Hour, MaxDelay: time.Hour}
	})
	fake.FailNext("ListTests", status.Error(codes.Unavailable, "x"))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.SyntheticsAdmin().ListTests(ctx, &syntheticspb.ListTestsRequest{})
	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestResolveEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in        string
		region    kentikapi.Region
		want      string
		plaintext bool
		wantErr   bool
	}{
		{in: "", want: kentikapi.DefaultEndpoint},
		{in: "", region: "eu", want: kentikapi.EUEndpoint},
		{in: "", region: "mars", wantErr: true},
		{in: "grpc.api.kentik.eu:443", want: kentikapi.EUEndpoint},
		{in: "example.com", want: "example.com:443"},
		{in: "example.com:9000", want: "example.com:9000"},
		{in: "https://synthetics.api.kentik.com", want: kentikapi.DefaultEndpoint},
		{in: "https://api.kentik.eu/api/v6", want: kentikapi.EUEndpoint},
		{in: "http://localhost:9955", want: "localhost:9955", plaintext: true},
		{in: "http://localhost", want: "localhost:80", plaintext: true},
		{in: "passthrough:///bufnet", want: "passthrough:///bufnet"},
		{in: "ftp://example.com", wantErr: true},
	}
	for _, tt := range tests {
		got, plaintext, err := kentikapi.ResolveEndpoint(tt.in, tt.region)
		if tt.wantErr {
			assert.Error(t, err, tt.in)
			continue
		}
		require.NoError(t, err, tt.in)
		assert.Equal(t, tt.want, got, tt.in)
		assert.Equal(t, tt.plaintext, plaintext, tt.in)
	}
}

func TestClient_CredentialsRequireTLSUnlessInsecure(t *testing.T) {
	t.Parallel()
	fake := fakesynthetics.Start(t)
	// The fake's dial options force a plaintext transport. Without Insecure the auth credentials demand
	// TLS, so the client must refuse to send them.
	_, err := kentikapi.New(context.Background(), kentikapi.Config{
		Email: "a@example.com", Token: "t", Endpoint: fake.Endpoint(), DialOptions: fake.DialOptions(),
	})
	require.ErrorContains(t, err, "transport level security")
}
