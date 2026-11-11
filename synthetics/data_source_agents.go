package synthetics

import (
	"context"
	"math"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	geo "github.com/kellydunn/golang-geo"
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

const (
	invalidAgentsCountKey = "invalid_agents_count"
	latitudeKey           = "latitude"
	longitudeKey          = "longitude"
	minDistanceKey        = "min_distance"
	maxDistanceKey        = "max_distance"
)

func dataSourceAgents() *schema.Resource {
	return &schema.Resource{
		Description: "All synthetic monitoring agents available to the account, optionally filtered by " +
			"great-circle distance from a reference point.",
		ReadContext: dataSourceAgentsRead,
		Schema: map[string]*schema.Schema{
			itemsKey: {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Agents.",
				Elem:        &schema.Resource{Schema: makeAgentSchema(readList)},
			},
			invalidAgentsCountKey: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of invalid entries encountered while collecting data.",
			},
			latitudeKey: {
				Type:         schema.TypeFloat,
				Optional:     true,
				RequiredWith: []string{longitudeKey},
				Description:  "Latitude of the reference point for distance filtering (signed decimal degrees).",
			},
			longitudeKey: {
				Type:         schema.TypeFloat,
				Optional:     true,
				RequiredWith: []string{latitudeKey},
				Description:  "Longitude of the reference point for distance filtering (signed decimal degrees).",
			},
			minDistanceKey: {
				Type:        schema.TypeFloat,
				Optional:    true,
				Description: "Minimum distance from the reference point, in kilometers (default 0).",
			},
			maxDistanceKey: {
				Type:        schema.TypeFloat,
				Optional:    true,
				Description: "Maximum distance from the reference point, in kilometers (default unlimited).",
			},
		},
	}
}

func dataSourceAgentsRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	c := m.(*apiClient)
	resp, err := c.admin.ListAgents(ctx, &syntheticspb.ListAgentsRequest{})
	if err != nil {
		return apiError("Failed to list agents", err)
	}

	s := makeAgentSchema(readList)
	agents := filterAgents(resp.GetAgents(), d)
	items := make([]any, 0, len(agents))
	for _, a := range agents {
		items = append(items, agentToTF(a, s))
	}
	if err := d.Set(itemsKey, items); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(invalidAgentsCountKey, int(resp.GetInvalidCount())); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("kentik-synthetics-agents")
	return nil
}

func filterAgents(agents []*syntheticspb.Agent, d *schema.ResourceData) []*syntheticspb.Agent {
	lat, latOK := d.GetOk(latitudeKey)
	lon, lonOK := d.GetOk(longitudeKey)
	if !latOK || !lonOK {
		return agents
	}
	minDist, maxDist := 0.0, math.Inf(1)
	if v, ok := d.GetOk(minDistanceKey); ok {
		minDist = v.(float64)
	}
	if v, ok := d.GetOk(maxDistanceKey); ok {
		maxDist = v.(float64)
	}

	ref := geo.NewPoint(lat.(float64), lon.(float64))
	var out []*syntheticspb.Agent
	for _, a := range agents {
		dist := ref.GreatCircleDistance(geo.NewPoint(a.GetLat(), a.GetLong()))
		if dist >= minDist && dist <= maxDist {
			out = append(out, a)
		}
	}
	return out
}
