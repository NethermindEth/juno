# juno

<div align="center"><img width="128" src="./docs/static/img/juno_rounded.png"></div>

StarkNet client implementation.

[![Go Reference](https://pkg.go.dev/badge/github.com/NethermindEth/juno.svg)](https://pkg.go.dev/github.com/NethermindEth/juno) [![Go Report Card](https://goreportcard.com/badge/github.com/NethermindEth/juno)](https://goreportcard.com/report/github.com/NethermindEth/juno) [![Actions Status](https://github.com/NethermindEth/juno/actions/workflows/juno-build.yml/badge.svg)](https://github.com/NethermindEth/juno/actions) [![codecov](https://codecov.io/gh/NethermindEth/juno/branch/main/graph/badge.svg)](https://codecov.io/gh/NethermindEth/juno)

[![Discord](https://img.shields.io/badge/Discord-5865F2?style=for-the-badge&logo=discord&logoColor=white)](https://discord.gg/TcHbSZ9ATd)
[![Twitter](https://img.shields.io/badge/Twitter-1DA1F2?style=for-the-badge&logo=twitter&logoColor=white)](https://twitter.com/nethermindeth?s=20&t=xLC_xrid_f17DJqdJ2EZnA)

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

**juno** uses a configuration file named **juno.yaml** that is located in the following places depending on the operating system.

- **macOS** - `$HOME/Library/Application Support/juno/`.
- Other **Unix** systems - `$XDG_CONFIG_HOME/juno/` or `$HOME/.config/juno/` if the `$XDG_CONFIG_HOME` variable is not set.
- **Windows** - `%AppData%/juno/`.

It generally looks like the following and a default will be generated if one does not exist.

The following is an example on how it would look on a macOS system (replace `$HOME` with a full path to the home directory).

```yaml
db_path: $HOME/Library/Application Support/juno
ethereum:
  enabled: true
  node: "ethereum_archive_node"
rpc:
  enabled: true
  port: 8080
starknet:
  enabled: true
  feeder_gateway: "https://alpha-mainnet.starknet.io"
```
