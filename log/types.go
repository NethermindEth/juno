package log

import "log/slog"

type fieldComps []FieldComp

// Validated returns validated fieldComps
func (comp fieldComps) Validated() fieldComps {
	// remove duplicates
	allKeys := make(map[FieldComp]struct{})
	list := []FieldComp{}
	for _, item := range comp {
		if _, value := allKeys[item]; !value {
			allKeys[item] = struct{}{}
			list = append(list, item)
		}
	}
	return list
}

type FieldComp uint8

const (
	// internal types
	unknown   FieldComp = 0
	none      FieldComp = 1
	attribute FieldComp = 2

	ReservedBuffer = 10 // up to this place everything is reserved

	TimestampFieldName        FieldComp = ReservedBuffer + 1
	LevelFieldName            FieldComp = TimestampFieldName + 1
	CallerFieldName           FieldComp = LevelFieldName + 1
	MessageFieldName          FieldComp = CallerFieldName + 1
	MessageAttributeFieldName FieldComp = MessageFieldName + 1
	HandlerAttributeFieldName FieldComp = MessageAttributeFieldName + 1
)

func (field FieldComp) String() string {
	switch field {
	case unknown:
		return "unknown"
	case none:
		return "none"
	case attribute:
		return "attribute"
	case ReservedBuffer:
		return "ReservedBuffer"
	case TimestampFieldName:
		return "TimestampFieldName"
	case LevelFieldName:
		return "LevelFieldName"
	case CallerFieldName:
		return "CallerFieldName"
	case MessageFieldName:
		return "MessageFieldName"
	case MessageAttributeFieldName:
		return "MessageAttributeFieldName"
	case HandlerAttributeFieldName:
		return "HandlerAttributeFieldName"
	default:
		return "not defined"
	}
}

// WeakEq returns whether two fields are weakly equal.
func (field FieldComp) WeakEq(f FieldComp) bool {
	return isAttribute(field) && isAttribute(f)
}

// WeakEq returns whether two fields are equal.
func (field FieldComp) Eq(f FieldComp) bool {
	return field == f
}

// LevelFunc is a func that returns current lvl.
type LevelFunc func() slog.Level

// Formatter formats the input into a string.
type Formatter[T any] func(T) string
