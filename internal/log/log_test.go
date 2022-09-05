package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalLoggerVerbosity(t *testing.T) {
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
