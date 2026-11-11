package synthetics

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

func makeAgentAlertSchema(mode schemaMode) map[string]*schema.Schema {
	b := newBuilder(mode, md(&syntheticspb.AgentAlert{}))
	return map[string]*schema.Schema{
		idKey: {
			Type:        schema.TypeString,
			Required:    mode == readSingle,
			Computed:    mode != readSingle,
			Description: "ID of the agent alert.",
		},
		"agent_id": b.field("agent_id", schema.TypeString, required, forceNew(),
			description("ID of the agent to watch. Changing it creates a new alert.")),
		"threshold_seconds": b.field("threshold_seconds", schema.TypeInt, required,
			validate(validation.IntAtLeast(1)),
			description("Alert when the agent has been offline (not reporting) for this many seconds.")),
		"notification_channel_ids": b.field("notification_channel_ids", schema.TypeSet, optional,
			setOf(schema.TypeString),
			description("IDs of the notification channels that receive the alert.")),
		"agent_name": b.field("agent_name", schema.TypeString, computedOnly,
			description("Name of the agent, as reported by the API.")),
	}
}

func resourceAgentAlert() *schema.Resource {
	return &schema.Resource{
		Description: "Offline alert for a synthetic monitoring agent: notifies the given channels when the agent " +
			"stops reporting for `threshold_seconds`. Can be imported by alert ID.",
		CreateContext: resourceAgentAlertCreate,
		ReadContext:   resourceAgentAlertRead,
		UpdateContext: resourceAgentAlertUpdate,
		DeleteContext: resourceAgentAlertDelete,
		Importer:      &schema.ResourceImporter{StateContext: schema.ImportStatePassthroughContext},
		Schema:        makeAgentAlertSchema(create),
	}
}

func channelIDs(d *schema.ResourceData) []string {
	var ids []string
	for _, v := range toList(d.Get("notification_channel_ids")) {
		ids = append(ids, fmt.Sprint(v))
	}
	return ids
}

func threshold(d *schema.ResourceData) uint32 {
	return uint32(d.Get("threshold_seconds").(int)) //nolint:gosec // validated >= 1
}

func resourceAgentAlertCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	resp, err := c.admin.CreateAgentAlert(ctx, &syntheticspb.CreateAgentAlertRequest{
		AgentId:                d.Get("agent_id").(string),
		ThresholdSeconds:       threshold(d),
		NotificationChannelIds: channelIDs(d),
	})
	if err != nil {
		return apiError("Failed to create agent alert", err)
	}
	d.SetId(resp.GetAgentAlert().GetId())
	return resourceAgentAlertRead(ctx, d, m)
}

func resourceAgentAlertRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	resp, err := c.admin.GetAgentAlert(ctx, &syntheticspb.GetAgentAlertRequest{Id: d.Id()})
	if err != nil {
		if isNotFound(err) {
			tflog.Warn(ctx, "Agent alert not found, removing it from state", map[string]any{"id": d.Id()})
			d.SetId("")
			return nil
		}
		return apiError("Failed to read agent alert "+d.Id(), err)
	}
	for k, v := range messageToTF(resp.GetAgentAlert().ProtoReflect(), makeAgentAlertSchema(create)) {
		if k == idKey {
			continue
		}
		if err := d.Set(k, v); err != nil {
			return diag.Errorf("set %s: %v", k, err)
		}
	}
	return nil
}

func resourceAgentAlertUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	_, err := c.admin.UpdateAgentAlert(ctx, &syntheticspb.UpdateAgentAlertRequest{
		Id:                     d.Id(),
		ThresholdSeconds:       threshold(d),
		NotificationChannelIds: channelIDs(d),
	})
	if err != nil {
		return apiError("Failed to update agent alert "+d.Id(), err)
	}
	return resourceAgentAlertRead(ctx, d, m)
}

func resourceAgentAlertDelete(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	_, err := c.admin.DeleteAgentAlert(ctx, &syntheticspb.DeleteAgentAlertRequest{Id: d.Id()})
	if err != nil && !isNotFound(err) {
		return apiError("Failed to delete agent alert "+d.Id(), err)
	}
	return nil
}
