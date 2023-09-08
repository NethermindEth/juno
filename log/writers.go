package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/NethermindEth/juno/log/colors"
)

type hintWrite struct {
	// attributes hints
	attribute hintAttribute
	prevField FieldComp
	depth     int
}

type hintAttribute struct {
	index int
	size  int
}

func (hint hintAttribute) isValid() bool {
	return hint.size != 0
}

func (hint hintAttribute) isEnding() bool {
	return hint.index+1 == hint.size
}

func (hint hintAttribute) isStarting() bool {
	return hint.index == 0
}

type writer interface {
	// pre is called before components
	pre(buf *bytes.Buffer)
	// writeComponent writes component to buffer
	writeComponent(buf *bytes.Buffer, field FieldComp, value any, hint hintWrite)
	// after is called after all components
	after(buf *bytes.Buffer)
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
			if t.IsZero() {
				return ""
			}
			val := t.Round(0)
			return val.Local().Format("15:04:05.000 02/01/2006 -07:00")
		},
		LevelFormatter: func(lvl slog.Level) string {
			return colors.Green(lvl.String())
		},
		SourceFormatter: func(source slog.Source) string {
			splitFile := strings.Split(source.File, "juno/")
			if len(splitFile) > 1 {
				return fmt.Sprintf("%v:%v", splitFile[1], source.Line)
			}
			return fmt.Sprintf("%s:%d", source.File, source.Line)
		},
		MessageFormatter: func(msg string) string {
			return msg
		},
		AttrFormatter: func(attr slog.Attr) string {
			k, _ := json.Marshal(attr.Key)
			v, _ := json.Marshal(attr.Value.Any())
			return fmt.Sprintf("%s: %s", k, v)
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

func (c *consoleWriter) pre(buf *bytes.Buffer)   {}
func (c *consoleWriter) after(buf *bytes.Buffer) { buf.WriteByte('\n') }

func (c *consoleWriter) writeComponent(buf *bytes.Buffer, field FieldComp, value any, hint hintWrite) {
	prev := hint.prevField
	cur := field
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

	// seperate different contexts
	if buf.Len() > 0 && prev != cur {
		buf.WriteByte('\t')
	}

	// add closure for first attribute
	if hint.attribute.isValid() && hint.attribute.isStarting() {
		buf.WriteByte('{')
	}

	buf.WriteString(msg)

	// either close or seperate
	if hint.attribute.isValid() {
		if !hint.attribute.isEnding() {
			buf.WriteString(", ")
		} else {
			buf.WriteByte('}')
		}
	}
}
