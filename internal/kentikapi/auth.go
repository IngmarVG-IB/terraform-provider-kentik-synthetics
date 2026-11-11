package kentikapi

import "context"

// Authentication metadata keys, as declared in the security_definitions of the api-schema-public protos.
// gRPC lower-cases metadata keys on the wire.
const (
	AuthEmailKey = "X-CH-Auth-Email"
	AuthTokenKey = "X-CH-Auth-API-Token" //nolint:gosec // header name, not a credential
)

// authCredentials implements credentials.PerRPCCredentials.
type authCredentials struct {
	email, token string
	requireTLS   bool
}

func newAuthCredentials(email, token string, requireTLS bool) authCredentials {
	return authCredentials{email: email, token: token, requireTLS: requireTLS}
}

func (a authCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{AuthEmailKey: a.email, AuthTokenKey: a.token}, nil
}

func (a authCredentials) RequireTransportSecurity() bool { return a.requireTLS }
