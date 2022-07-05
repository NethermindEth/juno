// Package log provides a logger.
package log

import (
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a "sugared" variant of the zap logger.
var Logger *zap.SugaredLogger

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		// notest
		log.Fatalln("failed to initialise application logger")
	}
	Logger = logger.Sugar()
}

// ReplaceGlobalLogger replace the logger and inject it globally
func ReplaceGlobalLogger(enableJsonOutput bool, verbosityLevel string, isProductionEnv bool) error {
	// parsing log verbosity level
	zapVerbosityLevel, err := zapcore.ParseLevel(verbosityLevel)
	if err != nil {
		return fmt.Errorf("parsing logger verbosity level failed %s", err)
	}

	// get the output format
	encoder := getEncoder(enableJsonOutput, isProductionEnv)

	// define where logs will be output
	writerSyncer := os.Stdout

	// initiate logger core config
	core := zapcore.NewCore(encoder, writerSyncer, zapVerbosityLevel)
	zapLogger := zap.New(core)

	// inject globally
	zap.ReplaceGlobals(zapLogger)

	// replace default
	Logger = zapLogger.Sugar()

	return nil
}

// getEncoder define the output format
func getEncoder(enableJsonOutput bool, isProductionEnv bool) zapcore.Encoder {
	encoderConfig := getEnvironmentEncoder(isProductionEnv)
	if enableJsonOutput {
		return zapcore.NewJSONEncoder(encoderConfig)
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

//getEnvironmentEncoder
func getEnvironmentEncoder(isProductionEnv bool) zapcore.EncoderConfig {
	var encoderConfig zapcore.EncoderConfig
	encoderConfig = zap.NewDevelopmentEncoderConfig()
	if isProductionEnv {
		encoderConfig = zap.NewProductionEncoderConfig()
	}
	return encoderConfig
}
