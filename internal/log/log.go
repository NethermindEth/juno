// Package log provides a logger.
package log

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a "sugared" variant of the zap logger.
var Logger *zap.SugaredLogger

func init() {
	SetGlobalLogger("info")
}

// SetGlobalLogger replace the logger and inject it globally
func SetGlobalLogger(verbosityLevel string) error {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.Encoding = "console"

	// Timestamp format (ANSIC) and time zone (local)
	config.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Local().Format(time.ANSIC))
	}

	logLevel, err := zapcore.ParseLevel(verbosityLevel)
	if err != nil {
		return err
	}

	config.Level.SetLevel(logLevel)

	logger, err := config.Build()
	if err != nil {
		return err
	}

	Logger = logger.Sugar()
	return nil
}
