package synthetics

// Note the file is named "data_source_test_impl.go" instead of "data_source_test.go", so that
// Go toolchain does not treat it as test file (because of _test.go suffix).

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

func dataSourceTest() *schema.Resource {
	return &schema.Resource{
		Description: "A single synthetic test, looked up by ID.",
		ReadContext: dataSourceTestRead,
		Schema:      makeTestSchema(readSingle),
	}
}

func dataSourceTestRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	id := d.Get(idKey).(string)
	resp, err := c.admin.GetTest(ctx, &syntheticspb.GetTestRequest{Id: id})
	if err != nil {
		return apiError("Failed to read test "+id, err)
	}
	for k, v := range testToDataSourceMap(resp.GetTest(), readSingle) {
		if err := d.Set(k, v); err != nil {
			return diag.Errorf("set %s: %v", k, err)
		}
	}
	d.SetId(resp.GetTest().GetId())
	return nil
}
