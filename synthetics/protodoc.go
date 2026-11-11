package synthetics

import (
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// protoFieldDescription returns the OpenAPI description annotated on a proto field in api-schema-public.
func protoFieldDescription(fd protoreflect.FieldDescriptor) string {
	opts := fd.Options()
	if opts == nil {
		return ""
	}
	ext, ok := proto.GetExtension(opts, options.E_Openapiv2Field).(*options.JSONSchema)
	if !ok || ext == nil {
		return ""
	}
	d := strings.TrimSpace(ext.GetDescription())
	if d != "" && !strings.HasSuffix(d, ".") {
		d += "."
	}
	return d
}
