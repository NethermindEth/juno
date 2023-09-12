---
slug: /
sidebar_position: 1
title: Quick Start
---

*Juno is your fast and featureful Starknet client implementation.*

Suitable for casual setups, production-grade indexers, and everything in between.

- :floppy_disk: **Tiny database size**: ~30Gb on mainnet
- :zap: **Blazing fast sync**: constrained only by hardware and the sequencer
- :100: **100% [JSON-RPC spec](https://github.com/starkware-libs/starknet-specs/tree/master) compliance**: all things Starknet, in one place
- :racing_car: **Minimal RPC response latency**: to keep your applications moving
- :mag_right: **Low-level GRPC database API**: for the most demanding workloads

# Sync Starknet in Two Commands

```shell
# Juno's database directory. Can be any directory on the machine.
mkdir -p junodb

# Juno's HTTP server listens on port 6060.
docker run -d --name juno -p 6060:6060 -v junodb:/var/lib/juno nethermind/juno:latest --db-path /var/lib/juno --http
```

For a complete list of options and their explanations, see the [Example Configuration](config) or run:

```shell
docker run nethermind/juno --help
```

# Looking for a Starknet RPC Provider? 

If you are looking for a Starknet RPC provider, Nethermind will offer a Starknet RPC service before the upcoming feeder gateway deprecation. You can register your interest on [this Google Form](https://docs.google.com/forms/d/e/1FAIpQLSf2Bl4fc9-38E-fpWf0tnMWc3jSeOFkpjSPMN_j1en1WmEgKg/viewform?usp=sf_link).

# Questions, Discussions, Community

Find active Juno team members and users in the following places.

- [GitHub](https://github.com/NethermindEth/juno)
- [Discord](https://discord.gg/SZkKcmmChJ)
- [Telegram](https://t.me/+LHRF4H8iQ3c5MDY0)
