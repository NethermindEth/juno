---
title: Juno Sequencer
---
Juno can now operate as a **standalone sequencer**. When Juno is run in this experimental mode, users are able to submit transactions to the Juno client, which stores them in a mempool. Every _N_ seconds, Juno will attempt to build a new block using any transactions that are present.

Sequencer mode is activated via a set of dedicated command-line flags:

- `--seq-enable`: Enables sequencer mode of operation  
- `--seq-block-time`: Time interval (in seconds) between block production attempts  
- `--seq-genesis-file`: Optional path to a genesis file for initializing the state (useful for pre-funded accounts). An example genesis file is provided at `./genesis/genesis_prefund_accounts.json`. Users can customize this file to include any accounts, balances, or deployed contracts needed for their setup.
- `--seq-disable-fees`: When set, disables transaction fee enforcement in the sequencer (useful for testing)

This makes it ideal for testing rollup flows, simulating block production, or running isolated environments without requiring Layer 1 verification.

## Features

- Accepts transactions via standard RPC calls  
- Stores transactions in a local mempool  
- Automatically builds blocks at a fixed interval (`--seq-block-time`)  
- Optionally starts with a blank or pre-funded state  

## Running the Sequencer

You can run the Juno sequencer in two modes:

### 1. Blank State (Clean Genesis)

```bash
./build/juno \
  --http \
  --http-port=6060 \
  --http-host=0.0.0.0 \
  --db-path=./seq-db-tmp \
  --log-level=debug \
  --seq-enable \
  --seq-block-time=1 \
  --network sequencer \
  --disable-l1-verification \
  --rpc-call-max-steps=4123000
```

### 2. With Pre-Funded Accounts
```bash
./build/juno \
  --http \
  --http-port=6060 \
  --http-host=0.0.0.0 \
  --db-path=./seq-db-tmp-w-accounts \
  --log-level=debug \
  --seq-enable \
  --seq-block-time=1 \
  --network sequencer \
  --disable-l1-verification \
  --seq-genesis-file ./genesis/genesis_prefund_accounts.json \
  --rpc-call-max-steps=4123000
```