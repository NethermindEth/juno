---
title: Plugins
---

Juno supports plugins that satisfy the `JunoPlugin` interface, enabling developers to extend and customize Juno's behaviour and functionality by dynamically loading external plugins during runtime.

The `JunoPlugin` interface provides a structured way for plugins to interact with the blockchain by sending notifications when new blocks are added or reverted. This ensures state consistency, especially during blockchain reorganizations, while abstracting away the complexity of implementing block syncing and revert logic.

## JunoPlugin Interface

Your plugin must implement the `JunoPlugin` interface, which includes methods for initializing, shutting down, and handling new and reverted blocks.

```go
type JunoPlugin interface {
	Init() error
	Shutdown() error
	NewBlock(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error
	RevertBlock(from, to *BlockAndStateUpdate, reverseStateDiff *core.StateDiff) error
}
```

**Init**: Called when the plugin is initialized. This can be used to set up database connections or any other necessary resources.

**Shutdown**: Called when the Juno node is shut down. This can be used to clean up resources like database connections.

**NewBlock**: Triggered when a new block is synced by the Juno client. Juno will send the block, the corresponding state update, and any new classes. Importantly, Juno waits for the plugin to finish processing this function call before continuing. This ensures that the plugin completes its task before Juno proceeds with the blockchain sync.

**RevertBlock**: Called during a blockchain reorganization (reorg). Juno will invoke this method for each block that needs to be reverted. Similar to NewBlock, the client will wait for the plugin to finish handling the revert before moving on to the next block.

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

func (p *examplePlugin) NewBlock(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error {
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

This command compiles the plugin into a shared object file (`plugin.so`), which can then be loaded by the Juno client using the `--plugin-path` flag.

## Running Juno with the plugin

Once your plugin has been compiled into a `.so` file, you can run Juno with your plugin by providing the `--plugin-path` flag. This flag tells Juno where to find and load your plugin at runtime.
