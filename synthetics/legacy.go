package synthetics

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Legacy (v202101beta1) attributes of kentik-synthetics_test that have a v202309 equivalent.
//
// - On create/update, a legacy attribute set in the configuration is used for its replacement when the
//   replacement itself is not configured.
// - On resource read, legacy attributes are not returned by the API, so their values are kept from state
//   (preserveLegacy) to avoid perpetual diffs.
// - On data source read, legacy attributes are filled from their replacement (fillLegacy) so existing
//   references keep working.
// - The V0 -> V1 state upgrader copies legacy values into their replacement.
//
// docs/UPGRADING.md documents the same table; keep both in sync.

type legacyAlias struct {
	from, to string // dotted paths from the test root; blocks are implicit (first element)
}

var testLegacyAliases = []legacyAlias{ //nolint:gochecknoglobals // constant table
	{"settings.monitoring_settings.notification_channels", "settings.notification_channels"},
	{"settings.protocol", "settings.ping.protocol"},
	{"settings.port", "settings.ping.port"},
	{"settings.servers", "settings.dns.servers"},
	{"settings.servers", "settings.dns_grid.servers"},
	{"settings.ping.expiry", "settings.ping.timeout"},
	{"settings.trace.expiry", "settings.trace.timeout"},
	{"settings.http.method", "settings.url.method"},
	{"settings.http.body", "settings.url.body"},
	{"settings.http.headers", "settings.url.headers"},
	{"settings.http.headers", "settings.page_load.headers"},
	{"settings.http.ignore_tls_errors", "settings.url.ignore_tls_errors"},
	{"settings.http.ignore_tls_errors", "settings.page_load.ignore_tls_errors"},
	{"settings.http.css_selectors", "settings.page_load.css_selectors"},
	{"settings.http.expiry", "settings.url.timeout"},
	{"settings.http.expiry", "settings.page_load.timeout"},
	{"settings.dns.type", "settings.dns.record_type"},
	{"settings.dns_grid.type", "settings.dns_grid.record_type"},
	{"settings.dns_grid.targets", "settings.dns_grid.target"},
	{"settings.flow.max_tasks", "settings.flow.max_ip_targets"},
}

// applyLegacyOnWrite copies configured legacy values into unconfigured replacements in data (a test tree
// built from ResourceData).
func applyLegacyOnWrite(raw cty.Value, data map[string]any, s map[string]*schema.Schema) {
	for _, a := range testLegacyAliases {
		if !rawConfigured(raw, a.from) || rawConfigured(raw, a.to) {
			continue
		}
		v, ok := getPath(data, a.from)
		if !ok || isZero(v) {
			continue
		}
		setPath(data, a.to, coerce(v, schemaAt(s, a.to)))
	}
}

// fillLegacy sets legacy attributes from their replacements (used by data sources).
func fillLegacy(data map[string]any, s map[string]*schema.Schema) {
	for _, a := range testLegacyAliases {
		v, ok := getPath(data, a.to)
		if !ok || isZero(v) {
			continue
		}
		if cur, ok := getPath(data, a.from); ok && !isZero(cur) {
			continue
		}
		setPath(data, a.from, coerce(v, schemaAt(s, a.from)))
	}
}

// upgradeLegacy copies legacy values into empty replacements (used by the state upgrader).
func upgradeLegacy(data map[string]any, s map[string]*schema.Schema) {
	for _, a := range testLegacyAliases {
		v, ok := getPath(data, a.from)
		if !ok || isZero(v) {
			continue
		}
		if cur, ok := getPath(data, a.to); ok && !isZero(cur) {
			continue
		}
		setPath(data, a.to, coerce(v, schemaAt(s, a.to)))
	}
}

// preserveLegacy copies attributes that the API did not return (absent from fresh) from prior, recursing
// into nested blocks present in both.
func preserveLegacy(fresh, prior map[string]any, s map[string]*schema.Schema) {
	for k, sch := range s {
		fv, inFresh := fresh[k]
		pv := prior[k]
		if !inFresh {
			if pv != nil {
				fresh[k] = pv
			}
			continue
		}
		sub := resourceSchema(sch)
		if sub == nil {
			continue
		}
		fl, pl := toList(fv), toList(pv)
		for i := range min(len(fl), len(pl)) {
			fm, fok := fl[i].(map[string]any)
			pm, pok := pl[i].(map[string]any)
			if fok && pok {
				preserveLegacy(fm, pm, sub)
			}
		}
	}
}

func getPath(data map[string]any, path string) (any, bool) {
	cur := data
	parts := strings.Split(path, ".")
	for i, p := range parts {
		v, ok := cur[p]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return v, true
		}
		l := toList(v)
		if len(l) == 0 {
			return nil, false
		}
		m, ok := l[0].(map[string]any)
		if !ok {
			return nil, false
		}
		cur = m
	}
	return nil, false
}

// setPath sets a value, creating missing intermediate blocks except definition blocks (creating those
// would change the test type).
func setPath(data map[string]any, path string, v any) {
	cur := data
	parts := strings.Split(path, ".")
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = v
			return
		}
		l := toList(cur[p])
		if len(l) == 0 || l[0] == nil {
			if i > 0 && containsString(definitionBlocks, p) {
				return
			}
			m := map[string]any{}
			cur[p] = []any{m}
			cur = m
			continue
		}
		m, ok := l[0].(map[string]any)
		if !ok {
			return
		}
		cur = m
	}
}

// rawConfigured reports whether path is set (non-null, non-empty) in the raw configuration.
func rawConfigured(raw cty.Value, path string) bool {
	cur := raw
	for _, p := range strings.Split(path, ".") {
		if cur.IsNull() || !cur.IsKnown() || !cur.Type().IsObjectType() || !cur.Type().HasAttribute(p) {
			return false
		}
		cur = cur.GetAttr(p)
		if cur.IsNull() || !cur.IsKnown() {
			return false
		}
		t := cur.Type()
		if t.IsListType() || t.IsSetType() || t.IsTupleType() {
			if cur.LengthInt() == 0 {
				return false
			}
			if et := cur.Index(cty.NumberIntVal(0)); et.Type().IsObjectType() {
				cur = et
			}
		}
	}
	return true
}

func schemaAt(s map[string]*schema.Schema, path string) *schema.Schema {
	parts := strings.Split(path, ".")
	cur := s
	for i, p := range parts {
		sch, ok := cur[p]
		if !ok {
			return nil
		}
		if i == len(parts)-1 {
			return sch
		}
		cur = resourceSchema(sch)
		if cur == nil {
			return nil
		}
	}
	return nil
}

// coerce converts v to the type of the target attribute (int <-> float, list -> first element).
func coerce(v any, target *schema.Schema) any {
	if target == nil {
		return v
	}
	switch target.Type {
	case schema.TypeInt:
		return toInt(v)
	case schema.TypeFloat:
		return toFloat(v)
	case schema.TypeString:
		if l := toList(v); l != nil {
			if len(l) == 0 {
				return ""
			}
			return fmt.Sprint(l[0])
		}
		return v
	case schema.TypeList, schema.TypeSet:
		if l := toList(v); l != nil {
			return l
		}
		return []any{v}
	case schema.TypeBool, schema.TypeMap, schema.TypeInvalid:
	}
	return v
}

func isZero(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case int:
		return t == 0
	case float64:
		return t == 0
	case bool:
		return !t
	case map[string]any:
		return len(t) == 0
	case *schema.Set:
		return t.Len() == 0
	}
	if l := toList(v); l != nil {
		return len(l) == 0
	}
	return false
}
