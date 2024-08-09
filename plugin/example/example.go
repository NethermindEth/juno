package main

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	junoplugin "github.com/NethermindEth/juno/plugin"
)

//go:generate go build -buildmode=plugin -o ../../build/plugin.so ./example.go
type examplePlugin string

// Important: "JunoPluginInstance" needs to be exported for Juno to load the plugin correctly
var JunoPluginInstance examplePlugin

var _ junoplugin.JunoPlugin = (*examplePlugin)(nil)

func (p *examplePlugin) Init() error {
	fmt.Println("ExamplePlugin initialised")
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
