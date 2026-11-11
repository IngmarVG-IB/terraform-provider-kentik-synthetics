package synthetics

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The functions in this file convert between generated proto messages and Terraform values generically,
// using protobuf reflection. A Terraform attribute maps to the proto field with the same (snake_case) name;
// attributes without a matching field (deprecated legacy attributes) are skipped and handled in legacy.go.
//
// Mapping:
//   - scalars map to scalars (float32 is converted via its shortest decimal form to avoid 0.1 -> 0.10000000149),
//   - enums map to their value name ("" for the zero value),
//   - google.protobuf.Timestamp maps to an RFC 3339 string,
//   - map<string,string> maps to TypeMap,
//   - messages map to a list with one object; repeated messages map to a list of objects,
//     except one-field wrapper messages (e.g. AgentMetadata.IpValue) used with a primitive Elem, which map
//     to a list of strings.

const timestampFullName = "google.protobuf.Timestamp"

func messageToTF(m protoreflect.Message, s map[string]*schema.Schema) map[string]any {
	out := make(map[string]any, len(s))
	fields := m.Descriptor().Fields()
	for name, sch := range s {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil {
			continue
		}
		out[name] = fieldToTF(m, fd, sch)
	}
	return out
}

func fieldToTF(m protoreflect.Message, fd protoreflect.FieldDescriptor, sch *schema.Schema) any {
	switch {
	case fd.IsMap():
		// Empty collections are returned as null so that unset Optional attributes do not show a diff.
		if m.Get(fd).Map().Len() == 0 {
			return nil
		}
		res := map[string]any{}
		m.Get(fd).Map().Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
			res[k.String()] = v.String()
			return true
		})
		return res
	case fd.IsList():
		l := m.Get(fd).List()
		if l.Len() == 0 {
			return nil
		}
		res := make([]any, 0, l.Len())
		for i := range l.Len() {
			res = append(res, valueToTF(fd, l.Get(i), sch.Elem))
		}
		return res
	case fd.Kind() == protoreflect.MessageKind && fd.Message().FullName() == timestampFullName:
		if !m.Has(fd) {
			return ""
		}
		ts, ok := m.Get(fd).Message().Interface().(*timestamppb.Timestamp)
		if !ok {
			return ""
		}
		return ts.AsTime().UTC().Format(time.RFC3339Nano)
	case fd.Kind() == protoreflect.MessageKind:
		if !m.Has(fd) {
			return []any{}
		}
		return []any{messageToTF(m.Get(fd).Message(), resourceSchema(sch))}
	default:
		return scalarToTF(fd, m.Get(fd))
	}
}

func valueToTF(fd protoreflect.FieldDescriptor, v protoreflect.Value, elem any) any {
	if fd.Kind() != protoreflect.MessageKind {
		return scalarToTF(fd, v)
	}
	if r, ok := elem.(*schema.Resource); ok {
		return messageToTF(v.Message(), r.Schema)
	}
	// One-field wrapper message rendered as a primitive.
	wm := v.Message()
	inner := wm.Descriptor().Fields().Get(0)
	return scalarToTF(inner, wm.Get(inner))
}

func scalarToTF(fd protoreflect.FieldDescriptor, v protoreflect.Value) any {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return v.Bool()
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return int(v.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return int(v.Uint()) //nolint:gosec // API values fit in int
	case protoreflect.FloatKind:
		f, _ := strconv.ParseFloat(strconv.FormatFloat(v.Float(), 'g', -1, 32), 64)
		return f
	case protoreflect.DoubleKind:
		return v.Float()
	case protoreflect.StringKind:
		return v.String()
	case protoreflect.EnumKind:
		if v.Enum() == 0 {
			return ""
		}
		if ev := fd.Enum().Values().ByNumber(v.Enum()); ev != nil {
			return string(ev.Name())
		}
		return strconv.Itoa(int(v.Enum()))
	case protoreflect.BytesKind, protoreflect.MessageKind, protoreflect.GroupKind:
		return nil
	}
	return nil
}

// tfToMessage sets fields of m from Terraform data. Computed-only attributes are skipped (they are
// output-only in the API), as are attributes without a matching proto field.
func tfToMessage(data map[string]any, m protoreflect.Message, s map[string]*schema.Schema) error {
	fields := m.Descriptor().Fields()
	for name, sch := range s {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil || (sch.Computed && !sch.Optional && !sch.Required) {
			continue
		}
		v, ok := data[name]
		if !ok || v == nil {
			continue
		}
		if err := setField(m, fd, sch, v); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func setField(m protoreflect.Message, fd protoreflect.FieldDescriptor, sch *schema.Schema, v any) error {
	switch {
	case fd.IsMap():
		mv, _ := v.(map[string]any)
		if len(mv) == 0 {
			return nil
		}
		pm := m.Mutable(fd).Map()
		for k, val := range mv {
			s, _ := val.(string)
			pm.Set(protoreflect.ValueOfString(k).MapKey(), protoreflect.ValueOfString(s))
		}
	case fd.IsList():
		items := toList(v)
		if len(items) == 0 {
			return nil
		}
		pl := m.Mutable(fd).List()
		for i, item := range items {
			pv, err := tfToListValue(pl, fd, sch.Elem, item)
			if err != nil {
				return fmt.Errorf("[%d]: %w", i, err)
			}
			pl.Append(pv)
		}
	case fd.Kind() == protoreflect.MessageKind:
		if fd.Message().FullName() == timestampFullName {
			return nil
		}
		items := toList(v)
		if len(items) == 0 {
			return nil
		}
		// An empty block (items[0] == nil) still sets the message, which matters for oneof members
		// such as `network_mesh {}`.
		obj, _ := items[0].(map[string]any)
		return tfToMessage(obj, m.Mutable(fd).Message(), resourceSchema(sch))
	default:
		pv, err := tfToScalar(fd, v)
		if err != nil {
			return err
		}
		m.Set(fd, pv)
	}
	return nil
}

func tfToListValue(
	pl protoreflect.List, fd protoreflect.FieldDescriptor, elem any, item any,
) (protoreflect.Value, error) {
	if fd.Kind() != protoreflect.MessageKind {
		return tfToScalar(fd, item)
	}
	nv := pl.NewElement()
	if r, ok := elem.(*schema.Resource); ok {
		obj, _ := item.(map[string]any)
		return nv, tfToMessage(obj, nv.Message(), r.Schema)
	}
	inner := nv.Message().Descriptor().Fields().Get(0)
	sv, err := tfToScalar(inner, item)
	if err != nil {
		return nv, err
	}
	nv.Message().Set(inner, sv)
	return nv, nil
}

func tfToScalar(fd protoreflect.FieldDescriptor, v any) (protoreflect.Value, error) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		b, _ := v.(bool)
		return protoreflect.ValueOfBool(b), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(toInt(v))), nil //nolint:gosec // validated by schema
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(int64(toInt(v))), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		n := toInt(v)
		if n < 0 {
			return protoreflect.Value{}, fmt.Errorf("must not be negative, got %d", n)
		}
		return protoreflect.ValueOfUint32(uint32(n)), nil //nolint:gosec // checked above
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		n := toInt(v)
		if n < 0 {
			return protoreflect.Value{}, fmt.Errorf("must not be negative, got %d", n)
		}
		return protoreflect.ValueOfUint64(uint64(n)), nil
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(toFloat(v))), nil
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(toFloat(v)), nil
	case protoreflect.StringKind:
		s, _ := v.(string)
		return protoreflect.ValueOfString(s), nil
	case protoreflect.EnumKind:
		s, _ := v.(string)
		if s == "" {
			return protoreflect.ValueOfEnum(0), nil
		}
		ev := fd.Enum().Values().ByName(protoreflect.Name(s))
		if ev == nil {
			return protoreflect.Value{}, fmt.Errorf("invalid value %q for %s", s, fd.Enum().Name())
		}
		return protoreflect.ValueOfEnum(ev.Number()), nil
	case protoreflect.BytesKind, protoreflect.MessageKind, protoreflect.GroupKind:
	}
	return protoreflect.Value{}, fmt.Errorf("unsupported proto kind %v", fd.Kind())
}

func resourceSchema(s *schema.Schema) map[string]*schema.Schema {
	if r, ok := s.Elem.(*schema.Resource); ok {
		return r.Schema
	}
	return nil
}

func toList(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case *schema.Set:
		return t.List()
	case []string:
		res := make([]any, len(t))
		for i, s := range t {
			res[i] = s
		}
		return res
	case []map[string]any:
		res := make([]any, len(t))
		for i, m := range t {
			res[i] = m
		}
		return res
	}
	return nil
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n + 0.5*sign(n))
	}
	return 0
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}
