package kentikapi

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// legacyHosts maps REST hostnames used by older clients (for example the api_url of the v0.2 Terraform
// provider) to the gRPC endpoint of the same cluster.
var legacyHosts = map[string]string{
	"api.kentik.com":            DefaultEndpoint,
	"synthetics.api.kentik.com": DefaultEndpoint,
	"grpc.api.kentik.com":       DefaultEndpoint,
	"api.kentik.eu":             EUEndpoint,
	"synthetics.api.kentik.eu":  EUEndpoint,
	"grpc.api.kentik.eu":        EUEndpoint,
}

// ResolveEndpoint returns the "host:port" to dial and whether the connection must be plaintext.
//
// Accepted forms: "" (use region), "host", "host:port", "https://host[:port][/path]" (TLS),
// "http://host[:port]" (plaintext) and gRPC resolver targets such as "passthrough:///name" or "unix:///path"
// (TLS unless Config.Insecure is set). Known Kentik REST hostnames are mapped to the gRPC endpoint of the
// same cluster.
func ResolveEndpoint(endpoint string, region Region) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		switch Region(strings.ToUpper(string(region))) {
		case "", RegionUS:
			return DefaultEndpoint, false, nil
		case RegionEU:
			return EUEndpoint, false, nil
		default:
			return "", false, fmt.Errorf("kentikapi: unknown region %q (want US or EU)", region)
		}
	}

	// gRPC resolver targets (e.g. "passthrough:///bufnet", "unix:///tmp/sock") are used as-is.
	for _, scheme := range []string{"passthrough:", "unix:", "dns:"} {
		if strings.HasPrefix(endpoint, scheme) {
			return endpoint, false, nil
		}
	}

	plaintext := false
	defaultPort := "443"
	if strings.Contains(endpoint, "://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return "", false, fmt.Errorf("kentikapi: parse endpoint %q: %w", endpoint, err)
		}
		switch u.Scheme {
		case "https", "grpcs":
		case "http", "grpc":
			plaintext = true
			defaultPort = "80"
		default:
			return "", false, fmt.Errorf("kentikapi: unsupported endpoint scheme %q", u.Scheme)
		}
		endpoint = u.Host
	}

	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		host, port = endpoint, defaultPort
	}
	if host == "" {
		return "", false, fmt.Errorf("kentikapi: endpoint %q has no host", endpoint)
	}
	if mapped, ok := legacyHosts[strings.ToLower(host)]; ok && !plaintext {
		return mapped, false, nil
	}
	return net.JoinHostPort(host, port), plaintext, nil
}
