---
title: Monitoring Juno
---

# Monitoring Juno :bar_chart:

Juno uses [Prometheus](https://prometheus.io/) and [pprof](https://github.com/google/pprof) to monitor and collect metrics and profiling data, which you can visualise with [Grafana](https://grafana.com/). Insights into your node's performance are useful for debugging, tuning, and understanding what is happening when Juno is running.

## Enable the metrics server

To enable the metrics server, use the following configuration options:

- `metrics`: Enables the prometheus metrics endpoint on the default port (disabled by default).
- `metrics-host`: The interface on which the prometheus endpoint will listen for requests. If skipped, it defaults to `localhost`.
- `metrics-port`: The port on which the prometheus endpoint will listen for requests. If skipped, it defaults to `9090`.

```bash
# Docker container
docker run -d \
  --name juno \
  -p 9090:9090 \
  nethermind/juno \
  --metrics \
  --metrics-port 9090 \
  --metrics-host 0.0.0.0

# Standalone binary
./build/juno --metrics --metrics-port=9090 --metrics-host=localhost
```

## Configure Grafana dashboard

### 1. Follow the [Set up Grafana](https://grafana.com/docs/grafana/latest/setup-grafana/) guide to set up Grafana locally

### 2. Download and [set up](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/#configuration-file-location) the [Grafana configuration file](/juno_grafana.json)

### 3. Configure the data source:

1. Follow the [Grafana data sources](https://grafana.com/docs/grafana/latest/datasources/) guide to add a data source.
2. Choose **Prometheus** as the data source.
3. Enter `http://localhost:9090/` as the **Prometheus server URL**.
4. Click the **"Save & Test"** button.
