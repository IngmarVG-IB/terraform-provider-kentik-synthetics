package synthetics

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

const (
	agentIDKey         = "agent_id"
	deleteOnDestroyKey = "delete_on_destroy"
)

// agentWritable are the agent attributes the resource sends with UpdateAgent when they are configured.
var agentWritable = []string{ //nolint:gochecknoglobals // constant list
	"site_name", "status", "alias", "lat", "long", "family", "site_id", "city", "region", "country", "labels",
}

func resourceAgent() *schema.Resource {
	s := makeAgentSchema(create)
	s[agentIDKey] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		ForceNew:    true,
		Description: "ID of an existing agent to manage. The agent must already be registered with Kentik.",
	}
	s[deleteOnDestroyKey] = &schema.Schema{
		Type:     schema.TypeBool,
		Optional: true,
		Default:  false,
		Description: "Delete the agent from Kentik when the resource is destroyed. Default `false`: destroying " +
			"the resource only removes it from Terraform state. A deleted agent is removed from all tests and " +
			"has to be re-registered (re-installed) to come back.",
	}
	return &schema.Resource{
		Description: "Manages an existing synthetic monitoring agent. Agents cannot be created through the Kentik " +
			"API: they register themselves when installed. This resource adopts an existing agent (by " +
			"`agent_id`, or with `terraform import`) and manages the attributes that the UpdateAgent API call " +
			"accepts: `alias`, `site_id`, `site_name`, `labels`, `status` (set `AGENT_STATUS_OK` to authorize " +
			"a waiting private agent), `family` and location. Attributes that are not configured are left " +
			"unchanged. Global (Kentik-managed) agents are read-only.",
		CreateContext: resourceAgentCreate,
		ReadContext:   resourceAgentRead,
		UpdateContext: resourceAgentUpdate,
		DeleteContext: resourceAgentDelete,
		Importer:      &schema.ResourceImporter{StateContext: schema.ImportStatePassthroughContext},
		Schema:        s,
	}
}

func resourceAgentCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	id := d.Get(agentIDKey).(string)
	resp, err := c.admin.GetAgent(ctx, &syntheticspb.GetAgentRequest{Id: id})
	if err != nil {
		if isNotFound(err) {
			return diag.Diagnostics{{
				Severity: diag.Error,
				Summary:  "Agent " + id + " not found",
				Detail: "kentik-synthetics_agent manages existing agents only. Install and register the agent " +
					"first; agents cannot be created through the Kentik API.",
			}}
		}
		return apiError("Failed to read agent "+id, err)
	}
	d.SetId(id)
	if diags := updateAgent(ctx, c, d, resp.GetAgent()); diags.HasError() {
		return diags
	}
	return resourceAgentRead(ctx, d, m)
}

func resourceAgentRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	resp, err := c.admin.GetAgent(ctx, &syntheticspb.GetAgentRequest{Id: d.Id()})
	if err != nil {
		if isNotFound(err) {
			tflog.Warn(ctx, "Agent not found, removing it from state", map[string]any{"id": d.Id()})
			d.SetId("")
			return nil
		}
		return apiError("Failed to read agent "+d.Id(), err)
	}
	s := makeAgentSchema(create)
	for k, v := range agentToTF(resp.GetAgent(), s) {
		if k == idKey {
			continue
		}
		if err := d.Set(k, v); err != nil {
			return diag.Errorf("set %s: %v", k, err)
		}
	}
	if err := d.Set(agentIDKey, d.Id()); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(deleteOnDestroyKey, d.Get(deleteOnDestroyKey)); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceAgentUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	if d.HasChanges(agentWritable...) {
		resp, err := c.admin.GetAgent(ctx, &syntheticspb.GetAgentRequest{Id: d.Id()})
		if err != nil {
			return apiError("Failed to read agent "+d.Id(), err)
		}
		if diags := updateAgent(ctx, c, d, resp.GetAgent()); diags.HasError() {
			return diags
		}
	}
	return resourceAgentRead(ctx, d, m)
}

// updateAgent overlays the configured writable attributes on the current agent and calls UpdateAgent when
// anything differs.
func updateAgent(ctx context.Context, c *apiClient, d *schema.ResourceData, cur *syntheticspb.Agent) diag.Diagnostics {
	raw := d.GetRawConfig()
	s := makeAgentSchema(create)
	next, ok := cloneMessage(cur)
	if !ok {
		return diag.Errorf("clone agent")
	}
	data := map[string]any{}
	for _, k := range agentWritable {
		if k == "labels" {
			continue
		}
		if rawConfigured(raw, k) {
			data[k] = d.Get(k)
		}
	}
	if err := tfToMessage(data, next.ProtoReflect(), s); err != nil {
		return diag.FromErr(fmt.Errorf("convert agent configuration: %w", err))
	}
	// labels: an explicitly empty set clears them.
	if lv := raw.GetAttr("labels"); !lv.IsNull() && lv.IsKnown() {
		next.Labels = nil
		for _, l := range toList(d.Get("labels")) {
			next.Labels = append(next.Labels, fmt.Sprint(l))
		}
	}
	if protoEqual(cur, next) {
		return nil
	}
	if _, err := c.admin.UpdateAgent(ctx, &syntheticspb.UpdateAgentRequest{Agent: next}); err != nil {
		return apiError("Failed to update agent "+cur.GetId(), err)
	}
	return nil
}

func resourceAgentDelete(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	if !d.Get(deleteOnDestroyKey).(bool) {
		return diag.Diagnostics{{
			Severity: diag.Warning,
			Summary:  "Agent " + d.Id() + " was removed from Terraform state only",
			Detail: "The agent still exists in Kentik. Set delete_on_destroy = true to delete agents on destroy " +
				"(a deleted agent is removed from all tests and must be re-registered).",
		}}
	}
	c := m.(*apiClient)
	if _, err := c.admin.DeleteAgent(ctx, &syntheticspb.DeleteAgentRequest{Id: d.Id()}); err != nil && !isNotFound(err) {
		return apiError("Failed to delete agent "+d.Id(), err)
	}
	return nil
}
