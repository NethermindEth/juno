package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/NethermindEth/juno/log/colors"
	"github.com/stretchr/testify/assert"
)

func TestConsoleLogger(t *testing.T) {
	t.Run("text formatting", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 0))
		handler, err := NewConsoleHandler(
			HandlerConfig{Lvl: func() slog.Level { return -100 }},
			Formatters{
				TimeFormatter: func(_ time.Time) string {
					// replace timestamp
					t, err := time.Parse("15:04:05.000 02/01/2006 -07:00", "16:43:58.120 08/09/2023 +02:00")
					if err != nil {
						panic(err)
					}
					return t.Local().Format("15:04:05.000 02/01/2006 -07:00")
				},
			},
			buf,
		)
		if err != nil {
			t.Fatal(err)
		}

		colors.DisableColor()
		logger := slog.New(handler)
		expected := `16:43:58.120 08/09/2023 +02:00	WARN	log/handler_test.go:40	Sanity checks failed	{"number": 1234, "hash": "0x123"}`
		expected = fmt.Sprintln(expected) // add new line
		logger.Warn("Sanity checks failed", "number", 1234, "hash", "0x123")
		assert.Equal(t, expected, buf.String())
	})

	t.Run("with", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 0))
		handler, err := NewConsoleHandler(
			HandlerConfig{Lvl: func() slog.Level { return -100 }},
			Formatters{
				TimeFormatter: func(_ time.Time) string {
					// replace timestamp
					t, err := time.Parse("15:04:05.000 02/01/2006 -07:00", "16:43:58.120 08/09/2023 +02:00")
					if err != nil {
						panic(err)
					}
					return t.Local().Format("15:04:05.000 02/01/2006 -07:00")
				},
				SourceFormatter: func(source slog.Source) string {
					splitFile := strings.Split(source.File, "juno/")
					if len(splitFile) > 1 {
						return splitFile[1]
					}
					return source.File
				},
			},
			buf,
		)
		if err != nil {
			t.Fatal(err)
		}

		colors.DisableColor()

		logger := slog.New(handler).With("foo", "bar")
		logger.Info("baz", "1", 2)
		expected := `16:43:58.120 08/09/2023 +02:00	INFO	log/handler_test.go	baz	{"foo": "bar", "1": 2}`
		expected = fmt.Sprintln(expected) // add new line
		assert.Equal(t, expected, buf.String())
		buf.Reset()

		logger = logger.With("test", 2)
		logger.Info("baz", "1", 2)
		expected = `16:43:58.120 08/09/2023 +02:00	INFO	log/handler_test.go	baz	{"foo": "bar", "test": 2, "1": 2}`
		expected = fmt.Sprintln(expected) // add new line
		assert.Equal(t, expected, buf.String())
		buf.Reset()

		logger = logger.With("key", "value")
		logger.Info("baz", "1", 2)
		expected = `16:43:58.120 08/09/2023 +02:00	INFO	log/handler_test.go	baz	{"foo": "bar", "test": 2, "key": "value", "1": 2}`
		expected = fmt.Sprintln(expected) // add new line
		assert.Equal(t, expected, buf.String())
		buf.Reset()
	})
}
