---
title: Monitoring Juno
---

# Monitoring Juno :bar_chart:

Juno uses [Prometheus](https://prometheus.io/) to monitor and collect metrics data, which you can visualise with [Grafana](https://grafana.com/). You can use these insights to understand what is happening when Juno is running.

## Enable the metrics server

To enable the metrics server, use the following configuration options:

- `metrics`: Enables the Prometheus metrics endpoint on the default port (disabled by default).
- `metrics-host`: The interface on which the Prometheus endpoint will listen for requests. If skipped, it defaults to `localhost`.
- `metrics-port`: The port on which the Prometheus endpoint will listen for requests. If skipped, it defaults to `9090`.

```bash
# Docker container
docker run -d \
  --name juno \
  -p 9090:9090 \
  nethermind/juno \
  --metrics \
  --metrics-port 9090 \
  --metrics-host 0.0.0.0 \
  --eth-node <YOUR-ETH-NODE>


# Standalone binary
./build/juno --metrics --metrics-port 9090 --metrics-host=0.0.0.0 --eth-node <YOUR-ETH-NODE>
```

## Configure Grafana dashboard

The Juno Grafana dashboard is optimized for single-node deployments and uses generic metric selectors that work with standard Prometheus configurations.

### 1. Set up Grafana

- Follow the [Set up Grafana](https://grafana.com/docs/grafana/latest/setup-grafana/) guide to install Grafana.
- Download and [configure](https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/#configuration-file-location) the [Grafana dashboard file](/juno_grafana.json).

### 2. Set up Prometheus

- Follow the [First steps with Prometheus](https://prometheus.io/docs/introduction/first_steps/) guide to install Prometheus.
- Add the Juno metrics endpoint in the `prometheus.yml` configuration file:

```yaml title="prometheus.yml" showLineNumbers
scrape_configs:
  - job_name: "juno"
    static_configs:
      - targets: ["localhost:9090"]
```

### 3. Set up Grafana Loki

- Follow the [Get started with Grafana Loki](https://grafana.com/docs/loki/latest/get-started/) guide to install [Loki](https://grafana.com/oss/loki/).
- Configure Loki to [collect logs](https://grafana.com/docs/loki/latest/send-data/) from Juno. You might need to configure log paths or use [Promtail](https://grafana.com/docs/loki/latest/send-data/promtail/) (Loki's agent) to send logs to Loki:

```yaml title="Sample Loki Configuration" showLineNumbers
scrape_configs:
  - job_name: "juno-logs"
    labels:
      job: "juno"
      __path__: "/path/to/juno/logs/*"
```

:::tip
To have Juno write logs to a file, use the command:

```bash
./build/juno >> /path/juno.log 2>&1
```

:::

### 4. Configure the data sources

1. Follow the [Grafana data sources](https://grafana.com/docs/grafana/latest/datasources/) guide to add data sources.
2. Choose **Prometheus** as a data source:

- Enter the URL where Prometheus is running, e.g., `http://localhost:9090`.
- Click the **"Save & Test"** button.

3. Choose **Loki** as a data source:

- Enter the URL where Loki is running, e.g., `http://localhost:3100`.
- Click the **"Save & Test"** button.

![Grafana dashboard](/img/grafana-1.png)

![Grafana dashboard](/img/grafana-2.png)

## Change log level in runtime

In case you want to change the log level in runtime without the need to restart the juno process, you can do it via HTTP calls.

To enable this feature, use the following configuration options:

- `log-host`: The interface to listen for requests. Defaults to `localhost`.
- `log-port`: The port to listen for requests.  REQUIRED

Examples:

```console
# Start juno specifying the log port
juno --log-port=6789 --log-level=error ...

# Get current level
curl -X GET 'localhost:6789/log/level'
error

# Set level
curl -X PUT 'localhost:6789/log/level?level=trace'
Replaced log level with 'trace' successfully

# Get new level
curl -X GET 'localhost:6789/log/level'
trace
```
