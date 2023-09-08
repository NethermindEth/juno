package log

import (
	"log/slog"
	"os"
	"testing"
)

func TestLogger(t *testing.T) {
	// TODO write actual test
	handler, err := NewConsoleHandler(
		HandlerConfig{Lvl: func() slog.Level { return -100 }},
		Formatters{},
		os.Stdout,
	)
	if err != nil {
		t.Fatal(err)
	}

	logger := slog.New(handler)
	logger.Warn("foo", "bar", "baz", "bar", "baz")
	// TODO wrap slog.logger so that writing non-string keys is possible
	logger.Warn("1", "2", 3, "4", 5)
}
