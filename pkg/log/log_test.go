package log

import (
	"fmt"
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

func ExampleNewProductionLogger() {
	logger, err := NewProductionLogger("info")
	if err != nil {
		panic(fmt.Sprintf("unexpected error creating production logger: %s", err))
	}

	logger.Debug("test msg")
	logger.Debugw("test msg", "key", "value")
	logger.Info("test msg")
	logger.Infow("test msg", "key", "value")
	logger.Error("test msg")
	logger.Errorw("test msg", "key", "value")
	logger.Named("TEST")
}

func ExampleNewNopLogger() {
	logger := NewNopLogger()

	logger.Debug("test msg")
	logger.Debugw("test msg", "key", "value")
	logger.Info("test msg")
	logger.Infow("test msg", "key", "value")
	logger.Error("test msg")
	logger.Errorw("test msg", "key", "value")
	logger.Named("TEST")
}
