package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerbosity(t *testing.T) {
	tests := []struct {
		verbosity string
		err       bool
	}{
		{verbosity: "info", err: false},
		{verbosity: "debug", err: false},
		{verbosity: "warn", err: true},
		{verbosity: "error", err: false},
		{verbosity: "something", err: true},

		{verbosity: "INFO", err: false},
		{verbosity: "DEBUG", err: false},
		{verbosity: "WARN", err: true},
		{verbosity: "ERROR", err: false},
		{verbosity: "FATAL", err: true},
		{verbosity: "SOMETHING", err: true},
	}
	for _, test := range tests {
		t.Run(test.verbosity, func(t *testing.T) {
			_, err := NewProductionLogger(test.verbosity)
			if test.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLog ensures all functions execute successfully on all provided loggers.
func TestLog(t *testing.T) {
	productionLogger, err := NewProductionLogger("info")
	if err != nil {
		t.Fatalf("unexpected error creating production logger: %s", err)
	}
	tests := []struct {
		name   string
		logger *Log
	}{
		{
			"NopLogger",
			NewNopLogger(),
		},
		{
			"ProductionLogger",
			productionLogger,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			test.logger.Debug("test msg")
			test.logger.Debugw("test msg", "key", "value")
			test.logger.Info("test msg")
			test.logger.Infow("test msg", "key", "value")
			test.logger.Error("test msg")
			test.logger.Errorw("test msg", "key", "value")
			test.logger.Named("TEST")
		})
	}
}
