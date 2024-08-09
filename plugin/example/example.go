package main

import (
	"fmt"
)

//go:generate go build -buildmode=plugin -o ./example.so ./example.go
type ExamplePlugin struct{}

func (p *ExamplePlugin) Init() error {
	fmt.Println("ExamplePlugin initialized")
	return nil
}

func (p *ExamplePlugin) Shutdown() error {
	fmt.Println("ExamplePlugin shutdown")
	return nil
}

func (p *ExamplePlugin) NewBlock(block, stateUpdate, newClasses interface{}) error {
	fmt.Println("ExamplePlugin NewBlock called")
	return nil
}

func (p *ExamplePlugin) RevertBlock(block, stateUpdate, newClasses interface{}) error {
	fmt.Println("ExamplePlugin RevertBlock called")
	return nil
}
