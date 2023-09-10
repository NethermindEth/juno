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
	currentAttribute hintAttribute
	nextField        FieldComp
	prevField        FieldComp
	depth            int
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
	// writeComponent writes component to buffer. Returns true if wrote. Ignores empty messages
	writeComponent(buf *bytes.Buffer, field FieldComp, value any, hint hintWrite) bool
	// after is called after all components
	after(buf *bytes.Buffer)
}

func consoleDefaultComponents() []FieldComp {
	return []FieldComp{
		TimestampFieldName,
		LevelFieldName,
		CallerFieldName,
		MessageFieldName,
		HandlerAttributeFieldName,
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
	splitter string
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
	return &consoleWriter{Formatters: defaultFormatters, splitter: ", "}
}

func (c *consoleWriter) pre(buf *bytes.Buffer)   {}
func (c *consoleWriter) after(buf *bytes.Buffer) { buf.WriteByte('\n') }

func (c *consoleWriter) writeComponent(buf *bytes.Buffer, field FieldComp, value any, hint hintWrite) bool {
	var (
		prev = hint.prevField
		cur  = field
		next = hint.nextField
		msg  string
	)

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

	if len(msg) == 0 {
		return false
	}

	if buf.Len() > 0 && !cur.Eq(prev) {
		if !cur.WeakEq(prev) {
			buf.WriteByte('\t')
		} else {
			buf.WriteString(c.splitter)
		}
	}

	// add closure for first attribute
	if hint.currentAttribute.isValid() && hint.currentAttribute.isStarting() && !isAttribute(prev) {
		buf.WriteByte('{')
	}

	buf.WriteString(msg)

	// either close or seperate
	if hint.currentAttribute.isValid() {
		if !hint.currentAttribute.isEnding() {
			buf.WriteString(c.splitter)
		} else if !isAttribute(next) {
			buf.WriteByte('}')
		}
	}

	return true
}

func isAttribute(field FieldComp) bool {
	return field == HandlerAttributeFieldName || field == MessageAttributeFieldName
}
