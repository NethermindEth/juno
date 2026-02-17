// timeouts.go implements adaptive timeout management for HTTP requests to Starknet nodes.
// This file handles dynamic timeout adjustments based on request performance, automatically
// scaling timeouts up or down depending on success/failure rates.

package feeder

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	growthFactorFast    = 2
	growthFactorMedium  = 1.5
	growthFactorSlow    = 1.2
	fastGrowThreshold   = 1 * time.Minute
	mediumGrowThreshold = 2 * time.Minute
	timeoutsCount       = 30
	DefaultTimeouts     = "5s"
)

type Timeouts struct {
	timeouts   []time.Duration
	curTimeout int
	mu         sync.RWMutex
}

func (t *Timeouts) GetCurrentTimeout() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.timeouts[t.curTimeout]
}

func (t *Timeouts) DecreaseTimeout() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.curTimeout > 0 {
		t.curTimeout--
	}
}

func (t *Timeouts) IncreaseTimeout() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.curTimeout++
	if t.curTimeout >= len(t.timeouts) {
		t.curTimeout = len(t.timeouts) - 1
	}
}

func (t *Timeouts) String() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	timeouts := make([]string, len(t.timeouts))
	for i, t := range t.timeouts {
		timeouts[i] = t.String()
	}
	return strings.Join(timeouts, ",")
}

// timeoutsListFromNumber generates a list of timeouts based on the initial timeout and the number of retries.
// The list is generated using a geometric progression with a growth factor of 2 for the first 1 minute,
// 1.5 for the next 1 minute, and 1.2 for the rest.
func timeoutsListFromNumber(initial time.Duration) []time.Duration {
	timeouts := make([]time.Duration, timeoutsCount)
	timeouts[0] = initial

	for i := 1; i < timeoutsCount; i++ {
		prev := timeouts[i-1]
		next := increaseDuration(prev)
		timeouts[i] = next
	}

	return timeouts
}

func increaseDuration(prev time.Duration) time.Duration {
	var next time.Duration
	if prev < fastGrowThreshold {
		seconds := math.Ceil(float64(prev.Seconds()) * growthFactorFast)
		return time.Duration(seconds) * time.Second
	} else if prev < mediumGrowThreshold {
		seconds := math.Ceil(float64(prev.Seconds()) * growthFactorMedium)
		return time.Duration(seconds) * time.Second
	} else {
		seconds := math.Ceil(float64(prev.Seconds()) * growthFactorSlow)
		next = time.Duration(seconds) * time.Second
	}
	return next
}

func getDynamicTimeouts(timeouts time.Duration) Timeouts {
	return Timeouts{
		curTimeout: 0,
		timeouts:   timeoutsListFromNumber(timeouts),
	}
}

func getFixedTimeouts(timeouts []time.Duration) Timeouts {
	return Timeouts{
		curTimeout: 0,
		timeouts:   timeouts,
	}
}

func getDefaultFixedTimeouts() Timeouts {
	timeouts, _, _ := ParseTimeouts(DefaultTimeouts)
	return getFixedTimeouts(timeouts)
}

// ParseTimeouts parses a comma-separated string of duration values into a slice of time.Duration.
// Returns:
// - the parsed timeout values
// - if a fixed or dynamic timeouts should be used
// - an error in case the string cannot be parsed
func ParseTimeouts(value string) ([]time.Duration, bool, error) {
	if value == "" {
		return nil, true, fmt.Errorf("timeouts are not set")
	}

	values := strings.Split(value, ",")
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}

	hasTrailingComma := len(values) > 0 && values[len(values)-1] == ""
	if hasTrailingComma {
		values = values[:len(values)-1]
	}

	timeouts := make([]time.Duration, 0, len(values))
	for i, v := range values {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, false, fmt.Errorf("parsing timeout parameter number %d: %v", i+1, err)
		}
		timeouts = append(timeouts, d)
	}
	if len(timeouts) == 1 && hasTrailingComma {
		return timeouts, true, nil
	}

	for i := 1; i < len(timeouts); i++ {
		if timeouts[i] <= timeouts[i-1] {
			return nil, false, fmt.Errorf("timeout values must be in ascending order, got %v <= %v", timeouts[i], timeouts[i-1])
		}
	}

	if len(timeouts) > timeoutsCount {
		return nil, false, fmt.Errorf("exceeded max amount of allowed timeout parameters. Set %d but max is %d", len(timeouts), timeoutsCount)
	}
	return timeouts, false, nil
}

func HTTPTimeoutsSettings(w http.ResponseWriter, r *http.Request, client *Client) {
	switch r.Method {
	case http.MethodGet:
		timeouts := client.timeouts.Load()
		fmt.Fprintf(w, "%s\n", timeouts.String())
	case http.MethodPut:
		timeoutsStr := r.URL.Query().Get("timeouts")
		if timeoutsStr == "" {
			http.Error(w, "missing timeouts query parameter", http.StatusBadRequest)
			return
		}

		newTimeouts, fixed, err := ParseTimeouts(timeoutsStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		client.WithTimeouts(newTimeouts, fixed)
		fmt.Fprintf(w, "Replaced timeouts with '%s' successfully\n", html.EscapeString(timeoutsStr))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
