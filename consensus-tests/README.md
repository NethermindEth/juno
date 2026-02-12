# Juno Consensus Test

This is an experimental test setup for running 4 Juno nodes (0-3) with Tendermint consensus algorithm to observe network performance under load.

**Node topology**: Node 1 → Node 0, Node 2 → Node 1, Node 3 → Node 2

## Test Flow

1. **Node Setup**: 4 Juno nodes are initialized and connected in sequence using Docker containers
2. **Account Setup**: A setup service creates `SHOOTER_COUNT` accounts, each funded with ETH and STRK tokens
3. **Load Testing**: `SHOOTER_COUNT` shooter instances each send 1,000 transactions concurrently, load balanced across all 4 nodes

## Prerequisites

Docker and Docker Compose installed.

## Usage

Set the compose file location:
```bash
export COMPOSE_FILE=consensus-tests/docker-compose.yaml
```

Run the test:
```bash
docker compose up -d --build
```

View logs:
```bash
docker compose logs -f
```

Stop and cleanup:
```bash
docker compose down -v
```

## Configuration

- `SHOOTER_COUNT`: Number of shooter instances and funded accounts to create (default: 4)
  - Total transactions = `SHOOTER_COUNT × 1,000`
  - Adjust based on your system resources