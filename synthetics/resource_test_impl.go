package synthetics

// Note the file is named "resource_test_impl.go" instead of "resource_test.go", so that
// Go toolchain does not treat it as test file (because of _test.go suffix).

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

func resourceTest() *schema.Resource {
	return &schema.Resource{
		Description: "Synthetic test (synthetics API v202309). The `type` selects which `settings` block " +
			"configures the test target (for example `type = \"hostname\"` with `settings { hostname { ... } }`). " +
			"Supported types: `" + strings.Join(testTypes, "`, `") + "`. " +
			"Tests can be imported by ID: `terraform import kentik-synthetics_test.example <id>`.",
		CreateContext: resourceTestCreate,
		ReadContext:   resourceTestRead,
		UpdateContext: resourceTestUpdate,
		DeleteContext: resourceTestDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		CustomizeDiff: customizeTestDiff,
		Schema:        makeTestSchema(create),
		SchemaVersion: testSchemaVersion,
		StateUpgraders: []schema.StateUpgrader{{
			Version: 0,
			Type:    testResourceV0().CoreConfigSchema().ImpliedType(),
			Upgrade: upgradeTestStateV0,
		}},
	}
}

func resourceTestCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	test, err := testFromResourceData(d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.admin.CreateTest(ctx, &syntheticspb.CreateTestRequest{Test: test})
	if err != nil {
		return apiError("Failed to create test", err)
	}
	d.SetId(resp.GetTest().GetId())
	tflog.Debug(ctx, "Created test", map[string]any{"id": d.Id()})

	if resp.GetTest().GetStatus() != test.GetStatus() {
		if diags := setTestStatus(ctx, c, d.Id(), test.GetStatus()); diags.HasError() {
			return diags
		}
	}
	return resourceTestRead(ctx, d, m)
}

func resourceTestRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	resp, err := c.admin.GetTest(ctx, &syntheticspb.GetTestRequest{Id: d.Id()})
	if err != nil {
		if isNotFound(err) {
			tflog.Warn(ctx, "Test not found, removing it from state", map[string]any{"id": d.Id()})
			d.SetId("")
			return nil
		}
		return apiError("Failed to read test", err)
	}
	return setTestResourceData(d, resp.GetTest())
}

func resourceTestUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	test, err := testFromResourceData(d)
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChangeExcept("status") {
		if _, err := c.admin.UpdateTest(ctx, &syntheticspb.UpdateTestRequest{Test: test}); err != nil {
			return apiError("Failed to update test", err)
		}
	}
	if d.HasChange("status") {
		if diags := setTestStatus(ctx, c, d.Id(), test.GetStatus()); diags.HasError() {
			return diags
		}
	}
	return resourceTestRead(ctx, d, m)
}

func resourceTestDelete(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	if _, err := c.admin.DeleteTest(ctx, &syntheticspb.DeleteTestRequest{Id: d.Id()}); err != nil && !isNotFound(err) {
		return apiError("Failed to delete test", err)
	}
	return nil
}

func setTestStatus(ctx context.Context, c *apiClient, id string, st syntheticspb.TestStatus) diag.Diagnostics {
	if _, err := c.admin.SetTestStatus(ctx, &syntheticspb.SetTestStatusRequest{Id: id, Status: st}); err != nil {
		return apiError("Failed to set test status", err)
	}
	return nil
}

// testFromResourceData builds the API test from configuration and state.
func testFromResourceData(d *schema.ResourceData) (*syntheticspb.Test, error) {
	s := makeTestSchema(create)
	data := make(map[string]any, len(s))
	for k := range s {
		data[k] = d.Get(k)
	}
	applyLegacyOnWrite(d.GetRawConfig(), data, s)
	data["status"] = normalizeTestStatus(fmt.Sprint(data["status"]))
	ensureEmptyDefinition(data)

	test := &syntheticspb.Test{}
	if err := tfToMessage(data, test.ProtoReflect(), s); err != nil {
		return nil, fmt.Errorf("convert test configuration: %w", err)
	}
	test.Id = d.Id()

	if err := checkDefinition(test); err != nil {
		return nil, err
	}
	return test, nil
}

// ensureEmptyDefinition adds the definition block for types whose block has no required attributes
// (network_mesh), so `network_mesh {}` may be omitted.
func ensureEmptyDefinition(data map[string]any) {
	if data["type"] != "network_mesh" {
		return
	}
	settings := toList(data["settings"])
	if len(settings) == 0 {
		return
	}
	sm, ok := settings[0].(map[string]any)
	if !ok {
		return
	}
	if len(toList(sm["network_mesh"])) == 0 {
		sm["network_mesh"] = []any{map[string]any{}}
	}
}

func checkDefinition(test *syntheticspb.Test) error {
	rm := test.GetSettings().ProtoReflect()
	which := rm.WhichOneof(rm.Descriptor().Oneofs().ByName("definition"))
	if which == nil || string(which.Name()) != test.GetType() {
		return fmt.Errorf("a test of type %q requires the settings.%s block", test.GetType(), test.GetType())
	}
	return nil
}

func customizeTestDiff(_ context.Context, d *schema.ResourceDiff, _ any) error {
	typ, _ := d.Get("type").(string)
	if typ == "" || !d.NewValueKnown("type") || !d.NewValueKnown("settings") {
		return nil
	}
	var set []string
	for _, blk := range definitionBlocks {
		if l, _ := d.Get("settings.0." + blk).([]any); len(l) > 0 {
			set = append(set, blk)
		}
	}
	switch {
	case len(set) > 1:
		return fmt.Errorf("only one test definition block may be set in settings, got: %s", strings.Join(set, ", "))
	case len(set) == 1 && set[0] != typ:
		return fmt.Errorf("test type %q does not match the settings.%s block; use settings.%s", typ, set[0], typ)
	case len(set) == 0 && typ != "network_mesh":
		return fmt.Errorf("a test of type %q requires the settings.%s block", typ, typ)
	}
	return nil
}

func setTestResourceData(d *schema.ResourceData, test *syntheticspb.Test) diag.Diagnostics {
	s := makeTestSchema(create)
	fresh := messageToTF(test.ProtoReflect(), s)
	prior := make(map[string]any, len(s))
	for k := range s {
		prior[k] = d.Get(k)
	}
	preserveLegacy(fresh, prior, s)
	// Keep the user's spelling of the status ("paused" vs "TEST_STATUS_PAUSED") when equivalent.
	if p, _ := prior["status"].(string); p != "" && normalizeTestStatus(p) == fresh["status"] {
		fresh["status"] = p
	}
	delete(fresh, idKey)
	for k, v := range fresh {
		// SDKv2 stores a nil set as an empty set; skip it while the prior value is empty, so an unset
		// attribute stays null instead of showing a null -> [] change on refresh.
		if v == nil && isZero(prior[k]) {
			continue
		}
		if err := d.Set(k, v); err != nil {
			return diag.Errorf("set %s: %v", k, err)
		}
	}
	return nil
}

// testToDataSourceMap converts an API test for data sources, filling deprecated attributes from their
// replacements.
func testToDataSourceMap(test *syntheticspb.Test, mode schemaMode) map[string]any {
	s := makeTestSchema(mode)
	m := messageToTF(test.ProtoReflect(), s)
	m["expires_on"] = ""
	fillLegacy(m, s)
	return m
}
