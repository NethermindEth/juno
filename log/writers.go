package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"time"

	"github.com/NethermindEth/juno/log/colors"
)

type writer interface {
	writeComponent(buf *bytes.Buffer, field FieldComp, value any)
}

func consoleDefaultComponents() []FieldComp {
	return []FieldComp{
		TimestampFieldName,
		LevelFieldName,
		CallerFieldName,
		HandlerAttributeFieldName,
		MessageFieldName,
		MessageAttributeFieldName,
	}
}

func consoleDefaultFormatters() Formatters {
	// TODO match colors as in legacy
	return Formatters{
		TimeFormatter: func(t time.Time) string {
			if !t.IsZero() {
				val := t.Round(0)
				return val.Local().Format("15:04:05.000 02/01/2006 -07:00")
			}
			return ""
		},
		LevelFormatter: func(lvl slog.Level) string {
			return colors.Green(lvl.String())
		},
		SourceFormatter: func(source slog.Source) string {
			// TODO format as in legacy
			// Currently it is: "/home/dev/contrib/juno/log/handler_test.go.github.com/NethermindEth/juno/log.TestLogger.22"
			// Should be eg.: "utils/log_test.go:106"
			return fmt.Sprintf("%v.%v.%v", source.File, source.Function, source.Line)
		},
		MessageFormatter: func(msg string) string {
			return msg
		},
		AttrFormatter: func(attr slog.Attr) string {
			// TODO format the same way as in legacy mode
			// Currently it is: KEY=VALUE, should be KEY VALUE
			return attr.String()
		},
	}
}

type consoleWriter struct {
	Formatters
}

func newConsoleWriter(formatters Formatters) *consoleWriter {
	defaultFormatters := consoleDefaultFormatters()
	if formatters.TimeFormatter != nil {
		defaultFormatters.TimeFormatter = formatters.TimeFormatter
	}
	if formatters.LevelFormatter != nil {
		defaultFormatters.LevelFormatter = formatters.LevelFormatter
	}
	if formatters.SourceFormatter != nil {
		defaultFormatters.SourceFormatter = formatters.SourceFormatter
	}
	if formatters.AttrFormatter != nil {
		defaultFormatters.AttrFormatter = formatters.AttrFormatter
	}
	if formatters.MessageFormatter != nil {
		defaultFormatters.MessageFormatter = formatters.MessageFormatter
	}
	return &consoleWriter{Formatters: defaultFormatters}
}

func (c *consoleWriter) writeComponent(buf *bytes.Buffer, field FieldComp, value any) {
	var msg string
	switch field {
	case TimestampFieldName:
		msg = c.TimeFormatter(value.(time.Time))
	case LevelFieldName:
		msg = c.LevelFormatter(value.(slog.Level))
	case CallerFieldName:
		msg = c.SourceFormatter(value.(slog.Source))
	case MessageFieldName:
		msg = c.MessageFormatter(value.(string))
	case MessageAttributeFieldName, HandlerAttributeFieldName:
		msg = c.AttrFormatter(value.(slog.Attr))
	}

	if len(msg) > 0 {
		if buf.Len() > 0 {
			buf.WriteByte(' ') // Write space once
		}
		buf.WriteString(msg)
	}
}
