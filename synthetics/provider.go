package synthetics

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/kentik/terraform-provider-kentik-synthetics/internal/kentikapi"
)

const (
	apiURLKey      = "api_url"
	endpointKey    = "endpoint"
	regionKey      = "region"
	emailKey       = "email"
	tokenKey       = "token"
	timeoutKey     = "timeout"
	retryKey       = "retry"
	maxAttemptsKey = "max_attempts"
	minDelayKey    = "min_delay"
	maxDelayKey    = "max_delay"
	logPayloadsKey = "log_payloads"
)

// apiClient is the provider meta passed to resources and data sources.
type apiClient struct {
	*kentikapi.Client
	admin syntheticspb.SyntheticsAdminServiceClient
}

// Option customises the API client configuration. Tests use it to connect to a fake API.
type Option func(*kentikapi.Config)

// New returns a provider factory. version is reported in the gRPC user agent.
func New(version string, opts ...Option) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: providerSchema(),
			ResourcesMap: map[string]*schema.Resource{
				"kentik-synthetics_test": resourceTest(),
			},
			DataSourcesMap: map[string]*schema.Resource{
				"kentik-synthetics_agent":  dataSourceAgent(),
				"kentik-synthetics_agents": dataSourceAgents(),
				"kentik-synthetics_test":   dataSourceTest(),
				"kentik-synthetics_tests":  dataSourceTests(),
			},
		}
		p.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
			return configure(ctx, d, version, p.TerraformVersion, opts)
		}
		return p
	}
}

// NewProvider returns the provider with version "dev". Kept for compatibility.
func NewProvider() *schema.Provider { return New("dev")() }

func providerSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		emailKey: {
			Type:        schema.TypeString,
			Required:    true,
			DefaultFunc: schema.EnvDefaultFunc("KTAPI_AUTH_EMAIL", nil),
			Description: "Kentik user email (required). Can also be set with the `KTAPI_AUTH_EMAIL` environment variable.",
		},
		tokenKey: {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			DefaultFunc: schema.EnvDefaultFunc("KTAPI_AUTH_TOKEN", nil),
			Description: "Kentik API token (required). Can also be set with the `KTAPI_AUTH_TOKEN` environment variable.",
		},
		endpointKey: {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("KTAPI_ENDPOINT", nil),
			Description: "Kentik gRPC API endpoint as `host:port`. Defaults to `" + kentikapi.DefaultEndpoint +
				"` (or `" + kentikapi.EUEndpoint + "` with `region = \"EU\"`). An `http://` prefix selects a " +
				"plaintext connection, for local testing only. Can also be set with `KTAPI_ENDPOINT`.",
		},
		regionKey: {
			Type:         schema.TypeString,
			Optional:     true,
			DefaultFunc:  schema.EnvDefaultFunc("KTAPI_REGION", "US"),
			ValidateFunc: validation.StringInSlice([]string{"US", "EU", "us", "eu"}, false),
			Description:  "Kentik cluster used when `endpoint` is not set: `US` (default) or `EU`. Can also be set with `KTAPI_REGION`.",
		},
		apiURLKey: {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("KTAPI_URL", nil),
			Deprecated:  "Use endpoint (or region) instead.",
			Description: "**Deprecated.** Use `endpoint` or `region`. URL of the former REST API; known Kentik " +
				"hostnames are mapped to the gRPC endpoint of the same cluster. Can also be set with `KTAPI_URL`.",
		},
		timeoutKey: {
			Type:         schema.TypeString,
			Optional:     true,
			DefaultFunc:  schema.EnvDefaultFunc("KTAPI_TIMEOUT", "60s"),
			ValidateFunc: validateDuration,
			Description:  "Timeout of a single API request attempt, as a Go duration (default `60s`). Can also be set with `KTAPI_TIMEOUT`.",
		},
		retryKey: {
			Type:        schema.TypeList,
			Optional:    true,
			MaxItems:    1,
			Description: "Retry of requests that fail with a transient error (gRPC `UNAVAILABLE` or `RESOURCE_EXHAUSTED`).",
			Elem: &schema.Resource{Schema: map[string]*schema.Schema{
				maxAttemptsKey: {
					Type:        schema.TypeInt,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("KTAPI_RETRY_MAX_ATTEMPTS", nil),
					Description: fmt.Sprintf("Maximum number of attempts, including the first one (default %d; 1 disables "+
						"retries). Can also be set with `KTAPI_RETRY_MAX_ATTEMPTS`.", kentikapi.DefaultMaxAttempts),
				},
				minDelayKey: {
					Type:         schema.TypeString,
					Optional:     true,
					DefaultFunc:  schema.EnvDefaultFunc("KTAPI_RETRY_MIN_DELAY", nil),
					ValidateFunc: validateDuration,
					Description: "Delay before the first retry, as a Go duration (default `1s`); it doubles on every " +
						"retry. Can also be set with `KTAPI_RETRY_MIN_DELAY`.",
				},
				maxDelayKey: {
					Type:         schema.TypeString,
					Optional:     true,
					DefaultFunc:  schema.EnvDefaultFunc("KTAPI_RETRY_MAX_DELAY", nil),
					ValidateFunc: validateDuration,
					Description:  "Maximum delay between retries, as a Go duration (default `30s`). Can also be set with `KTAPI_RETRY_MAX_DELAY`.",
				},
			}},
		},
		logPayloadsKey: {
			Type:        schema.TypeBool,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("KTAPI_LOG_PAYLOADS", false),
			Description: "Log request and response payloads at debug level (`TF_LOG=DEBUG`). Credentials are never " +
				"logged. Can also be set with `KTAPI_LOG_PAYLOADS`.",
		},
	}
}

func configure(
	ctx context.Context, d *schema.ResourceData, version, tfVersion string, opts []Option,
) (any, diag.Diagnostics) {
	var diags diag.Diagnostics

	cfg := kentikapi.Config{
		Email:     d.Get(emailKey).(string),
		Token:     d.Get(tokenKey).(string),
		Region:    kentikapi.Region(d.Get(regionKey).(string)),
		UserAgent: fmt.Sprintf("terraform-provider-kentik-synthetics/%s (terraform/%s)", version, tfVersion),
	}

	cfg.Endpoint = d.Get(endpointKey).(string)
	if cfg.Endpoint == "" {
		cfg.Endpoint = d.Get(apiURLKey).(string)
	}

	var err error
	if cfg.Timeout, err = parseDuration(d.Get(timeoutKey).(string)); err != nil {
		return nil, diag.Errorf("parse %s: %v", timeoutKey, err)
	}
	if cfg.Retry, err = retryConfig(d); err != nil {
		return nil, diag.FromErr(err)
	}
	if d.Get(logPayloadsKey).(bool) {
		cfg.UnaryInterceptors = append(cfg.UnaryInterceptors, logPayloadsInterceptor)
	}
	for _, o := range opts {
		o(&cfg)
	}

	c, err := kentikapi.New(ctx, cfg)
	if err != nil {
		return nil, diag.FromErr(err)
	}
	tflog.Debug(ctx, "Created Kentik API client", map[string]any{
		"endpoint": c.Endpoint(), "email": cfg.Email, "timeout": cfg.Timeout.String(),
		"retry_max_attempts": cfg.Retry.MaxAttempts,
	})
	return &apiClient{Client: c, admin: c.SyntheticsAdmin()}, diags
}

func retryConfig(d *schema.ResourceData) (kentikapi.RetryConfig, error) {
	var rc kentikapi.RetryConfig
	l, _ := d.Get(retryKey).([]any)
	m := map[string]any{}
	if len(l) > 0 && l[0] != nil {
		m, _ = l[0].(map[string]any)
	}
	if v, ok := m[maxAttemptsKey].(int); ok {
		rc.MaxAttempts = v
	}
	var err error
	if v, _ := m[minDelayKey].(string); v != "" {
		if rc.MinDelay, err = time.ParseDuration(v); err != nil {
			return rc, fmt.Errorf("parse %s duration: %w", minDelayKey, err)
		}
	}
	if v, _ := m[maxDelayKey].(string); v != "" {
		if rc.MaxDelay, err = time.ParseDuration(v); err != nil {
			return rc, fmt.Errorf("parse %s duration: %w", maxDelayKey, err)
		}
	}
	return rc, nil
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

func validateDuration(v any, k string) ([]string, []error) {
	s, _ := v.(string)
	if _, err := parseDuration(s); err != nil {
		return nil, []error{fmt.Errorf("parse %s duration: %w", k, err)}
	}
	return nil, nil
}

func logPayloadsInterceptor(
	ctx context.Context, method string, req, reply any, cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	tflog.Debug(ctx, "Kentik API request", map[string]any{"method": method, "payload": marshalPayload(req)})
	err := invoker(ctx, method, req, reply, cc, opts...)
	fields := map[string]any{"method": method, "payload": marshalPayload(reply)}
	if err != nil {
		fields["error"] = err.Error()
	}
	tflog.Debug(ctx, "Kentik API response", fields)
	return err
}

func marshalPayload(v any) string {
	m, ok := v.(proto.Message)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	b, err := protojson.Marshal(m)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func errInvalidStatus(k, s string) error {
	return fmt.Errorf("%s: invalid test status %q, want %s or %s", k, s, statusActive, statusPaused)
}
