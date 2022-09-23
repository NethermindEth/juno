package log

import (
	"testing"

	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestGlobalLoggerVerbosity(t *testing.T) {
	tests := []struct {
		verbosity string
		err       bool
	}{
		{verbosity: "info", err: false},
		{verbosity: "debug", err: false},
		{verbosity: "warn", err: false},
		{verbosity: "error", err: false},
		{verbosity: "dpanic", err: false},
		{verbosity: "panic", err: false},
		{verbosity: "fatal", err: false},
		{verbosity: "something", err: true},

		{verbosity: "INFO", err: false},
		{verbosity: "DEBUG", err: false},
		{verbosity: "WARN", err: false},
		{verbosity: "ERROR", err: false},
		{verbosity: "DPANIC", err: false},
		{verbosity: "PANIC", err: false},
		{verbosity: "FATAL", err: false},
		{verbosity: "SOMETHING", err: true},
	}
	for _, test := range tests {
		t.Run(test.verbosity, func(t *testing.T) {
			err := SetGlobalLogger(test.verbosity)
			if test.err {
				assert.Check(t, is.ErrorContains(err, ""))
			} else {
				assert.Check(t, err)
			}
		})
	}
}
