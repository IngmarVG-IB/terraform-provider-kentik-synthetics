package synthetics

import "google.golang.org/protobuf/proto"

func cloneMessage[T proto.Message](m T) (T, bool) {
	c, ok := proto.Clone(m).(T)
	return c, ok
}

func protoEqual(a, b proto.Message) bool { return proto.Equal(a, b) }
