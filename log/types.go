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
	unknown                   FieldComp = 0
	TimestampFieldName        FieldComp = 1
	LevelFieldName            FieldComp = 2
	CallerFieldName           FieldComp = 3
	MessageFieldName          FieldComp = 4
	MessageAttributeFieldName FieldComp = 5
	HandlerAttributeFieldName FieldComp = 6
)

// LevelFunc is a func that returns current lvl.
type LevelFunc func() slog.Level

// Formatter formats the input into a string.
type Formatter[T any] func(T) string
