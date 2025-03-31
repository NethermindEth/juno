// timeouts.go implements adaptive timeout management for HTTP requests to Starknet nodes.
// This file handles dynamic timeout adjustments based on request performance, automatically
// scaling timeouts up or down depending on success/failure rates.

package feeder

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimeout      = 5 * time.Second
	growthFactorFast    = 2
	growthFactorMedium  = 1.5
	growthFactorSlow    = 1.2
	fastGrowThreshold   = 1 * time.Minute
	mediumGrowThreshold = 2 * time.Minute
	timeoutsCount       = 30
)

type Timeouts struct {
	timeouts   []time.Duration
	curTimeout int
}

func (t Timeouts) GetCurrentTimeout() time.Duration {
	return t.timeouts[t.curTimeout]
}

func (t *Timeouts) DecreaseTimeout() {
	if t.curTimeout > 0 {
		t.curTimeout--
	}
}

func (t *Timeouts) IncreaseTimeout() {
	t.curTimeout++
	if t.curTimeout >= timeoutsCount {
		t.curTimeout = timeoutsCount - 1
	}
}

func (t *Timeouts) UseDefaultTimeouts() {
	t.timeouts = timeoutsListFromNumber(defaultTimeout, timeoutsCount)
}

func (t Timeouts) String() string {
	timeouts := make([]string, len(t.timeouts))
	for i, t := range t.timeouts {
		timeouts[i] = t.String()
	}
	return strings.Join(timeouts, ",")
}

// timeoutsListFromNumber generates a list of timeouts based on the initial timeout and the number of retries.
// The list is generated using a geometric progression with a growth factor of 2 for the first 1 minute,
// 1.5 for the next 1 minute, and 1.2 for the rest.
func timeoutsListFromNumber(initial time.Duration, count int) []time.Duration {
	timeouts := make([]time.Duration, count)
	timeouts[0] = initial

	for i := 1; i < count; i++ {
		prev := timeouts[i-1]
		next := increaseDuration(prev)
		timeouts[i] = next
	}

	return timeouts
}

func getTimeouts(timeouts []time.Duration, count int) Timeouts {
	var timeoutsList []time.Duration
	if len(timeouts) == 0 {
		timeoutsList = timeoutsListFromNumber(defaultTimeout, count)
	} else {
		if len(timeouts) > 0 {
			sort.Slice(timeouts, func(i, j int) bool {
				return timeouts[i] < timeouts[j]
			})
		}

		if len(timeouts) > count {
			timeoutsList = timeouts[:count]
		} else {
			count := count + 1 - len(timeouts)
			next := increaseDuration(timeouts[len(timeouts)-1])
			timeouts := append(timeouts, timeoutsListFromNumber(next, count)...)
			timeoutsList = timeouts
		}
	}
	return Timeouts{
		curTimeout: 0,
		timeouts:   timeoutsList,
	}
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

func ParseTimeouts(values []string) ([]time.Duration, error) {
	if len(values) == 0 {
		return []time.Duration{defaultTimeout}, nil
	}

	timeouts := make([]time.Duration, 0, len(values))
	for i, v := range values {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parsing timeouts at index %d: %v", i, err)
		}
		timeouts = append(timeouts, d)
	}
	return timeouts, nil
}

func HTTPTimeoutsSettings(w http.ResponseWriter, r *http.Request, client *Client) {
	switch r.Method {
	case http.MethodGet:
		fmt.Fprintf(w, "%s\n", client.timeouts.String())
	case http.MethodPut:
		timeoutsStr := r.URL.Query().Get("timeouts")
		if timeoutsStr == "" {
			http.Error(w, "missing timeouts query parameter", http.StatusBadRequest)
			return
		}

		timeoutStrs := strings.Split(timeoutsStr, ",")
		timeouts := make([]string, 0, len(timeoutStrs))
		for _, t := range timeoutStrs {
			t = strings.TrimSpace(t)
			if t != "" {
				timeouts = append(timeouts, t)
			}
		}

		newTimeouts, err := ParseTimeouts(timeouts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		client.WithTimeouts(newTimeouts)
		fmt.Fprintf(w, "Replaced timeouts with '%s' successfully\n", timeoutsStr)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
