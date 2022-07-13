package log

import (
	"errors"
	"testing"
)

func TestReplaceGlobalLogger(t *testing.T) {
	type args struct {
		enableJsonOutput bool
		verbosityLevel   string
	}
	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "replace logger with good configuration (console encoding) should not return error",
			args: args{
				enableJsonOutput: false,
				verbosityLevel:   "debug",
			},
			err: nil,
		},
		{
			name: "replace logger with good configuration (json encoding) should not return error",
			args: args{
				enableJsonOutput: true,
				verbosityLevel:   "debug",
			},
			err: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ReplaceGlobalLogger(test.args.enableJsonOutput, test.args.verbosityLevel); err != nil {
				if !errors.Is(err, test.err) {
					t.Errorf("ReplaceGlobalLogger() error = %v, wantErr %v", err, test.err.Error())
				}
			}
		})
	}
}
