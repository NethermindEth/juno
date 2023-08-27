package utils

import (
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

var ErrUnknownLogLevel = errors.New("unknown log level (known: debug, info, warn, error, fatal)")

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
	FATAL
)

const (
	timeFormat = "15:04:05.000 02/01/2006 -07:00"
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
	case FATAL:
		return "fatal"
	default:
		// Should not happen.
		panic(ErrUnknownLogLevel)
	}
}

func (l LogLevel) StringUpper() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
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
	case "FATAL", "fatal":
		*l = FATAL
	default:
		return ErrUnknownLogLevel
	}
	return nil
}

func (l *LogLevel) Type() string {
	return "LogLevel"
}

func (l *LogLevel) MarshalJSON() ([]byte, error) {
	return json.RawMessage(`"` + l.String() + `"`), nil
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
}

type ZapLogger struct {
	*zap.SugaredLogger
}

var _ Logger = (*ZapLogger)(nil)
var _ Logger = (*SlogLogger)(nil)
var _ Logger = (*noopLogger)(nil)

func (l *ZapLogger) Warningf(msg string, args ...any) {
	l.Warnf(msg, args)
}

var ctx = context.Background()

type SlogLogger struct {
	*slog.Logger
}

func NewSlogLogger(logLevel LogLevel, _ bool) (*SlogLogger, error) {
	lvl, err := adaptToSlog(logLevel)
	if err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     lvl,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			switch attr.Value.Kind() {
			case slog.KindTime:
				attr.Value = slog.StringValue(attr.Value.Any().(time.Time).Local().Format(timeFormat))
			case slog.KindAny:
				if src, ok := attr.Value.Any().(*slog.Source); ok {
					after := strings.SplitN(src.File, "juno/", 2)
					if len(after) == 2 {
						attr.Value = slog.StringValue(fmt.Sprintf("%s:%d", after[1], src.Line))
					}
				}
			}

			switch attr.Key {
			case slog.LevelKey:
				level := attr.Value.Any().(slog.Level)
				adapted := mustAdaptFromSlog(level)
				switch adapted {
				case FATAL:
					attr.Value = slog.StringValue(adapted.StringUpper())
				}
			}

			return attr
		},
	}))

	return &SlogLogger{Logger: logger}, nil
}

func (l *SlogLogger) Debugw(msg string, keysAndValues ...any)  { l.Debug(msg, keysAndValues...) }
func (l *SlogLogger) Infow(msg string, keysAndValues ...any)   { l.Info(msg, keysAndValues...) }
func (l *SlogLogger) Warnw(msg string, keysAndValues ...any)   { l.Warn(msg, keysAndValues...) }
func (l *SlogLogger) Errorw(msg string, keysAndValues ...any)  { l.Error(msg, keysAndValues...) }
func (l *SlogLogger) Infof(format string, args ...interface{}) { l.Info(fmt.Sprintf(format, args...)) }
func (l *SlogLogger) Fatalf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	l.Log(ctx, mustAdaptToSlog(FATAL), s)
	os.Exit(1)
}

type noopLogger struct{}

func NewNopLogger() *noopLogger {
	return &noopLogger{}
}

func (l *noopLogger) Debugw(msg string, keysAndValues ...any)   {}
func (l *noopLogger) Infow(msg string, keysAndValues ...any)    {}
func (l *noopLogger) Warnw(msg string, keysAndValues ...any)    {}
func (l *noopLogger) Errorw(msg string, keysAndValues ...any)   {}
func (l *noopLogger) Infof(format string, args ...interface{})  {}
func (l *noopLogger) Fatalf(format string, args ...interface{}) {}

func adaptToSlog(lvl LogLevel) (slog.Level, error) {
	switch lvl {
	case DEBUG:
		return slog.LevelDebug, nil
	case INFO:
		return slog.LevelInfo, nil
	case WARN:
		return slog.LevelWarn, nil
	case ERROR:
		return slog.LevelError, nil
	case FATAL:
		return slog.Level(9), nil
	default:
		return slog.Level(-1), ErrUnknownLogLevel
	}
}

func mustAdaptToSlog(lvl LogLevel) slog.Level {
	v, err := adaptToSlog(lvl)
	if err != nil {
		panic(err)
	}
	return v
}

func adaptFromSlog(lvl slog.Level) (LogLevel, error) {
	switch lvl {
	case slog.LevelDebug:
		return DEBUG, nil
	case slog.LevelInfo:
		return INFO, nil
	case slog.LevelWarn:
		return WARN, nil
	case slog.LevelError:
		return ERROR, nil
	case slog.Level(9):
		return FATAL, nil
	default:
		return LogLevel(-1), ErrUnknownLogLevel
	}
}

func mustAdaptFromSlog(lvl slog.Level) LogLevel {
	v, err := adaptFromSlog(lvl)
	if err != nil {
		panic(err)
	}
	return v
}
