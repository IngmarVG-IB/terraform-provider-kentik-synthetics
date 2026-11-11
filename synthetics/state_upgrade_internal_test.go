package synthetics

import (
	"context"
	"encoding/json"
	"testing"

	ctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v0State is kentik-synthetics_test state as written by provider v0.2.1 (flatmap-free JSON attributes).
const v0State = `{
  "id": "1001", "name": "legacy", "type": "dns_grid", "status": "", "expires_on": "", "cdate": "", "edate": "",
  "created_by": [], "last_updated_by": [],
  "settings": [{
    "hostname": [], "ip": [], "agent": [], "flow": [], "site": [], "tag": [], "dns": [], "url": [],
    "network_grid": [], "page_load": [],
    "dns_grid": [{"targets": ["www.example.com", "ignored.example.com"], "type": "DNS_RECORD_AAAA"}],
    "agent_ids": ["968"], "period": 60, "count": 0, "expiry": 0, "limit": 0, "tasks": ["dns"],
    "health_settings": [{"latency_critical": 1.5, "http_valid_codes": [200], "dns_valid_codes": []}],
    "monitoring_settings": [{"notification_channels": ["nc-1"]}],
    "ping": [{"period": 60, "count": 5.0, "expiry": 3000.0, "delay": 4.5}],
    "trace": [{"period": 60, "count": 3.0, "protocol": "udp", "port": 33434.0, "expiry": 22500.0, "limit": 30.0, "delay": 7}],
    "port": 443, "protocol": "tcp", "family": "IP_FAMILY_DUAL", "servers": ["192.0.2.1", "192.0.2.2"],
    "rollup_level": 1,
    "http": [{"period": 1, "expiry": 2, "method": "GET", "headers": {}, "body": "", "ignore_tls_errors": false, "css_selectors": {}}]
  }]
}`

func TestUpgradeTestStateV0(t *testing.T) {
	t.Parallel()
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(v0State), &raw))

	got, err := upgradeTestStateV0(context.Background(), raw, nil)
	require.NoError(t, err)

	get := func(path string) any {
		v, ok := getPath(got, path)
		require.Truef(t, ok, "path %s missing", path)
		return v
	}
	assert.Equal(t, statusActive, got["status"])
	assert.Equal(t, []any{"nc-1"}, get("settings.notification_channels"))
	assert.Equal(t, "tcp", get("settings.ping.protocol"))
	assert.Equal(t, 443, get("settings.ping.port"))
	assert.Equal(t, 3000, get("settings.ping.timeout"))
	assert.Equal(t, 5, get("settings.ping.count"))
	assert.InDelta(t, 4.5, get("settings.ping.delay"), 1e-9)
	assert.Equal(t, 22500, get("settings.trace.timeout"))
	assert.Equal(t, 33434, get("settings.trace.port"))
	assert.Equal(t, 30, get("settings.trace.limit"))
	assert.Equal(t, "www.example.com", get("settings.dns_grid.target"))
	assert.Equal(t, "DNS_RECORD_AAAA", get("settings.dns_grid.record_type"))
	assert.Equal(t, []any{"192.0.2.1", "192.0.2.2"}, get("settings.dns_grid.servers"))
	// Legacy values stay in place (they are still accepted, with a deprecation warning).
	assert.Equal(t, "tcp", get("settings.protocol"))
	// No definition block is invented for a different type.
	_, hasDNS := getPath(got, "settings.dns.servers")
	assert.False(t, hasDNS)
	// url did not exist: the http block is not mapped into it.
	_, hasURL := getPath(got, "settings.url.method")
	assert.False(t, hasURL)

	// The upgraded state must decode with the current schema type.
	b, err := json.Marshal(got)
	require.NoError(t, err)
	_, err = ctyjson.Unmarshal(b, resourceTest().CoreConfigSchema().ImpliedType())
	require.NoError(t, err)
}

func TestTestResourceV0TypeMatchesFixture(t *testing.T) {
	t.Parallel()
	_, err := ctyjson.Unmarshal([]byte(v0State), testResourceV0().CoreConfigSchema().ImpliedType())
	require.NoError(t, err)
}
