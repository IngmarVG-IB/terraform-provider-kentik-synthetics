package synthetics

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

func dataSourceAgent() *schema.Resource {
	return &schema.Resource{
		Description: "A single synthetic monitoring agent (global or private), looked up by ID.",
		ReadContext: dataSourceAgentRead,
		Schema:      makeAgentSchema(readSingle),
	}
}

func dataSourceAgentRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	id := d.Get(idKey).(string)
	resp, err := c.admin.GetAgent(ctx, &syntheticspb.GetAgentRequest{Id: id})
	if err != nil {
		return apiError("Failed to read agent "+id, err)
	}
	for k, v := range agentToTF(resp.GetAgent(), makeAgentSchema(readSingle)) {
		if err := d.Set(k, v); err != nil {
			return diag.Errorf("set %s: %v", k, err)
		}
	}
	d.SetId(resp.GetAgent().GetId())
	return nil
}
