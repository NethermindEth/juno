# juno

![Juno Logo](./.github/juno.jpg?raw=true)

[![Go Report Card](https://goreportcard.com/badge/github.com/NethermindEth/juno)](https://goreportcard.com/report/github.com/NethermindEth/juno)

[![Actions Status](https://github.com/NethermindEth/juno/actions/workflows/juno-build.yml/badge.svg)](https://github.com/NethermindEth/juno/actions)

[![codecov](https://codecov.io/gh/NethermindEth/juno/branch/main/graph/badge.svg)](https://codecov.io/gh/NethermindEth/juno)

Juno is a StarkNet client written in Go.

[Docs](https://nethermindeth.github.io/juno/)

Here you will find various links to help you start with the StarkNet ecosystem:

- [StarkNet Docs](https://starknet.io/)
- [Voyager block explorer](https://voyager.online)
- [Warp Docs](https://github.com/NethermindEth/warp)
- [CairoLang Docs](https://www.cairo-lang.org/)
- [StarkEx Docs](https://docs.starkware.co/starkex-v4)
- [StarkNet Devs Discord](https://discord.com/invite/uJ9HZTUk2Y)
- [Starknet 101](https://github.com/l-henri/starknet-cairo-101)
- [StarkNet Shamans Forum](https://community.starknet.io/)
- [StarkNet Medium](https://medium.com/starkware/starknet/home)
- [StarkNet Twitter](https://twitter.com/Starknet_Intern)
- [Nethermind Twitter](https://twitter.com/NethermindEth)

## Logging

For logging we use [zap](https://github.com/uber-go/zap). This library has 6 levels of logging: Debug, Info, Warning,
Error, Fatal, and Panic. For example:

```go
package main

import "github.com/NethermindEth/juno/internal/log"

var logger = log.GetLogger()

func main() {
	// Set of levels
	logger.Debug("Useful debugging information.")
	logger.Info("Something noteworthy happened!")
	logger.Warn("You should probably take a look at this.")
	logger.Error("Something failed but I'm not quitting.")
	logger.Fatal("Bye.")
	logger.Panic("I'm bailing.")
}
```

Use `import log "github.com/sirupsen/logrus"` instead of `import "log"`.

It also allows us to add fields to the outputs, like this:

```go
  logger.With("Key0", "Value0").Debugw("Useful debugging information.")
  logger.Infow("Useful information.", "Key0", "Value0", "Key1", "1")
```

Resulting in an output like this:

![Zap](./docs/static/img/log.png)

For more details about logging, see [zap](https://github.com/uber-go/zap).

## Configuration

For configuration and cli, we use [Viper](https://github.com/spf13/viper) and [Cobra](https://github.com/spf13/cobra)
respectively.

### Configuration File

An example of a config file can be:

```yaml
rpc:
  enabled: true
  port: 8080
db_path: $HOME/.juno/data
```

The config file in case it didn't exist, is generated, and we read it using Viper. We will add more configurations in
the future.

### CLI

Available CLI commands are:

```
$ juno -h
Juno, StarkNet Client in Go

Usage:
  juno [flags]

Flags:
      --config string   config file (default is $HOME/.juno/config.yaml)
  -h, --help            help for juno

```
