package log

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"runtime"
	"sync"
)

var (
	outputBuffer = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 64))
		},
	}
)

func getBuffer() *bytes.Buffer {
	return outputBuffer.Get().(*bytes.Buffer)
}

func putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	outputBuffer.Put(buf)
}

type slogHandler struct {
	lvl   LevelFunc
	comps fieldComps
	attrs []slog.Attr

	bufWriter writer

	out io.Writer
}

// NewConsoleHandler returns handler for console output
func NewConsoleHandler(cfg HandlerConfig, formats Formatters, output io.Writer) (*slogHandler, error) {
	var (
		comps fieldComps
		err   error
	)
	if cfg.Components != nil {
		comps, err = validateComps(cfg.Components)
	} else {
		comps, err = validateComps(consoleDefaultComponents())
	}
	if err != nil {
		return nil, err
	}

	return &slogHandler{
		comps:     comps,
		lvl:       cfg.Lvl,
		bufWriter: newConsoleWriter(formats),
		out:       output,
	}, nil
}

func (h *slogHandler) copy() *slogHandler {
	attrs := make([]slog.Attr, len(h.attrs))
	copy(attrs, h.attrs)
	return &slogHandler{
		lvl:       h.lvl,
		comps:     h.comps,
		attrs:     attrs,
		bufWriter: h.bufWriter,
		out:       h.out,
	}
}

func (h *slogHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.lvl()
}

func (h *slogHandler) Handle(_ context.Context, record slog.Record) error {
	buf := getBuffer()
	defer putBuffer(buf)

	h.bufWriter.pre(buf)

	// initialzie some values
	var (
		prevField FieldComp = none
		compSlots           = len(h.comps) - 1
	)
	// write components in order
	for x, comp := range h.comps {
		var (
			// writer.writeComponent result
			wrote bool
			// rebuild hint
			hint = hintWrite{
				prevField: prevField,
				nextField: none,
				depth:     x,
			}
		)
		// hint next attribute if any
		if x+1 <= compSlots {
			hint.nextField = h.comps[x+1]
		}
		switch comp {
		case TimestampFieldName:
			wrote = h.bufWriter.writeComponent(buf, comp, record.Time, hint)
		case LevelFieldName:
			wrote = h.bufWriter.writeComponent(buf, comp, record.Level, hint)
		case CallerFieldName:
			fs := runtime.CallersFrames([]uintptr{record.PC})
			f, _ := fs.Next()
			if f.File != "" {
				src := slog.Source{
					Function: f.Function,
					File:     f.File,
					Line:     f.Line,
				}
				wrote = h.bufWriter.writeComponent(buf, comp, src, hint)
			}
		case HandlerAttributeFieldName:
			size := len(h.attrs)
			backup := hint.nextField
			hint.nextField = HandlerAttributeFieldName
			for i, attr := range h.attrs {
				hint.currentAttribute = hintAttribute{i, size}
				// restore actual next field if its last local elem
				if hint.currentAttribute.isEnding() {
					hint.nextField = backup
				}
				if h.bufWriter.writeComponent(
					buf,
					HandlerAttributeFieldName,
					attr,
					hint,
				) {
					wrote = true
				}
				hint.prevField = comp // resolve inner loop field
			}
			hint.currentAttribute = hintAttribute{} // reset attribute
			hint.nextField = backup
		case MessageFieldName:
			wrote = h.bufWriter.writeComponent(buf, MessageFieldName, record.Message, hint)
		case MessageAttributeFieldName:
			var index int
			backup := hint.nextField
			hint.currentAttribute.size = record.NumAttrs()
			hint.nextField = comp // resolve inner next attribute
			record.Attrs(func(attr slog.Attr) bool {
				hint.currentAttribute.index = index
				// restore actual next field if its last local elem
				if hint.currentAttribute.isEnding() {
					hint.nextField = backup
				}
				if h.bufWriter.writeComponent(
					buf,
					MessageAttributeFieldName,
					attr,
					hint,
				) {
					wrote = true
				}
				index++
				hint.prevField = comp // resolve inner loop field
				return true
			})
			hint.currentAttribute = hintAttribute{} // reset attribute
			hint.nextField = backup
		default:
			continue
		}
		if !wrote {
			continue
		}
		prevField = comp
	}

	h.bufWriter.after(buf)
	_, err := buf.WriteTo(h.out)
	return err
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handler := h.copy()
	handler.attrs = append(handler.attrs, attrs...)
	return handler
}

func (l *slogHandler) WithGroup(name string) slog.Handler {
	// TODO implement
	panic("implement me")
	return nil
}

func validateComps(comps fieldComps) (fieldComps, error) {
	comps = comps.Validated()
	if len(comps) == 0 {
		return nil, errors.New("invalid components provided")
	}
	return comps, nil
}

// voidHandler is a hadnler that does no-op on every method.
type voidHandler struct{}

func NewVoidHandler() *voidHandler { return &voidHandler{} }

func (h *voidHandler) Enabled(_ context.Context, lvl slog.Level) bool { return false }

func (h *voidHandler) Handle(_ context.Context, record slog.Record) error { return nil }

func (h *voidHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }

func (h *voidHandler) WithGroup(name string) slog.Handler { return h }
