//nolint:dupl
package log_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var levelStrings = map[*log.Level]string{
	log.NewLevel(log.DEBUG): "debug",
	log.NewLevel(log.INFO):  "info",
	log.NewLevel(log.WARN):  "warn",
	log.NewLevel(log.ERROR): "error",
	log.NewLevel(log.TRACE): "trace",
}

func TestLogLevelString(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level "+str, func(t *testing.T) {
			assert.Equal(t, str, level.String())
		})
	}
}

// Tests are similar for LogLevel and Network since they
// both implement the pflag.Value and encoding.TextUnmarshaller interfaces.
// We can open a PR on github.com/thediveo/enumflag to add TextUnmarshaller
// support.
func TestLogLevelSet(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level "+str, func(t *testing.T) {
			l := log.NewLevel(log.TRACE)
			require.NoError(t, l.Set(str))
			assert.Equal(t, *level, *l)
		})
		uppercase := strings.ToUpper(str)
		t.Run("level "+uppercase, func(t *testing.T) {
			l := log.NewLevel(log.TRACE)
			require.NoError(t, l.Set(uppercase))
			assert.Equal(t, *level, *l)
		})
	}

	t.Run("unknown log level", func(t *testing.T) {
		l := new(log.Level)
		require.ErrorIs(t, l.Set("blah"), log.ErrUnknownLogLevel)
	})
}

func TestLogLevelUnmarshalText(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level "+str, func(t *testing.T) {
			l := log.NewLevel(log.TRACE)
			require.NoError(t, l.UnmarshalText([]byte(str)))
			assert.Equal(t, *level, *l)
		})
		uppercase := strings.ToUpper(str)
		t.Run("level "+uppercase, func(t *testing.T) {
			l := log.NewLevel(log.TRACE)
			require.NoError(t, l.UnmarshalText([]byte(uppercase)))
			assert.Equal(t, *level, *l)
		})
	}

	t.Run("unknown log level", func(t *testing.T) {
		l := new(log.Level)
		require.ErrorIs(t, l.UnmarshalText([]byte("blah")), log.ErrUnknownLogLevel)
	})
}

func TestLogLevelMarshalJSON(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level "+str, func(t *testing.T) {
			lb, err := json.Marshal(&level)
			require.NoError(t, err)

			expectedStr := `"` + str + `"`
			assert.Equal(t, expectedStr, string(lb))
		})
	}
}

func TestLogLevelType(t *testing.T) {
	assert.Equal(t, "LogLevel", new(log.Level).Type())
}

func TestZapWithColour(t *testing.T) {
	var buf bytes.Buffer
	logLevel := log.NewLevel(log.INFO)
	logger, err := log.NewZapLogger(
		logLevel, log.WithWriter(&buf), log.WithColour(true),
	)
	require.NoError(t, err)

	logger.Info("colour test message")

	output := buf.String()
	assert.Contains(t, output, "colour test message")
	assert.Contains(t, output, "\x1b[")
}

func TestZapWithoutColour(t *testing.T) {
	var buf bytes.Buffer
	logLevel := log.NewLevel(log.INFO)
	logger, err := log.NewZapLogger(
		logLevel, log.WithWriter(&buf),
	)
	require.NoError(t, err)

	logger.Info("no colour message")

	output := buf.String()
	assert.Contains(t, output, "no colour message")
	assert.NotContains(t, output, "\x1b[")
}

func TestZapWithJSON(t *testing.T) {
	logLevel := log.NewLevel(log.INFO)

	t.Run("produces valid JSON output", func(t *testing.T) {
		var buf bytes.Buffer
		logger, err := log.NewZapLogger(
			logLevel, log.WithJSON(true), log.WithWriter(&buf),
		)
		require.NoError(t, err)

		logger.Info("test message", zap.String("key", "value"))

		var m map[string]any
		err = json.Unmarshal(buf.Bytes(), &m)
		require.NoError(t, err)
		assert.Equal(t, "test message", m["msg"])
		assert.Equal(t, "value", m["key"])
		assert.Equal(t, "INFO", m["level"])
	})

	t.Run("console output is not JSON", func(t *testing.T) {
		var buf bytes.Buffer
		logger, err := log.NewZapLogger(
			logLevel, log.WithJSON(false), log.WithWriter(&buf),
		)
		require.NoError(t, err)

		logger.Info("hello console")

		output := buf.String()
		assert.Contains(t, output, "hello console")

		var m map[string]any
		err = json.Unmarshal(buf.Bytes(), &m)
		assert.Error(t, err, "console output should not be valid JSON")
	})

	t.Run("JSON ignores colour option", func(t *testing.T) {
		var buf bytes.Buffer
		logger, err := log.NewZapLogger(
			logLevel,
			log.WithJSON(true),
			log.WithColour(true),
			log.WithWriter(&buf),
		)
		require.NoError(t, err)

		logger.Info("json colour test")

		output := buf.String()
		assert.NotContains(t, output, "\x1b[")

		var m map[string]any
		err = json.Unmarshal(buf.Bytes(), &m)
		require.NoError(t, err)
		assert.Equal(t, "INFO", m["level"])
	})

	t.Run("JSON timestamp is ISO8601", func(t *testing.T) {
		var buf bytes.Buffer
		logger, err := log.NewZapLogger(
			logLevel, log.WithJSON(true), log.WithWriter(&buf),
		)
		require.NoError(t, err)

		logger.Info("timestamp test")

		var m map[string]any
		err = json.Unmarshal(buf.Bytes(), &m)
		require.NoError(t, err)

		ts, ok := m["ts"].(string)
		require.True(t, ok)
		_, err = time.Parse("2006-01-02T15:04:05.000Z0700", ts)
		assert.NoError(t, err)
	})
}

func TestConsoleOutput(t *testing.T) {
	t.Run("timestamp format", func(t *testing.T) {
		var buf bytes.Buffer
		logLevel := log.NewLevel(log.INFO)
		logger, err := log.NewZapLogger(
			logLevel, log.WithWriter(&buf),
		)
		require.NoError(t, err)

		logger.Info("timestamp test")

		output := buf.String()
		pattern := `\d{2}:\d{2}:\d{2}\.\d{3} \d{2}/\d{2}/\d{4} [+-]\d{2}:\d{2}`
		assert.Regexp(t, regexp.MustCompile(pattern), output)
	})

	t.Run("TRACE level is cyan with colour", func(t *testing.T) {
		var buf bytes.Buffer
		logLevel := log.NewLevel(log.TRACE)
		logger, err := log.NewZapLogger(
			logLevel, log.WithWriter(&buf), log.WithColour(true),
		)
		require.NoError(t, err)

		logger.Trace("cyan trace message")

		output := buf.String()
		assert.Contains(t, output, "\x1b[36mTRACE\x1b[0m")
	})
}

func TestHTTPLogSettings(t *testing.T) {
	logLevel := log.NewLevel(log.INFO)
	ctx := t.Context()

	t.Run("GET current log level", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/log/level", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "info\n", rr.Body.String())
	})

	t.Run("PUT update log level", func(t *testing.T) {
		req, err := http.NewRequestWithContext(
			ctx, http.MethodPut, "/log/level?level=debug", http.NoBody,
		)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced log level with 'debug' successfully\n", rr.Body.String())
		assert.Equal(t, log.DEBUG, logLevel.Level())
	})

	t.Run("PUT update log level with missing parameter", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, "/log/level", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "missing level query parameter\n", rr.Body.String())
	})

	t.Run("PUT update log level with invalid level", func(t *testing.T) {
		req, err := http.NewRequestWithContext(
			ctx, http.MethodPut, "/log/level?level=invalid", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, rr.Body.String(), fmt.Sprint(log.ErrUnknownLogLevel)+"\n")
	})

	t.Run("Method not allowed", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/log/level", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		assert.Equal(t, "Method not allowed\n", rr.Body.String())
	})
}

func TestMarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		logLevel log.Level
		expected string
	}{
		{"InfoLevel", *log.NewLevel(log.INFO), "info"},
		{"DebugLevel", *log.NewLevel(log.DEBUG), "debug"},
		{"ErrorLevel", *log.NewLevel(log.ERROR), "error"},
		{"WarnLevel", *log.NewLevel(log.WARN), "warn"},
		{"TraceLevel", *log.NewLevel(log.TRACE), "trace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(tt.logLevel)
			assert.NoError(t, err)
			assert.Contains(t, string(data), tt.expected)
		})
	}
}

func TestIsTraceEnabled(t *testing.T) {
	t.Run("Trace enabled", func(t *testing.T) {
		logLevel := log.NewLevel(log.TRACE)
		logger, err := log.NewZapLogger(logLevel)
		require.NoError(t, err)
		assert.True(t, logger.IsTraceEnabled())
	})

	t.Run("Trace disabled", func(t *testing.T) {
		logLevel := log.NewLevel(log.INFO)
		logger, err := log.NewZapLogger(logLevel)
		require.NoError(t, err)
		assert.False(t, logger.IsTraceEnabled())
	})
}

func TestTrace(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		var buf bytes.Buffer
		logLevel := log.NewLevel(log.TRACE)
		logger, err := log.NewZapLogger(
			logLevel, log.WithWriter(&buf),
		)
		require.NoError(t, err)

		logger.Trace("trace message")

		output := buf.String()
		assert.Contains(t, output, "trace message")
		assert.Contains(t, output, "TRACE")
	})

	t.Run("disabled", func(t *testing.T) {
		var buf bytes.Buffer
		logLevel := log.NewLevel(log.INFO)
		logger, err := log.NewZapLogger(
			logLevel, log.WithWriter(&buf),
		)
		require.NoError(t, err)

		logger.Trace("trace message")

		assert.Empty(t, buf.String())
	})
}

func TestSanitizeString(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"empty":                 {"", ""},
		"no special chars":      {"hello world", "hello world"},
		"strips LF":             {"hello\nworld", "helloworld"},
		"strips CR":             {"hello\rworld", "helloworld"},
		"strips CRLF":           {"hello\r\nworld", "helloworld"},
		"strips multiple":       {"a\nb\nc\r\nd", "abcd"},
		"forged log entry":      {"foo\nINFO\tfake entry", "fooINFO\tfake entry"},
		"preserves tabs/spaces": {"a\tb c", "a\tb c"},
		"preserves unicode":     {"héllo→wörld", "héllo→wörld"},
		"only newlines":         {"\n\r\n\r", ""},
		"leading/trailing LF":   {"\nmiddle\n", "middle"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, log.SanitizeString(tc.input))
		})
	}
}
