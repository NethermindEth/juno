---
slug: /http
sidebar_position: 2
---

# HTTP API

By default, Juno exposes an HTTP API on `http://localhost:6060` with the following routes:

- `/`: a `GET` request will return an empty response with status 200. A `POST` message with a JSON-RPC request will respond according to the [JSON-RPC spec](https://github.com/starkware-libs/starknet-specs/tree/master).
- `/metrics`: if enabled with `--metrics`, the Prometheus metrics server.
- `/debug/pprof/`: if enabled with `--pprof`, serves metrics from the Go runtime. 
  If profiles are generated using the sub-routes, Juno's performance may degrade. See [here](https://www.sobyte.net/post/2022-06/go-pprof/) for a tutorial.
- `/grpc`: a GRPC API for directly querying Juno's database. Useful for demanding applications; otherwise, the standard JSON-RPC API is recommended.
  Users of this feature should note that Juno's database layout can change between patch versions and plan accordingly.
