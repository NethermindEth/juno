package upgrader_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/upgrader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type upgradeLogger struct {
	infoMsg  string
	warnMsg  string
	errorMsg string
	traceMsg string
}

func (l *upgradeLogger) Debug(msg string, _ ...zap.Field) {}

func (l *upgradeLogger) Info(msg string, _ ...zap.Field) {
	l.infoMsg = msg
}

func (l *upgradeLogger) Warn(msg string, _ ...zap.Field) {
	l.warnMsg = msg
}

func (l *upgradeLogger) Error(msg string, _ ...zap.Field) {
	l.errorMsg = msg
}

func (l *upgradeLogger) Trace(msg string, _ ...zap.Field) {
	l.traceMsg = msg
}

func newVersion(t *testing.T, v string) semver.Version {
	version, err := semver.StrictNewVersion(v)
	require.NoError(t, err)
	return *version
}

func TestUpgrader(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		current semver.Version
		latest  semver.Version
		equal   bool
	}{
		"up to date": {
			current: newVersion(t, "1.2.0"),
			latest:  newVersion(t, "1.2.0"),
			equal:   true,
		},
		"new patch version": {
			current: newVersion(t, "1.2.0"),
			latest:  newVersion(t, "1.2.1"),
		},
		"new minor version": {
			current: newVersion(t, "1.2.0"),
			latest:  newVersion(t, "1.3.0"),
		},
		"new major version": {
			current: newVersion(t, "1.2.0"),
			latest:  newVersion(t, "2.2.0"),
		},
		"new major and minor version": {
			current: newVersion(t, "1.2.0"),
			latest:  newVersion(t, "2.3.0"),
		},
		"new rc release": {
			current: newVersion(t, "1.2.0"),
			latest:  newVersion(t, "1.2.0-rc0"),
			equal:   true,
		},
		"outdated rc release": {
			current: newVersion(t, "1.2.0-rc0"),
			latest:  newVersion(t, "1.2.1"),
		},
		"consecutive rc releases": {
			current: newVersion(t, "1.2.0-rc1"),
			latest:  newVersion(t, "1.2.0-rc2"),
			equal:   true,
		},
		"non-semver equal": {
			current: newVersion(t, "1.2.0-deadbeef"),
			latest:  newVersion(t, "1.2.0"),
			equal:   true,
		},
		"non-semver rc equal": {
			current: newVersion(t, "1.2.0-deadbeef"),
			latest:  newVersion(t, "1.2.0-rc0"),
			equal:   true,
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				require.Equal(t, "GET", req.Method)

				_, err := url.ParseQuery(req.URL.RawQuery)
				if err != nil {
					rw.WriteHeader(http.StatusBadRequest)
					return
				}

				release := &upgrader.Release{
					Version: &test.latest,
				}

				releaseBytes, err := json.Marshal(release)
				require.NoError(t, err)
				_, err = rw.Write(releaseBytes)
				require.NoError(t, err)
			}))
			t.Cleanup(srv.Close)
			log := &upgradeLogger{}
			ug := upgrader.NewUpgrader(
				&test.current, srv.URL, "example.com/releases", time.Millisecond, log,
			)

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			t.Cleanup(cancel)

			require.NoError(t, ug.Run(ctx))

			if test.equal {
				assert.Empty(t, log.warnMsg)
			} else {
				assert.Equal(t, "New release is available.", log.warnMsg)
			}
			assert.Empty(t, log.errorMsg)
			assert.Empty(t, log.infoMsg)
		})
	}
}
