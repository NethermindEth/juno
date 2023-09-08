package log

import (
	"log/slog"
	"time"
)

type HandlerConfig struct {
	Lvl LevelFunc // returns current lvl

	// The handler will output all matching components in order.
	Components []FieldComp
}

// Formatters includes all avaiable formatting methods
type Formatters struct {
	TimeFormatter    Formatter[time.Time]
	LevelFormatter   Formatter[slog.Level]
	SourceFormatter  Formatter[slog.Source]
	MessageFormatter Formatter[string]
	AttrFormatter    Formatter[slog.Attr]
}
