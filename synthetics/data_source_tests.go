package synthetics

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

const (
	itemsKey             = "items"
	invalidTestsCountKey = "invalid_tests_count"
)

func dataSourceTests() *schema.Resource {
	return &schema.Resource{
		Description: "All active and paused synthetic tests of the account.",
		ReadContext: dataSourceTestsRead,
		Schema: map[string]*schema.Schema{
			itemsKey: {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Tests.",
				Elem:        &schema.Resource{Schema: makeTestSchema(readList)},
			},
			invalidTestsCountKey: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of invalid entries encountered while collecting data.",
			},
		},
	}
}

func dataSourceTestsRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	resp, err := c.admin.ListTests(ctx, &syntheticspb.ListTestsRequest{})
	if err != nil {
		return apiError("Failed to list tests", err)
	}
	items := make([]any, 0, len(resp.GetTests()))
	for _, t := range resp.GetTests() {
		items = append(items, testToDataSourceMap(t, readList))
	}
	if err := d.Set(itemsKey, items); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(invalidTestsCountKey, int(resp.GetInvalidCount())); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("kentik-synthetics-tests")
	return nil
}
