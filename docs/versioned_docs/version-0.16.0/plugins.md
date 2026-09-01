---
title: Plugins
---

Juno supports plugins that satisfy the `JunoPlugin` interface, enabling developers to extend and customize Juno's behaviour and functionality by dynamically loading external plugins during runtime.

The `JunoPlugin` interface provides a structured way for plugins to interact with the blockchain by sending notifications when new blocks are added or reverted. This abstracts away the complexity of implementing block syncing and revert logic, especially during blockchain reorganizations.

## JunoPlugin Interface

Your plugin must implement the `JunoPlugin` interface, which includes methods for initializing, shutting down, and handling new and reverted blocks.

```go
type JunoPlugin interface {
	Init() error
	Shutdown() error
	NewBlock(
		block *core.Block,
		stateUpdate *core.StateUpdate,
		newClasses map[felt.Felt]core.ClassDefinition,
	) error
	// The state is reverted by applying a write operation with the reverseStateDiff's StorageDiffs, Nonces, and ReplacedClasses,
	// and a delete option with its DeclaredV0Classes, DeclaredV1Classes, and ReplacedClasses.
	RevertBlock(from, to *BlockAndStateUpdate, reverseStateDiff *core.StateDiff) error
}

type BlockAndStateUpdate struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
}
```

**Init**: Called once at node startup, when Juno loads the plugin (before syncing begins). This can be used to set up database connections or any other necessary resources. If `Init` returns an error, Juno fails to start.

**Shutdown**: Called when the Juno node is shut down. This can be used to clean up resources like database connections.

**NewBlock**: Called by the synchronizer each time a new block is stored. Juno sends the block, the corresponding state update, and any new classes (as `core.ClassDefinition` values), and waits for the call to return before continuing the sync. Note that a returned error is logged but does not stop the sync. The plugin is responsible for handling its own failures.

**RevertBlock**: Called during a blockchain reorganization (reorg), once for each block that needs to be reverted. `reverseStateDiff` describes how to undo the block's state changes: apply its `StorageDiffs`, `Nonces`, and `ReplacedClasses` as writes, and its `DeclaredV0Classes`, `DeclaredV1Classes`, and `ReplacedClasses` as deletions. As with `NewBlock`, Juno waits for the call to return, and errors are logged without stopping the sync.

## Example plugin

Here is a basic example of a plugin that satisfies the `JunoPlugin` interface:

```go
// go:generate go build -buildmode=plugin -o ../../build/plugin.so ./example.go
type examplePlugin string

// Important: "JunoPluginInstance" needs to be exported for Juno to load the plugin correctly
var JunoPluginInstance examplePlugin

var _ junoplugin.JunoPlugin = (*examplePlugin)(nil)

func (p *examplePlugin) Init() error {
	fmt.Println("ExamplePlugin initialized")
	return nil
}

func (p *examplePlugin) Shutdown() error {
	fmt.Println("ExamplePlugin shutdown")
	return nil
}

func (p *examplePlugin) NewBlock(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.ClassDefinition) error {
	fmt.Println("ExamplePlugin NewBlock called")
	return nil
}

func (p *examplePlugin) RevertBlock(from, to *junoplugin.BlockAndStateUpdate, reverseStateDiff *core.StateDiff) error {
	fmt.Println("ExamplePlugin RevertBlock called")
	return nil
}
```

The `JunoPluginInstance` variable must be exported for Juno to correctly load the plugin:
`var JunoPluginInstance examplePlugin`

We ensure the plugin implements the `JunoPlugin` interface, with the following line:
`var _ junoplugin.JunoPlugin = (*examplePlugin)(nil)`

## Building and loading the plugin

Once you have written your plugin, you can compile it into a shared object file (.so) using the following command:

```shell
go build -buildmode=plugin -o ./plugin.so /path/to/your/plugin.go
```

This command compiles the plugin into a shared object file (`plugin.so`), which can then be loaded by the Juno client.

Go plugins impose strict compatibility requirements. If any of them are not met, Juno will fail to load the plugin at runtime:

- The plugin must be built with the **same Go toolchain version** used to build the Juno binary.
- The plugin must depend on the **same version of the `github.com/NethermindEth/juno` module** (and identical versions of any shared dependencies) as the running node.
- The plugin must be built with **CGO enabled** (`CGO_ENABLED=1`).
- Go plugins are only supported on **Linux and macOS**.

## Running Juno with the plugin

Once your plugin has been compiled into a `.so` file, run Juno with the `--plugin-path` flag pointing at the file:

```shell
./build/juno --plugin-path ./plugin.so
```

The plugin path can also be set via the `JUNO_PLUGIN_PATH` environment variable or the `plugin-path` key in a YAML configuration file.
