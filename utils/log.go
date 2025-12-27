package utils

import (
	"encoding"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var ErrUnknownLogLevel = fmt.Errorf(
	"unknown log level (known: %s, %s, %s, %s, %s)",
	TRACE, DEBUG, INFO, WARN, ERROR,
)

type LogLevel struct {
	atomicLevel zap.AtomicLevel
}

func (l LogLevel) GetAtomicLevel() zap.AtomicLevel {
	return l.atomicLevel
}

func NewLogLevel(level zapcore.Level) *LogLevel {
	return &LogLevel{atomicLevel: zap.NewAtomicLevelAt(level)}
}

// The following are necessary for Cobra and Viper, respectively, to unmarshal log level
// CLI/config parameters properly.
var (
	_ pflag.Value              = (*LogLevel)(nil)
	_ encoding.TextUnmarshaler = (*LogLevel)(nil)
)

const (
	TRACE zapcore.Level = iota - 2
	DEBUG
	INFO
	WARN
	ERROR
)

func (l LogLevel) String() string {
	switch l.Level() {
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

func (l LogLevel) Level() zapcore.Level {
	return l.atomicLevel.Level()
}

func (l LogLevel) MarshalYAML() (any, error) {
	return l.String(), nil
}

func (l *LogLevel) Set(s string) error {
	switch strings.ToUpper(s) {
	case "DEBUG":
		l.atomicLevel.SetLevel(DEBUG)
	case "INFO":
		l.atomicLevel.SetLevel(INFO)
	case "WARN":
		l.atomicLevel.SetLevel(WARN)
	case "ERROR":
		l.atomicLevel.SetLevel(ERROR)
	case "TRACE":
		l.atomicLevel.SetLevel(TRACE)
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
	pebble.Logger
	SimpleLogger
	StructuredLogger
}

// Deprecated: use StructuredLogger interface instead
type SimpleLogger interface {
	Debugw(msg string, keysAndValues ...any)
	Infow(msg string, keysAndValues ...any)
	Warnw(msg string, keysAndValues ...any)
	Errorw(msg string, keysAndValues ...any)
	Tracew(msg string, keysAndValues ...any)
}

type StructuredLogger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
}

var _ Logger = (*ZapLogger)(nil)

type ZapLogger struct {
	Structured *zap.Logger
	// Deprecated logger
	Sugared *zap.SugaredLogger
}

func (l *ZapLogger) Infof(msg string, args ...any) {
	l.Sugared.Infof(msg, args)
}

func (l *ZapLogger) Errorf(msg string, args ...any) {
	l.Sugared.Infof(msg, args)
}

func (l *ZapLogger) Fatalf(msg string, args ...any) {
	l.Sugared.Fatalf(msg, args)
}

// Deprecated: use Debug with structured fields instead
func (l *ZapLogger) Debugw(msg string, keysAndValues ...any) {
	l.Sugared.Debugw(msg, keysAndValues...)
}

// Deprecated: use Info with structured fields instead
func (l *ZapLogger) Infow(msg string, keysAndValues ...any) {
	l.Sugared.Infow(msg, keysAndValues...)
}

// Deprecated: use Warn with structured fields instead
func (l *ZapLogger) Warnw(msg string, keysAndValues ...any) {
	l.Sugared.Warnw(msg, keysAndValues...)
}

// Deprecated: use Error with structured fields instead
func (l *ZapLogger) Errorw(msg string, keysAndValues ...any) {
	l.Sugared.Errorw(msg, keysAndValues...)
}

func (l *ZapLogger) Tracew(msg string, keysAndValues ...any) {
	if l.IsTraceEnabled() {
		// l.WithOptions() clones logger every time there is a Tracew() call
		// which may be inefficient, one possible improvement is to create
		// special logger just for traces in ZapLogger with AddCallerSkip(1)
		// also check this issue https://github.com/uber-go/zap/issues/930 for updates

		// AddCallerSkip(1) is necessary to skip the caller of this function
		l.Sugared.WithOptions(zap.AddCallerSkip(1)).Logw(TRACE, msg, keysAndValues...)
	}
}

func (l *ZapLogger) Debug(msg string, fields ...zap.Field) {
	l.Structured.Debug(msg, fields...)
}

func (l *ZapLogger) Info(msg string, fields ...zap.Field) {
	l.Structured.Info(msg, fields...)
}

func (l *ZapLogger) Warn(msg string, fields ...zap.Field) {
	l.Structured.Warn(msg, fields...)
}

func (l *ZapLogger) Error(msg string, fields ...zap.Field) {
	l.Structured.Error(msg, fields...)
}

func (l *ZapLogger) IsTraceEnabled() bool {
	return l.Structured.Core().Enabled(TRACE)
}

func NewNopZapLogger() *ZapLogger {
	noop := zap.NewNop()
	return &ZapLogger{
		noop,
		noop.Sugar(),
	}
}

func NewZapLogger(logLevel *LogLevel, colour bool) (*ZapLogger, error) {
	config := zap.NewProductionConfig()
	config.Sampling = nil
	config.Encoding = "console"
	config.EncoderConfig.EncodeLevel = capitalColorLevelEncoder
	if !colour {
		config.EncoderConfig.EncodeLevel = capitalLevelEncoder
	}
	config.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Local().Format("15:04:05.000 02/01/2006 -07:00"))
	}

	config.Level = logLevel.atomicLevel
	log, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &ZapLogger{log, log.Sugar()}, nil
}

// colour represents a text colour.
//
//nolint:misspell //colour type with methods were extracted from go.uber.org/zap/internal/color
type colour uint8

// Add adds the colouring to the given string.
func (c colour) Add(s string) string {
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", uint8(c), s)
}

// capitalColorLevelEncoder adds support for TRACE log level to the default CapitalColorLevelEncoder
func capitalColorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	const cyan colour = 36
	if l == TRACE {
		enc.AppendString(cyan.Add("TRACE"))
	} else {
		zapcore.CapitalColorLevelEncoder(l, enc)
	}
}

// capitalLevelEncoder adds support for TRACE log level to the default CapitalLevelEncoder
func capitalLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	if l == TRACE {
		enc.AppendString("TRACE")
	} else {
		zapcore.CapitalLevelEncoder(l, enc)
	}
}

// HTTPLogSettings is an HTTP handler that allows changing the log level of the logger.
// It can also be used to query what's the current log level.
func HTTPLogSettings(w http.ResponseWriter, r *http.Request, log *LogLevel) {
	switch r.Method {
	case http.MethodGet:
		fmt.Fprint(w, log.String()+"\n")
	case http.MethodPut:
		levelStr := r.URL.Query().Get("level")
		if levelStr == "" {
			http.Error(w, "missing level query parameter", http.StatusBadRequest)
			return
		}

		err := log.Set(levelStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "Replaced log level with '", html.EscapeString(levelStr), "' successfully\n")
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
