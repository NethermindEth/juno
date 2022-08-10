package log

import "testing"

func TestReplaceGlobalLogger(t *testing.T) {
	type args struct {
		enableJsonOutput   bool
		verbosityLevel     string
		enableColorEncoder bool
	}
	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "replace logger with good configuration (console encoding) should not return error",
			args: args{
				enableJsonOutput:   false,
				verbosityLevel:     "debug",
				enableColorEncoder: false,
			},
			err: nil,
		},
		{
			name: "replace logger with good configuration (json encoding) should not return error",
			args: args{
				enableJsonOutput:   true,
				verbosityLevel:     "debug",
				enableColorEncoder: true,
			},
			err: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ReplaceGlobalLogger(test.args.enableJsonOutput, test.args.verbosityLevel, test.args.enableColorEncoder); err != nil {
				t.Errorf("ReplaceGlobalLogger() error = %v", err)
			}
		})
	}
}
