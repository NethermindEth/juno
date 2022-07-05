package log

import (
	"fmt"
	"reflect"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
			name: "replace logger with wrong verbosity level should return error",
			args: args{
				enableJsonOutput: false,
				verbosityLevel:   "ddd",
			},
			err: fmt.Errorf("parsing logger verbosity level failed unrecognized level: \"ddd\""),
		},
		{
			name: "replace logger with good configuration should not return error",
			args: args{
				enableJsonOutput: false,
				verbosityLevel:   "debug",
			},
			err: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ReplaceGlobalLogger(tt.args.enableJsonOutput, tt.args.verbosityLevel, false); !reflect.DeepEqual(err, tt.err) {
				t.Errorf("ReplaceGlobalLogger() error = %v, wantErr %v", err, tt.err.Error())
			}
		})
	}
}

func Test_getEncoder(t *testing.T) {
	type args struct {
		enableJsonOutput bool
	}
	tests := []struct {
		name    string
		args    args
		encoder zapcore.Encoder
	}{
		{
			name: "enable json output should return a json encoder",
			args: args{
				true,
			},
			encoder: zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		},
		{
			name: "not enabling json output should return a console encoder",
			args: args{
				false,
			},
			encoder: zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEncoder(tt.args.enableJsonOutput, false)
			actual := typeOfObject(got)
			expected := typeOfObject(tt.encoder)
			if actual != expected {
				t.Errorf("getEncoder() = %v, encoder %v", actual, expected)
			}
		})
	}
}

func Test_getEnvironmentEncoder(t *testing.T) {
	type args struct {
		isProductionEnvironment bool
	}
	tests := []struct {
		name          string
		args          args
		encoderConfig zapcore.EncoderConfig
	}{
		{
			name: "if prod environment, should return prod environment config",
			args: args{
				true,
			},
			encoderConfig: zap.NewProductionEncoderConfig(),
		},
		{
			name: "not enabling json output should return a console encoder",
			args: args{
				false,
			},
			encoderConfig: zap.NewProductionEncoderConfig(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEnvironmentEncoder(tt.args.isProductionEnvironment)
			actual := typeOfObject(got)
			expected := typeOfObject(tt.encoderConfig)
			if actual != expected {
				t.Errorf("getEncoder() = %v, encoder %v", actual, expected)
			}
		})
	}
}

func typeOfObject(x interface{}) string {
	return fmt.Sprintf("%T", x)
}
