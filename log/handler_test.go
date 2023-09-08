package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/NethermindEth/juno/log/colors"
	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
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
	expected := `16:43:58.120 08/09/2023 +02:00	WARN	log/handler_test.go:38	Sanity checks failed	{"number": 1234, "hash": "0x123"}`
	expected = fmt.Sprintln(expected) // add new line
	logger.Warn("Sanity checks failed", "number", 1234, "hash", "0x123")
	assert.Equal(t, expected, buf.String())
}
