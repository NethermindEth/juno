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

	metric := func(name, key string) (float64, error) {
		entry := summary.Metrics[name]
		if entry == nil {
			return 0, nil
		}
		value := entry[key]
		if value == nil {
			if values, ok := entry["values"].(map[string]any); ok {
				value = values[key]
			}
		}
		if value == nil {
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

	values := make([]float64, len(keysForSummaryMetrics()))
	keys := keysForSummaryMetrics()
	for i, key := range keys {
		values[i], err = metric(key[0], key[1])
		if err != nil {
			return nil, err
		}
	}
	return &summaryMetrics{
		FailedChecks: values[0], RequestFailures: values[1],
		HTTPRequestFailures: values[2], VUFailures: values[3],
		DroppedIterations: values[4], CompletedIterations: values[5],
	}, nil
}

func keysForSummaryMetrics() [][2]string {
	return [][2]string{
		{"checks", "fails"},
		{"rpc_request_failures", metricCountKey},
		{"http_req_failed", "passes"},
		{"vu_failures", metricCountKey},
		{"dropped_iterations", metricCountKey},
		{"iterations", metricCountKey},
	}
}
