package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

const metricCountKey = "count"

func parseSummaryMetrics(path string) (*summaryMetrics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var summary struct {
		Metrics map[string]map[string]any `json:"metrics"`
	}
	if err := decoder.Decode(&summary); err != nil {
		return nil, err
	}

	metric := func(name, key string, required bool) (float64, error) {
		entry := summary.Metrics[name]
		if entry == nil {
			if required {
				return 0, fmt.Errorf("missing metric %s.%s", name, key)
			}
			return 0, nil
		}
		value := entry[key]
		if value == nil {
			if values, ok := entry["values"].(map[string]any); ok {
				value = values[key]
			}
		}
		if value == nil {
			if required {
				return 0, fmt.Errorf("missing metric %s.%s", name, key)
			}
			return 0, nil
		}
		number, ok := value.(json.Number)
		if !ok {
			return 0, fmt.Errorf("invalid metric value for %s.%s", name, key)
		}
		parsed, err := number.Float64()
		if err != nil {
			return 0, fmt.Errorf("invalid metric value for %s.%s", name, key)
		}
		return parsed, nil
	}

	metrics := &summaryMetrics{}
	required := []struct {
		name, key string
		value     *float64
	}{
		{name: "checks", key: "fails", value: &metrics.FailedChecks},
		// k6 records true values as passes; true means failure for http_req_failed.
		{name: "http_req_failed", key: "passes", value: &metrics.HTTPRequestFailures},
		{name: "iterations", key: metricCountKey, value: &metrics.CompletedIterations},
	}
	for _, item := range required {
		*item.value, err = metric(item.name, item.key, true)
		if err != nil {
			return nil, err
		}
	}
	metrics.RequestFailures = metrics.FailedChecks
	metrics.VUFailures, err = metric("vu_failures", metricCountKey, false)
	if err != nil {
		return nil, err
	}
	metrics.DroppedIterations, err = metric("dropped_iterations", metricCountKey, false)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}
