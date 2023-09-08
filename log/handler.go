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

func (h *slogHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.lvl()
}

func (h *slogHandler) Handle(_ context.Context, record slog.Record) error {
	buf := getBuffer()
	defer putBuffer(buf)

	h.bufWriter.pre(buf)

	// write components in order
	var prevField FieldComp
	for x, comp := range h.comps {
		hint := hintWrite{
			prevField: prevField,
			depth:     x,
		}
		switch comp {
		case TimestampFieldName:
			h.bufWriter.writeComponent(buf, comp, record.Time, hint)
		case LevelFieldName:
			h.bufWriter.writeComponent(buf, comp, record.Level, hint)
		case CallerFieldName:
			fs := runtime.CallersFrames([]uintptr{record.PC})
			f, _ := fs.Next()
			if f.File != "" {
				src := slog.Source{
					Function: f.Function,
					File:     f.File,
					Line:     f.Line,
				}
				h.bufWriter.writeComponent(buf, comp, src, hint)
			}
		case HandlerAttributeFieldName:
			size := len(h.attrs)
			for i, attr := range h.attrs {
				hint.attribute = hintAttribute{i, size}
				h.bufWriter.writeComponent(
					buf,
					HandlerAttributeFieldName,
					attr,
					hint,
				)
				hint.prevField = comp // resolve inner loop field
			}
			hint.attribute = hintAttribute{} // reset attribute
		case MessageFieldName:
			h.bufWriter.writeComponent(buf, MessageFieldName, record.Message, hint)
		case MessageAttributeFieldName:
			var index int
			hint.attribute.size = record.NumAttrs()
			record.Attrs(func(attr slog.Attr) bool {
				hint.attribute.index = index
				h.bufWriter.writeComponent(
					buf,
					MessageAttributeFieldName,
					attr,
					hint,
				)
				index++
				hint.prevField = comp // resolve inner loop field
				return true
			})
			hint.attribute = hintAttribute{} // reset attribute
		default:
			continue
		}
		prevField = comp
	}

	h.bufWriter.after(buf)
	_, err := buf.WriteTo(h.out)
	return err
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// TODO implement
	panic("implement me")
	return nil
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
