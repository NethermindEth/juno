package juno

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/internal/config"
)

type StarkNetNode interface {
	Run() error
	Shutdown() error
}

type NewStarkNetNodeFn func(cfg *config.Juno) (StarkNetNode, error)

type Node struct{}

func New(cfg *config.Juno) (StarkNetNode, error) {
	j, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return nil, err
	}
	fmt.Printf("Juno Config: \n%s\n", j)
	return nil, nil
}

func (n *Node) Run() error {
	return nil
}

func (n *Node) Shutdown() error {
	return nil
}
