---
title: Deploy on Kubernetes
---

# Running Juno on Kubernetes :wheel_of_dharma:

You can deploy Juno on Kubernetes using the official [Helm chart](https://github.com/NethermindEth/helm-charts/tree/main/charts/juno). The chart deploys a Juno Starknet full node as a StatefulSet with persistent storage. Optionally, a [staking validator](staking-validator) service can be enabled alongside.

## Prerequisites

- A running Kubernetes cluster (v1.23+)
- [Helm](https://helm.sh/docs/intro/install/) v3 installed
- [kubectl](https://kubernetes.io/docs/tasks/tools/) configured for your cluster
- An Ethereum node WebSocket endpoint (or use `--disable-l1-verification` for testing)

## 1. Add the Helm repository

```bash
helm repo add nethermind https://nethermindeth.github.io/helm-charts
helm repo update
```

## 2. Install the chart

```bash
# For testing (disables L1 verification, no Ethereum node required)
helm install my-juno nethermind/juno \
  --set juno.extraArgs[0]="--disable-l1-verification"
```

This installs Juno with the default configuration (Sepolia network) and disables L1 verification so it can start without an Ethereum node. For production use or to enable full L1 verification, configure an Ethereum node endpoint via `juno.extraArgs` in a `values.yaml` file, for example:

```yaml title="values.yaml"
juno:
  network: mainnet
  image:
    tag: "v0.16.0"
  extraArgs:
    - --eth-node=wss://mainnet.infura.io/ws/v3/<YOUR-PROJECT-ID>
  persistence:
    size: 1Ti
    storageClassName: "your-storage-class"
  resources:
    requests:
      cpu: "4"
      memory: "8Gi"
    limits:
      cpu: "8"
      memory: "16Gi"
```

Then install with your values:

```bash
helm install my-juno nethermind/juno -f values.yaml
```

:::caution
You must provide either `--eth-node <WS endpoint>` or `--disable-l1-verification` in `juno.extraArgs`. Without one of these, Juno will fail to start.
:::

:::tip
You can use a snapshot to quickly synchronise your node with the network. Check out the [Database Snapshots](snapshots) guide to get started. Use an init container to download the snapshot before Juno starts:

```yaml
juno:
  initContainers:
    - name: download-snapshot
      image: alpine:latest
      command: ["sh", "-c", "wget -O- <SNAPSHOT_URL> | tar xf - -C /data"]
      volumeMounts:
        - name: data-juno
          mountPath: /data
```

:::

## 3. Verify the deployment

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/name=juno

# View logs
kubectl logs -f statefulset/my-juno-juno

# Test the RPC endpoint
kubectl port-forward svc/my-juno-juno 6060:6060
curl -s http://localhost:6060 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"juno_version","params":[],"id":1}'
```

## Configuration

### Network selection

Set `juno.network` to one of: `mainnet`, `sepolia`, or `sepolia-integration`.

### Ports

The chart exposes the following ports by default:

| Port | Service | Description            |
| ---- | ------- | ---------------------- |
| 6060 | RPC     | HTTP JSON-RPC endpoint |
| 6061 | WS      | WebSocket endpoint     |
| 8080 | Metrics | Prometheus metrics     |

### Extra arguments

Pass additional CLI flags to Juno using `juno.extraArgs`:

```yaml
juno:
  extraArgs:
    - --rpc-cors-enable
    - --ws
    - --ws-port=6061
    - --readiness-block-tolerance=10
```

See the [Configuring Juno](configuring) guide for the full list of options.

### Persistent storage

The chart creates a PersistentVolumeClaim for Juno's data directory. Configure the storage size and class:

```yaml
juno:
  persistence:
    size: 1Ti
    storageClassName: "gp3"
    accessModes:
      - ReadWriteOnce
```

## Exposing the RPC endpoint

### Ingress

Enable an Ingress resource to expose Juno externally. Each path defaults to the RPC port (6060). Set `servicePort` to route to a different port (e.g., 6061 for WebSocket):

```yaml
ingress:
  enabled: true
  className: "nginx"
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: rpc.example.com
      paths:
        - path: /
          pathType: Prefix
    - host: ws.example.com
      paths:
        - path: /
          pathType: Prefix
          servicePort: 6061
  tls:
    - secretName: juno-tls
      hosts:
        - rpc.example.com
        - ws.example.com
```

### Gateway API (HTTPRoute)

If your cluster uses the Gateway API instead of Ingress:

```yaml
httpRoute:
  enabled: true
  parentRefs:
    - name: gateway
      sectionName: http
  hostnames:
    - rpc.example.com
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
```

## Enabling the staking service

The chart can deploy a [staking validator](staking-validator) alongside Juno. The staking service automatically connects to the Juno node via internal service DNS.

### Using an existing secret (recommended)

First, create a Kubernetes secret with your staking configuration:

```bash
kubectl create secret generic staking-config \
  --from-file=config.json=path/to/your/config.json
```

Then enable the staking service:

```yaml
staking:
  enabled: true
  config:
    existingSecret: "staking-config"
  resources:
    requests:
      cpu: "1"
      memory: "4Gi"
```

### Using inline config (dev/test only)

For development and testing, you can provide the config inline:

```yaml
staking:
  enabled: true
  config:
    data:
      signer:
        privateKey: "0x..."
        operationalAddress: "0x..."
```

:::warning
Do not use inline config with private keys in production. Use `existingSecret` instead.
:::

## Monitoring

### ServiceMonitor

If you have the [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) installed, enable the ServiceMonitor to automatically scrape metrics from both Juno and the staking service:

```yaml
serviceMonitor:
  enabled: true
  interval: "30s"
  labels:
    release: prometheus
```

The ServiceMonitor targets the `metrics` port on each enabled service at `/metrics`. For more details on monitoring, see the [Monitoring Juno](monitoring) guide.

## Upgrading

```bash
helm repo update
helm upgrade my-juno nethermind/juno -f values.yaml
```

## Uninstalling

```bash
helm uninstall my-juno
```

:::caution
Uninstalling the chart does **not** delete the PersistentVolumeClaim. To fully remove the data, delete the PVC manually:

```bash
kubectl delete pvc -l "app.kubernetes.io/name=juno,app.kubernetes.io/instance=my-juno"
```

:::

:::tip
For the full list of configurable values, see the [chart README](https://github.com/NethermindEth/helm-charts/tree/main/charts/juno) or run:

```bash
helm show values nethermind/juno
```

:::
