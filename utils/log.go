package utils

import (
	"errors"
	"time"

	"github.com/cockroachdb/pebble"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var ErrUnknownLogLevel = errors.New("unknown log level")

type LogLevel uint8

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
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
	default:
		return ""
	}
}

func (l LogLevel) IsValid() bool {
	return l.String() != ""
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
}

type ZapLogger struct {
	*zap.SugaredLogger
}

var _ Logger = (*ZapLogger)(nil)

func NewNopZapLogger() *ZapLogger {
	return &ZapLogger{zap.NewNop().Sugar()}
}

func NewZapLogger(logLevel LogLevel) (*ZapLogger, error) {
	config := zap.NewProductionConfig()
	config.Encoding = "console"
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Local().Format("15:04:05.000 02/01/2006 -07:00"))
	}
	level, err := zapcore.ParseLevel(logLevel.String())
	if err != nil {
		return nil, err
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
