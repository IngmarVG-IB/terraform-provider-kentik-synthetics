package synthetics

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProtoConv_TestRoundTrip(t *testing.T) {
	t.Parallel()
	in := &syntheticspb.Test{
		Name:   "rt",
		Type:   "url",
		Status: syntheticspb.TestStatus_TEST_STATUS_PAUSED,
		Labels: []string{"a", "b"},
		Settings: &syntheticspb.TestSettings{
			Definition: &syntheticspb.TestSettings_Url{Url: &syntheticspb.UrlTest{
				Target: "https://example.com", Method: "POST", Headers: map[string]string{"X-A": "1"}, Timeout: 5000,
			}},
			AgentIds: []string{"1", "2"},
			Tasks:    []string{"http"},
			Period:   120,
			Family:   syntheticspb.IPFamily_IP_FAMILY_V6,
			HealthSettings: &syntheticspb.HealthSettings{
				LatencyCritical: 0.1, HttpValidCodes: []uint32{200, 301},
				DisabledMetrics: &syntheticspb.DisabledMetrics{DnsIps: true},
			},
			Ping: &syntheticspb.TestPingSettings{Count: 3, Delay: 2.5, Protocol: "icmp"},
			Alerting: &syntheticspb.AlertingSettings{
				AlertingType: syntheticspb.AlertingType_ALERTING_TYPE_GROUPED,
				GroupedAlertSettings: &syntheticspb.GroupedAlertSettings{
					Overrides: []*syntheticspb.GroupedAlertSetting{{Metric: "jitter", FilterIds: []uint32{7}}},
				},
			},
		},
	}
	s := makeTestSchema(create)
	m := messageToTF(in.ProtoReflect(), s)

	assert.Equal(t, "TEST_STATUS_PAUSED", m["status"])
	hs, _ := getPath(m, "settings.health_settings.latency_critical")
	assert.InDelta(t, 0.1, hs, 0, "float32 must convert via its shortest decimal form")
	fam, _ := getPath(m, "settings.family")
	assert.Equal(t, "IP_FAMILY_V6", fam)
	dns, _ := getPath(m, "settings.dns")
	assert.Equal(t, []any{}, dns, "unset oneof member is an empty block list")

	out := &syntheticspb.Test{}
	require.NoError(t, tfToMessage(m, out.ProtoReflect(), s))
	assert.True(t, proto.Equal(in, out), "round trip mismatch:\n in: %v\nout: %v", in, out)
}

func TestProtoConv_OutputOnlyFieldsAreNotSent(t *testing.T) {
	t.Parallel()
	in := &syntheticspb.Test{
		Name:  "x",
		Cdate: timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
	}
	s := makeTestSchema(create)
	m := messageToTF(in.ProtoReflect(), s)
	assert.Equal(t, "2026-01-02T03:04:05Z", m["cdate"])

	out := &syntheticspb.Test{}
	require.NoError(t, tfToMessage(m, out.ProtoReflect(), s))
	assert.Nil(t, out.GetCdate())
}

func TestProtoConv_InvalidEnum(t *testing.T) {
	t.Parallel()
	s := makeTestSchema(create)
	err := tfToMessage(map[string]any{"status": "NOPE"}, (&syntheticspb.Test{}).ProtoReflect(), s)
	require.ErrorContains(t, err, `invalid value "NOPE"`)
}

func TestProtoConv_AgentWrapperMessages(t *testing.T) {
	t.Parallel()
	a := &syntheticspb.Agent{
		Id: "1", SiteName: "site", AgentImpl: syntheticspb.ImplementType_IMPLEMENT_TYPE_NETWORK,
		Metadata: &syntheticspb.AgentMetadata{
			PrivateIpv4Addresses: []*syntheticspb.AgentMetadata_IpValue{{Value: "10.0.0.1"}, {Value: "10.0.0.2"}},
		},
	}
	m := agentToTF(a, makeAgentSchema(readSingle))
	assert.Equal(t, "site", m["name"], "deprecated name mirrors site_name")
	assert.Equal(t, "IMPLEMENT_TYPE_NETWORK", m["agent_impl"])
	ips, _ := getPath(m, "metadata.private_ipv4_addresses")
	assert.Equal(t, []any{"10.0.0.1", "10.0.0.2"}, ips)
}

func TestSchema_EveryAttributeHasADescription(t *testing.T) {
	t.Parallel()
	p := NewProvider()
	var missing []string
	check := func(kind string, m map[string]*schema.Resource) {
		for name, r := range m {
			if r.Description == "" {
				missing = append(missing, kind+" "+name)
			}
			for path, sch := range flattenSchema("", r.Schema) {
				if sch.Description == "" {
					missing = append(missing, kind+" "+name+": "+path)
				}
			}
		}
	}
	check("resource", p.ResourcesMap)
	check("data source", p.DataSourcesMap)
	for path, sch := range flattenSchema("", p.Schema) {
		if sch.Description == "" {
			missing = append(missing, "provider: "+path)
		}
	}
	assert.Empty(t, missing, "add a description to the proto field or with description()")
}

func flattenSchema(prefix string, s map[string]*schema.Schema) map[string]*schema.Schema {
	out := map[string]*schema.Schema{}
	for k, v := range s {
		out[prefix+k] = v
		if sub := resourceSchema(v); sub != nil {
			for pk, pv := range flattenSchema(prefix+k+".", sub) {
				out[pk] = pv
			}
		}
	}
	return out
}
