package kentikapi

import (
	syntheticspb "github.com/kentik/api-schema-public/gen/go/kentik/synthetics/v202309"
)

// Typed accessors for the services used by this module. Add one per service and version
// (see AGENTS.md, "Adding an API service or version").

// SyntheticsAdmin returns a client for kentik.synthetics.v202309.SyntheticsAdminService.
func (c *Client) SyntheticsAdmin() syntheticspb.SyntheticsAdminServiceClient {
	return syntheticspb.NewSyntheticsAdminServiceClient(c.conn)
}

// SyntheticsData returns a client for kentik.synthetics.v202309.SyntheticsDataService.
func (c *Client) SyntheticsData() syntheticspb.SyntheticsDataServiceClient {
	return syntheticspb.NewSyntheticsDataServiceClient(c.conn)
}
