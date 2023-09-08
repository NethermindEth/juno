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

type FieldComp string

const (
	TimestampFieldName        FieldComp = "time"
	LevelFieldName            FieldComp = "level"
	CallerFieldName           FieldComp = "caller"
	MessageFieldName          FieldComp = "message"
	MessageAttributeFieldName FieldComp = "message-attribute"
	HandlerAttributeFieldName FieldComp = "handler-attribute"
)

// LevelFunc is a func that returns current lvl.
type LevelFunc func() slog.Level

// Formatter formats the input into a string.
// If empty string is returned the message value will be ommited.
type Formatter[T any] func(T) string
