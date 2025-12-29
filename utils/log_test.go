//nolint:dupl
package utils_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

var levelStrings = map[*utils.LogLevel]string{
	utils.NewLogLevel(utils.DEBUG): "debug",
	utils.NewLogLevel(utils.INFO):  "info",
	utils.NewLogLevel(utils.WARN):  "warn",
	utils.NewLogLevel(utils.ERROR): "error",
	utils.NewLogLevel(utils.TRACE): "trace",
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
			l := utils.NewLogLevel(utils.TRACE)
			require.NoError(t, l.Set(str))
			assert.Equal(t, *level, *l)
		})
		uppercase := strings.ToUpper(str)
		t.Run("level "+uppercase, func(t *testing.T) {
			l := utils.NewLogLevel(utils.TRACE)
			require.NoError(t, l.Set(uppercase))
			assert.Equal(t, *level, *l)
		})
	}

	t.Run("unknown log level", func(t *testing.T) {
		l := new(utils.LogLevel)
		require.ErrorIs(t, l.Set("blah"), utils.ErrUnknownLogLevel)
	})
}

func TestLogLevelUnmarshalText(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level "+str, func(t *testing.T) {
			l := utils.NewLogLevel(utils.TRACE)
			require.NoError(t, l.UnmarshalText([]byte(str)))
			assert.Equal(t, *level, *l)
		})
		uppercase := strings.ToUpper(str)
		t.Run("level "+uppercase, func(t *testing.T) {
			l := utils.NewLogLevel(utils.TRACE)
			require.NoError(t, l.UnmarshalText([]byte(uppercase)))
			assert.Equal(t, *level, *l)
		})
	}

	t.Run("unknown log level", func(t *testing.T) {
		l := new(utils.LogLevel)
		require.ErrorIs(t, l.UnmarshalText([]byte("blah")), utils.ErrUnknownLogLevel)
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
	assert.Equal(t, "LogLevel", new(utils.LogLevel).Type())
}

func TestZapWithColour(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level: "+str, func(t *testing.T) {
			_, err := utils.NewZapLogger(level, true)
			assert.NoError(t, err)
		})
	}
}

func TestZapWithoutColour(t *testing.T) {
	for level, str := range levelStrings {
		t.Run("level: "+str, func(t *testing.T) {
			_, err := utils.NewZapLogger(level, false)
			assert.NoError(t, err)
		})
	}
}

func TestHTTPLogSettings(t *testing.T) {
	logLevel := utils.NewLogLevel(utils.INFO)
	ctx := t.Context()

	t.Run("GET current log level", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/log/level", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			utils.HTTPLogSettings(w, r, logLevel)
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
			utils.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced log level with 'debug' successfully\n", rr.Body.String())
		assert.Equal(t, utils.DEBUG, logLevel.Level())
	})

	t.Run("PUT update log level with missing parameter", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, "/log/level", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			utils.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "missing level query parameter\n", rr.Body.String())
	})

	t.Run("PUT update log level with invalid level", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, "/log/level?level=invalid", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			utils.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, rr.Body.String(), fmt.Sprint(utils.ErrUnknownLogLevel)+"\n")
	})

	t.Run("Method not allowed", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/log/level", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			utils.HTTPLogSettings(w, r, logLevel)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		assert.Equal(t, "Method not allowed\n", rr.Body.String())
	})
}

func TestMarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		logLevel utils.LogLevel
		expected string
	}{
		{"InfoLevel", *utils.NewLogLevel(utils.INFO), "info"},
		{"DebugLevel", *utils.NewLogLevel(utils.DEBUG), "debug"},
		{"ErrorLevel", *utils.NewLogLevel(utils.ERROR), "error"},
		{"WarnLevel", *utils.NewLogLevel(utils.WARN), "warn"},
		{"TraceLevel", *utils.NewLogLevel(utils.TRACE), "trace"},
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
		logLevel := utils.NewLogLevel(utils.TRACE)
		logger, err := utils.NewZapLogger(logLevel, false)
		require.NoError(t, err)
		assert.True(t, logger.IsTraceEnabled())
	})

	t.Run("Trace disabled", func(t *testing.T) {
		logLevel := utils.NewLogLevel(utils.INFO)
		logger, err := utils.NewZapLogger(logLevel, false)
		require.NoError(t, err)
		assert.False(t, logger.IsTraceEnabled())
	})
}

func TestTracew(t *testing.T) {
	t.Run("Tracew with trace level enabled", func(t *testing.T) {
		logLevel := utils.NewLogLevel(utils.TRACE)

		var buf bytes.Buffer
		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
			zapcore.AddSync(&buf),
			logLevel.Level(),
		)
		zapLogger := utils.NewZapLoggerWithCore(core)

		expectedMessage := "trace message"
		zapLogger.Tracew(expectedMessage)

		logOutput := buf.String()
		assert.Contains(t, logOutput, expectedMessage)
	})

	t.Run("Tracew with trace level disabled", func(t *testing.T) {
		logLevel := utils.NewLogLevel(utils.INFO)

		var buf bytes.Buffer
		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
			zapcore.AddSync(&buf),
			logLevel.Level(),
		)
		zapLogger := utils.NewZapLoggerWithCore(core)

		expectedMessage := "trace message"
		zapLogger.Tracew(expectedMessage)

		logOutput := buf.String()
		assert.NotContains(t, logOutput, expectedMessage)
	})
}
