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

	// write components in order
	for _, comp := range h.comps {
		switch comp {
		case TimestampFieldName:
			h.bufWriter.writeComponent(buf, comp, record.Time)
		case LevelFieldName:
			h.bufWriter.writeComponent(buf, comp, record.Level)
		case CallerFieldName:
			fs := runtime.CallersFrames([]uintptr{record.PC})
			f, _ := fs.Next()
			if f.File != "" {
				src := slog.Source{
					Function: f.Function,
					File:     f.File,
					Line:     f.Line,
				}
				h.bufWriter.writeComponent(buf, comp, src)
			}
		case HandlerAttributeFieldName:
			for _, attr := range h.attrs {
				h.bufWriter.writeComponent(buf, HandlerAttributeFieldName, attr)
			}
		case MessageFieldName:
			h.bufWriter.writeComponent(buf, MessageFieldName, record.Message)
		case MessageAttributeFieldName:
			record.Attrs(func(attr slog.Attr) bool {
				h.bufWriter.writeComponent(buf, MessageAttributeFieldName, attr)
				return true
			})
		}
	}

	err := buf.WriteByte('\n')
	if err != nil {
		return err
	}

	_, err = buf.WriteTo(h.out)
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
