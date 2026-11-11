package synthetics

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

func dataSourceAgentAlert() *schema.Resource {
	return &schema.Resource{
		Description: "A single agent offline alert, looked up by ID.",
		ReadContext: dataSourceAgentAlertRead,
		Schema:      makeAgentAlertSchema(readSingle),
	}
}

func dataSourceAgentAlertRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	id := d.Get(idKey).(string)
	resp, err := c.admin.GetAgentAlert(ctx, &syntheticspb.GetAgentAlertRequest{Id: id})
	if err != nil {
		return apiError("Failed to read agent alert "+id, err)
	}
	for k, v := range messageToTF(resp.GetAgentAlert().ProtoReflect(), makeAgentAlertSchema(readSingle)) {
		if err := d.Set(k, v); err != nil {
			return diag.Errorf("set %s: %v", k, err)
		}
	}
	d.SetId(resp.GetAgentAlert().GetId())
	return nil
}

func dataSourceAgentAlerts() *schema.Resource {
	return &schema.Resource{
		Description: "Agent offline alerts, optionally limited to some agents.",
		ReadContext: dataSourceAgentAlertsRead,
		Schema: map[string]*schema.Schema{
			"agent_ids": {
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Only return alerts of these agents. All alerts when omitted.",
			},
			itemsKey: {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Agent alerts.",
				Elem:        &schema.Resource{Schema: makeAgentAlertSchema(readList)},
			},
		},
	}
}

func dataSourceAgentAlertsRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	var agentIDs []string
	for _, v := range toList(d.Get("agent_ids")) {
		agentIDs = append(agentIDs, fmt.Sprint(v))
	}
	sort.Strings(agentIDs)
	resp, err := c.admin.ListAgentAlerts(ctx, &syntheticspb.ListAgentAlertsRequest{AgentIds: agentIDs})
	if err != nil {
		return apiError("Failed to list agent alerts", err)
	}
	s := makeAgentAlertSchema(readList)
	items := make([]any, 0, len(resp.GetAgentAlerts()))
	for _, a := range resp.GetAgentAlerts() {
		items = append(items, messageToTF(a.ProtoReflect(), s))
	}
	if err := d.Set(itemsKey, items); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("kentik-synthetics-agent-alerts:" + strings.Join(agentIDs, ","))
	return nil
}
