// Package log provides a logger.
package log

import (
	"fmt"
	"time"
	"regexp"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Debug(...any)
	Debugw(string, ...any)

	Info(...any)
	Infow(string, ...any)

	Error(...any)
	Errorw(string, ...any)

	Named(string) Logger
}

type Log struct {
	zapLogger *zap.SugaredLogger
}

// *Log implements Logger
var _ Logger = &Log{}

func NewProductionLogger(verbosity string) (*Log, error) {
	re := regexp.MustCompile("(?i)debug|info|error")
	if !re.Match([]byte(verbosity)) {
		return nil, fmt.Errorf("cannot parse verbosity: %s", verbosity)
	}

	logConfig := zap.NewProductionConfig()
	logConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logConfig.Encoding = "console"
	// Timestamp format (ANSIC) and time zone (local)
	logConfig.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Local().Format(time.ANSIC))
	}
	logLevel, err := zapcore.ParseLevel(verbosity)
	if err != nil {
		return nil, err
	}
	logConfig.Level.SetLevel(logLevel)
	logger, err := logConfig.Build()
	if err != nil {
		return nil, err
	}
	return NewLogger(logger.Sugar()), nil
}

func NewLogger(zapLogger *zap.SugaredLogger) *Log {
	zapLogger.Sync()
	return &Log{
		zapLogger: zapLogger,
	}
}

func (l *Log) Debug(args ...any) {
	l.zapLogger.Debug(args...)
}

func (l *Log) Debugw(msg string, args ...any) {
	l.zapLogger.Debugw(msg, args...)
}

func (l *Log) Info(args ...any) {
	l.zapLogger.Info(args...)
}

func (l *Log) Infow(msg string, args ...any) {
	l.zapLogger.Infow(msg, args...)
}

func (l *Log) Error(args ...any) {
	l.zapLogger.Error(args...)
}

func (l *Log) Errorw(msg string, args ...any) {
	l.zapLogger.Errorw(msg, args...)
}

func (l *Log) Named(name string) Logger {
	return NewLogger(l.zapLogger.Named(name))
}
