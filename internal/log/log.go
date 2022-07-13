// Package log provides a logger.
package log

import (
	"log"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a "sugared" variant of the zap logger.
var Logger *zap.SugaredLogger

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		// notest
		log.Fatalln("failed to initialise logger")
	}
	Logger = logger.Sugar()
}

// ReplaceGlobalLogger replace the logger and inject it globally
func ReplaceGlobalLogger(enableJsonOutput bool, verbosityLevel string) error {
	config := zap.NewProductionConfig()

	// Timestamp format (ISO8601) and time zone (UTC)
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoder(func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format("2006-01-02T15:04:05Z0700"))
	})

	// Output encoding
	config.Encoding = "console"
	if enableJsonOutput {
		config.Encoding = "json"
	}

	// Log level
	logLevel, err := zapcore.ParseLevel(verbosityLevel)
	if err != nil {
		// notest
		return err
	}
	config.Level.SetLevel(logLevel)

	logger, err := config.Build()
	if err != nil {
		// notest
		return err
	}

	Logger = logger.Sugar()
	return nil
}
