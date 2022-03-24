# juno

<div align="center"><img width="128" src="./docs/static/img/juno_rounded.png"></div>

Starknet client implementation.

[![Go Reference](https://pkg.go.dev/badge/github.com/NethermindEth/juno.svg)](https://pkg.go.dev/github.com/NethermindEth/juno) [![Go Report Card](https://goreportcard.com/badge/github.com/NethermindEth/juno)](https://goreportcard.com/report/github.com/NethermindEth/juno) [![Actions Status](https://github.com/NethermindEth/juno/actions/workflows/juno-build.yml/badge.svg)](https://github.com/NethermindEth/juno/actions) [![codecov](https://codecov.io/gh/NethermindEth/juno/branch/main/graph/badge.svg)](https://codecov.io/gh/NethermindEth/juno)

## Building from source

Run the following command.

```sh
% make all
```

## Executables

<table>
  <tr><th>Command</th><th>Description</th></tr>
  <tr>
    <td><code>juno</code></td>
    <td>The StarkNet full node client.</td>
  <tr>
</table>

## Configuration

**juno** uses a configuration file named **config.yaml** that is located On Darwin in 
`$HOME/Library/Application Support/juno/`, on other Unix systems `$XDG_CONFIG_HOME/juno/`, and on Windows,
`%APPDATA%/juno`. It generally looks like the following and a default will be generated if one does not exist. 

```yaml
rpc:
  enabled: false
  port: 8080
db_path: (See above).
starknet_network: http://alpha4.starknet.io
```
