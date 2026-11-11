package synthetics

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// upgradeTestStateV0 upgrades kentik-synthetics_test state written by provider <= 0.2.x
// (synthetics v202101beta1) to schema version 1 (synthetics v202309):
//   - legacy values are copied to their v202309 replacements (see testLegacyAliases),
//   - numbers that became integers (ping.count, trace.count, trace.port, trace.limit, ...) are rounded,
//   - an empty status becomes TEST_STATUS_ACTIVE.
func upgradeTestStateV0(_ context.Context, raw map[string]any, _ any) (map[string]any, error) {
	if raw == nil {
		return raw, nil
	}
	s := makeTestSchema(create)
	upgradeLegacy(raw, s)
	coerceToSchema(raw, s)
	if st, _ := raw["status"].(string); st == "" {
		raw["status"] = statusActive
	} else {
		raw["status"] = normalizeTestStatus(st)
	}
	return raw, nil
}

// coerceToSchema rounds floats stored for integer attributes, recursing into nested blocks.
func coerceToSchema(data map[string]any, s map[string]*schema.Schema) {
	for k, v := range data {
		sch, ok := s[k]
		if !ok || v == nil {
			continue
		}
		switch sch.Type {
		case schema.TypeInt:
			if f, ok := v.(float64); ok {
				data[k] = toInt(f)
			}
		case schema.TypeList, schema.TypeSet:
			l, ok := v.([]any)
			if !ok {
				continue
			}
			switch elem := sch.Elem.(type) {
			case *schema.Resource:
				for _, item := range l {
					if m, ok := item.(map[string]any); ok {
						coerceToSchema(m, elem.Schema)
					}
				}
			case *schema.Schema:
				if elem.Type == schema.TypeInt {
					for i, item := range l {
						if f, ok := item.(float64); ok {
							l[i] = toInt(f)
						}
					}
				}
			}
		case schema.TypeBool, schema.TypeFloat, schema.TypeString, schema.TypeMap, schema.TypeInvalid:
		}
	}
}

// testResourceV0 is the kentik-synthetics_test schema of provider <= 0.2.x. Only its type matters (it
// decodes stored state for the upgrader), so descriptions and validation are omitted.
func testResourceV0() *schema.Resource {
	str := func() *schema.Schema { return &schema.Schema{Type: schema.TypeString, Optional: true} }
	num := func() *schema.Schema { return &schema.Schema{Type: schema.TypeInt, Optional: true} }
	flt := func() *schema.Schema { return &schema.Schema{Type: schema.TypeFloat, Optional: true} }
	boo := func() *schema.Schema { return &schema.Schema{Type: schema.TypeBool, Optional: true} }
	list := func(t schema.ValueType) *schema.Schema {
		return &schema.Schema{Type: schema.TypeList, Optional: true, Elem: &schema.Schema{Type: t}}
	}
	mp := func() *schema.Schema {
		return &schema.Schema{Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}}
	}
	blk := func(fields map[string]*schema.Schema) *schema.Schema {
		return &schema.Schema{Type: schema.TypeList, Optional: true, Elem: &schema.Resource{Schema: fields}}
	}
	target := func() *schema.Schema { return blk(map[string]*schema.Schema{"target": str()}) }
	targets := func() *schema.Schema { return blk(map[string]*schema.Schema{"targets": list(schema.TypeString)}) }
	user := func() *schema.Schema {
		return blk(map[string]*schema.Schema{"id": str(), "email": str(), "full_name": str()})
	}

	health := map[string]*schema.Schema{
		"http_valid_codes": list(schema.TypeInt),
		"dns_valid_codes":  list(schema.TypeInt),
	}
	for _, n := range []string{
		"latency_critical", "latency_warning", "packet_loss_critical", "packet_loss_warning",
		"jitter_critical", "jitter_warning", "http_latency_critical", "http_latency_warning",
		"latency_critical_stddev", "latency_warning_stddev", "jitter_critical_stddev", "jitter_warning_stddev",
		"http_latency_critical_stddev", "http_latency_warning_stddev",
	} {
		health[n] = flt()
	}

	settings := map[string]*schema.Schema{
		"hostname": target(),
		"ip":       targets(),
		"agent":    target(),
		"flow": blk(map[string]*schema.Schema{
			"target": str(), "target_refresh_interval_millis": num(), "max_tasks": num(), "type": str(),
			"inet_direction": str(), "direction": str(),
		}),
		"site":            target(),
		"tag":             target(),
		"dns":             blk(map[string]*schema.Schema{"target": str(), "type": str()}),
		"url":             target(),
		"network_grid":    targets(),
		"page_load":       target(),
		"dns_grid":        blk(map[string]*schema.Schema{"targets": list(schema.TypeString), "type": str()}),
		"agent_ids":       list(schema.TypeString),
		"period":          num(),
		"count":           num(),
		"expiry":          num(),
		"limit":           num(),
		"tasks":           list(schema.TypeString),
		"health_settings": blk(health),
		"monitoring_settings": blk(map[string]*schema.Schema{
			"notification_channels": list(schema.TypeString),
		}),
		"ping": blk(map[string]*schema.Schema{"period": flt(), "count": flt(), "expiry": flt(), "delay": flt()}),
		"trace": blk(map[string]*schema.Schema{
			"period": flt(), "count": flt(), "protocol": str(), "port": flt(), "expiry": flt(), "limit": flt(),
			"delay": flt(),
		}),
		"port":         num(),
		"protocol":     str(),
		"family":       str(),
		"servers":      list(schema.TypeString),
		"rollup_level": num(),
		"http": blk(map[string]*schema.Schema{
			"period": num(), "expiry": num(), "method": str(), "headers": mp(), "body": str(),
			"ignore_tls_errors": boo(), "css_selectors": mp(),
		}),
	}

	return &schema.Resource{Schema: map[string]*schema.Schema{
		"id":              {Type: schema.TypeString, Computed: true},
		"name":            str(),
		"type":            str(),
		"status":          str(),
		"settings":        blk(settings),
		"expires_on":      str(),
		"cdate":           str(),
		"edate":           str(),
		"created_by":      user(),
		"last_updated_by": user(),
	}}
}
