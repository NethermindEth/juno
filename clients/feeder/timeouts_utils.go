package feeder

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	maxTimeout        = 2 * time.Minute
	defaultTimeout    = 5 * time.Second
	growthFactorFast  = 1.5
	growthFactorSlow  = 1.2
	fastGrowThreshold = 45 * time.Second
)

func generateTimeouts(initial time.Duration, count int) TimeoutConfig {
	timeouts := make(TimeoutConfig, count)
	if count == 0 {
		return timeouts
	}
	timeouts[0] = initial

	for i := 1; i < count; i++ {
		prev := timeouts[i-1]
		next := getNextTimeout(prev)
		timeouts[i] = next
	}

	return timeouts
}

func getNextTimeout(prev time.Duration) time.Duration {
	var next time.Duration
	if prev < fastGrowThreshold {
		seconds := math.Ceil(float64(prev.Seconds()) * growthFactorFast)
		return time.Duration(seconds) * time.Second
	} else {
		seconds := math.Ceil(float64(prev.Seconds()) * growthFactorSlow)
		next = time.Duration(seconds) * time.Second
	}
	if next > maxTimeout {
		next = maxTimeout
	}
	return next
}

func ParseTimeouts(values []string) (TimeoutConfig, error) {
	if len(values) == 0 {
		return TimeoutConfig{defaultTimeout}, nil
	}

	timeouts := make(TimeoutConfig, 0, len(values))
	for _, v := range values {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parsing timeouts: %v", err)
		}
		if d > maxTimeout {
			d = maxTimeout
		}
		timeouts = append(timeouts, d)
	}
	return timeouts, nil
}

func HTTPTimeoutsSettings(w http.ResponseWriter, r *http.Request, client *Client) {
	switch r.Method {
	case http.MethodGet:
		timeouts := make([]string, len(client.timeouts))
		for i, t := range client.timeouts {
			timeouts[i] = t.String()
		}
		fmt.Fprintf(w, "%s\n", strings.Join(timeouts, ","))
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
