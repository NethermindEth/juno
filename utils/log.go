package utils

import (
	"encoding"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var ErrUnknownLogLevel = fmt.Errorf(
	"unknown log level (known: %s, %s, %s, %s, %s)",
	TRACE, DEBUG, INFO, WARN, ERROR,
)

type LogLevel int

// The following are necessary for Cobra and Viper, respectively, to unmarshal log level
// CLI/config parameters properly.
var (
	_ pflag.Value              = (*LogLevel)(nil)
	_ encoding.TextUnmarshaler = (*LogLevel)(nil)
)

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	TRACE
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "debug"
	case INFO:
		return "info"
	case WARN:
		return "warn"
	case ERROR:
		return "error"
	case TRACE:
		return "trace"
	default:
		// Should not happen.
		panic(ErrUnknownLogLevel)
	}
}

func (l LogLevel) MarshalYAML() (interface{}, error) {
	return l.String(), nil
}

func (l *LogLevel) Set(s string) error {
	switch s {
	case "DEBUG", "debug":
		*l = DEBUG
	case "INFO", "info":
		*l = INFO
	case "WARN", "warn":
		*l = WARN
	case "ERROR", "error":
		*l = ERROR
	case "TRACE", "trace":
		*l = TRACE
	default:
		return ErrUnknownLogLevel
	}
	return nil
}

func (l *LogLevel) Type() string {
	return "LogLevel"
}

func (l *LogLevel) MarshalText() ([]byte, error) {
	return []byte(l.String()), nil
}

func (l *LogLevel) UnmarshalText(text []byte) error {
	return l.Set(string(text))
}

type Logger interface {
	SimpleLogger
	pebble.Logger
}

type SimpleLogger interface {
	Debugw(msg string, keysAndValues ...any)
	Infow(msg string, keysAndValues ...any)
	Warnw(msg string, keysAndValues ...any)
	Errorw(msg string, keysAndValues ...any)
	Tracew(msg string, keysAndValues ...any)
}

type ZapLogger struct {
	*zap.SugaredLogger
}

const (
	traceLevel = zapcore.Level(-2)
)

func (l *ZapLogger) IsTraceEnabled() bool {
	return l.Desugar().Core().Enabled(traceLevel)
}

func (l *ZapLogger) Tracew(msg string, keysAndValues ...interface{}) {
	if l.IsTraceEnabled() {
		l.Debugw("[TRACE] "+msg, keysAndValues...) // Hack: we use Debug for traces
	}
}

var _ Logger = (*ZapLogger)(nil)

func NewNopZapLogger() *ZapLogger {
	return &ZapLogger{zap.NewNop().Sugar()}
}

func NewZapLogger(logLevel LogLevel, colour bool) (*ZapLogger, error) {
	config := zap.NewProductionConfig()
	config.Sampling = nil
	config.Encoding = "console"
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	if !colour {
		config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	config.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Local().Format("15:04:05.000 02/01/2006 -07:00"))
	}

	var level zapcore.Level
	var err error
	levelStr := logLevel.String()
	if logLevel == TRACE {
		level = traceLevel
	} else {
		level, err = zapcore.ParseLevel(levelStr)
		if err != nil {
			return nil, err
		}
	}
	config.Level.SetLevel(level)
	log, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &ZapLogger{log.Sugar()}, nil
}

func (l *ZapLogger) Warningf(msg string, args ...any) {
	l.Warnf(msg, args)
}
