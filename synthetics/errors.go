package synthetics

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// apiError converts a gRPC error into a Terraform diagnostic with the status code and message.
func apiError(summary string, err error) diag.Diagnostics {
	st, ok := status.FromError(err)
	if !ok {
		return diag.Diagnostics{{Severity: diag.Error, Summary: summary, Detail: err.Error()}}
	}
	detail := fmt.Sprintf("Kentik API returned %s: %s", st.Code(), st.Message())
	switch st.Code() {
	case codes.Unauthenticated:
		detail += "\nCheck the provider email and token (KTAPI_AUTH_EMAIL / KTAPI_AUTH_TOKEN)."
	case codes.PermissionDenied:
		detail += "\nThe API token lacks the permission required for this operation."
	default:
	}
	return diag.Diagnostics{{Severity: diag.Error, Summary: summary, Detail: detail}}
}

func isNotFound(err error) bool { return status.Code(err) == codes.NotFound }
