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
	maxTimeout        = 2 * time.Minute
	defaultTimeout    = 5 * time.Second
	growthFactorFast  = 1.5
	growthFactorSlow  = 1.2
	fastGrowThreshold = 45 * time.Second
)

type TimeoutsList []time.Duration

func generateTimeoutsListFromInitial(initial time.Duration, count int) TimeoutsList {
	timeouts := make(TimeoutsList, count)
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

func getTimeouts(timeouts TimeoutsList, maxRetries int) TimeoutsList {
	if len(timeouts) == 0 {
		return generateTimeoutsListFromInitial(defaultTimeout, maxRetries+1)
	} else {
		if len(timeouts) > 0 {
			sort.Slice(timeouts, func(i, j int) bool {
				return timeouts[i] < timeouts[j]
			})
		}

		if len(timeouts) > maxRetries+1 {
			return timeouts[:maxRetries+1]
		} else {
			count := maxRetries + 1 - len(timeouts)
			next := getNextTimeout(timeouts[len(timeouts)-1])
			timeouts := append(timeouts, generateTimeoutsListFromInitial(next, count)...)
			return timeouts
		}
	}
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

func ParseTimeouts(values []string) (TimeoutsList, error) {
	if len(values) == 0 {
		return TimeoutsList{defaultTimeout}, nil
	}

	timeouts := make(TimeoutsList, 0, len(values))
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
