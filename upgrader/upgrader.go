package upgrader

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/utils"
)

type Upgrader struct {
	client         *http.Client
	log            utils.StructuredLogger
	apiURL         string
	currentVersion *semver.Version
	releasesURL    string
	delay          time.Duration
}

func NewUpgrader(version *semver.Version, apiURL, releasesURL string, delay time.Duration, log utils.StructuredLogger) *Upgrader {
	return &Upgrader{
		currentVersion: version,
		client:         &http.Client{},
		log:            log,
		apiURL:         apiURL,
		releasesURL:    releasesURL,
		delay:          delay,
	}
}

func (u *Upgrader) WithClient(client *http.Client) *Upgrader {
	u.client = client
	return u
}

func (u *Upgrader) WithLog(log utils.StructuredLogger) *Upgrader {
	u.log = log
	return u
}

type Release struct {
	Version    *semver.Version `json:"tag_name"`
	Draft      bool            `json:"draft"`
	PreRelease bool            `json:"prerelease"`
}

func (u *Upgrader) Run(ctx context.Context) error {
	timer := time.NewTimer(time.Millisecond) // Don't wait the first time.
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			var req *http.Request
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.apiURL, http.NoBody)
			if err != nil {
				u.log.Debug("Failed to create new request with context")
				continue
			}

			resp, err := u.client.Do(req)
			if err != nil {
				u.log.Debug("Failed to fetch latest release", utils.SugaredFields("err", err)...)
				continue
			} else if resp.StatusCode != http.StatusOK {
				u.log.Debug("Failed to fetch latest release", utils.SugaredFields("status", resp.Status)...)
				continue
			}

			latest := new(Release)
			if err := json.NewDecoder(resp.Body).Decode(latest); err == nil {
				if needsUpdate(*u.currentVersion, *latest.Version) {
					u.log.Warn("New release is available.",
						utils.SugaredFields(
							"currentVersion", u.currentVersion.String(),
							"newVersion", latest.Version.String(),
							"link", u.releasesURL,
						)...,
					)
				} else {
					u.log.Debug("Application is up-to-date.")
				}
			} else {
				u.log.Debug("Failed to unmarshal latest release")
			}

			timer.Reset(u.delay)
			resp.Body.Close()
		}
	}
}

// needsUpdate compares major, minor, and patch versions of the currentVersion and latestVersion.
// It returns true if the latestVersion is greater than the currentVersion and false otherwise.
//
// It doesn't consider:
//   - metadata, such as commit hashes.
//   - rc releases.
func needsUpdate(currentVersion, latestVersion semver.Version) bool {
	if currentVersion.Major() == latestVersion.Major() {
		if currentVersion.Minor() < latestVersion.Minor() {
			return true
		} else if currentVersion.Minor() == latestVersion.Minor() {
			if currentVersion.Patch() < latestVersion.Patch() {
				return true
			}
		}
	} else if currentVersion.Major() < latestVersion.Major() {
		return true
	}
	return false
}
