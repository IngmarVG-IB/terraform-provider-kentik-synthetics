package synthetics

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const idKey = "id"

// schemaMode selects how one shared schema definition is rendered:
//   - create: resource schema; attributes are Required, Optional or Optional+Computed as declared,
//   - readSingle: single-item data source; "id" is Required, everything else Computed,
//   - readList: list data source; everything Computed.
type schemaMode int

const (
	readSingle schemaMode = iota
	readList
	create
)

// attrKind is how an attribute behaves in a resource (create mode). In read modes every attribute is Computed.
type attrKind int

const (
	required attrKind = iota
	optional
	optionalComputed
	computedOnly
)

type schemaOpt func(*schema.Schema)

func listOf(t schema.ValueType) schemaOpt {
	return func(s *schema.Schema) { s.Type, s.Elem = schema.TypeList, &schema.Schema{Type: t} }
}

func setOf(t schema.ValueType) schemaOpt {
	return func(s *schema.Schema) { s.Type, s.Elem = schema.TypeSet, &schema.Schema{Type: t} }
}

func mapOfString() schemaOpt {
	return func(s *schema.Schema) { s.Type, s.Elem = schema.TypeMap, &schema.Schema{Type: schema.TypeString} }
}

func deprecated(msg string) schemaOpt { return func(s *schema.Schema) { s.Deprecated = msg } }

func description(d string) schemaOpt { return func(s *schema.Schema) { s.Description = d } }

func forceNew() schemaOpt { return func(s *schema.Schema) { s.ForceNew = true } }

func withDefault(v any) schemaOpt { return func(s *schema.Schema) { s.Default = v } }

func validate(f func(any, string) ([]string, []error)) schemaOpt {
	return func(s *schema.Schema) { s.ValidateFunc = f }
}

// oneOf restricts a string attribute to the given values and documents them.
func oneOf(values ...string) schemaOpt {
	return func(s *schema.Schema) {
		s.ValidateFunc = validation.StringInSlice(values, false)
		s.Description = strings.TrimSpace(s.Description) + " Valid values: `" + strings.Join(values, "`, `") + "`."
	}
}

// enumOf restricts a string attribute to the names of a proto enum, excluding the zero (*_UNSPECIFIED) value.
func enumOf(ed protoreflect.EnumDescriptor, exclude ...string) schemaOpt {
	var names []string
	for i := range ed.Values().Len() {
		v := ed.Values().Get(i)
		name := string(v.Name())
		if v.Number() == 0 || containsString(exclude, name) {
			continue
		}
		names = append(names, name)
	}
	return oneOf(names...)
}

// finalize applies mode and kind to s. In read modes all constraints that only make sense for configurable
// attributes are dropped.
func finalize(mode schemaMode, kind attrKind, s *schema.Schema) *schema.Schema {
	if mode != create || kind == computedOnly {
		s.Required, s.Optional, s.Computed = false, false, true
		s.Default, s.DiffSuppressFunc, s.ForceNew = nil, nil, false
		s.ValidateFunc, s.ValidateDiagFunc = nil, nil
		s.MaxItems, s.MinItems = 0, 0
		return s
	}
	switch kind {
	case required:
		s.Required = true
	case optional:
		s.Optional = true
	case optionalComputed:
		s.Optional, s.Computed = true, true
	case computedOnly:
	}
	return s
}

// builder creates attributes whose descriptions come from the matching field of a proto message, so
// the registry docs follow api-schema-public.
type builder struct {
	mode schemaMode
	msg  protoreflect.MessageDescriptor
}

func newBuilder(mode schemaMode, msg protoreflect.MessageDescriptor) builder {
	return builder{mode: mode, msg: msg}
}

func (b builder) field(name string, typ schema.ValueType, kind attrKind, opts ...schemaOpt) *schema.Schema {
	fd := b.msg.Fields().ByName(protoreflect.Name(name))
	if fd == nil {
		panic(fmt.Sprintf("schema attribute %q has no field in proto message %s", name, b.msg.FullName()))
	}
	s := &schema.Schema{Type: typ, Description: protoFieldDescription(fd)}
	for _, o := range opts {
		o(s)
	}
	return finalize(b.mode, kind, s)
}

// legacy declares a deprecated attribute that has no proto field in the current API version.
// Its value is kept in state (or mapped, see legacy.go) but never read from the API.
func (b builder) legacy(name string, typ schema.ValueType, msg string, opts ...schemaOpt) *schema.Schema {
	_ = name
	s := &schema.Schema{Type: typ, Description: "**Deprecated.** " + msg, Deprecated: msg}
	for _, o := range opts {
		o(s)
	}
	return finalize(b.mode, optional, s)
}

// block declares a nested object (a list with at most one element) backed by proto message field name.
func (b builder) block(
	name string, kind attrKind, fields func(builder) map[string]*schema.Schema, opts ...schemaOpt,
) *schema.Schema {
	fd := b.msg.Fields().ByName(protoreflect.Name(name))
	if fd == nil || fd.Message() == nil {
		panic(fmt.Sprintf("schema block %q has no message field in proto message %s", name, b.msg.FullName()))
	}
	s := &schema.Schema{
		Type:        schema.TypeList,
		MaxItems:    1,
		Description: protoFieldDescription(fd),
		Elem:        &schema.Resource{Schema: fields(newBuilder(b.mode, fd.Message()))},
	}
	for _, o := range opts {
		o(s)
	}
	return finalize(b.mode, kind, s)
}

// blockList declares a repeated nested object backed by a repeated message field.
func (b builder) blockList(
	name string, kind attrKind, fields func(builder) map[string]*schema.Schema, opts ...schemaOpt,
) *schema.Schema {
	s := b.block(name, kind, fields, opts...)
	s.MaxItems = 0
	return s
}

// legacyBlock declares a deprecated nested object that has no proto field in the current API version.
func (b builder) legacyBlock(msg string, fields map[string]*schema.Schema) *schema.Schema {
	s := &schema.Schema{
		Type:        schema.TypeList,
		MaxItems:    1,
		Description: "**Deprecated.** " + msg,
		Deprecated:  msg,
		Elem:        &schema.Resource{Schema: fields},
	}
	return finalize(b.mode, optional, s)
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
