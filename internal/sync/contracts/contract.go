package contracts

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Contract interface {
	FilterLogs(ctx context.Context, start, end uint64, name string) (LogIterator, error)
	SubscribeFilterLogs(ctx context.Context, name string) (LogIterator, error)
	ParseLog(out any, log types.Log, name string) error
	DeploymentBlock() uint64
	Abi() abi.ABI
}

type contract struct {
	filterer        IteratorFilterer
	address         common.Address
	deploymentBlock uint64
	abi             abi.ABI
}

var _ Contract = &contract{}

func NewContract(address common.Address, deploymentBlock uint64, abiString string, filterer ethereum.LogFilterer) (*contract, error) {
	parsed, err := parseAbi(abiString)
	if err != nil {
		return nil, err
	}
	return &contract{
		filterer:        newIteratorFilterer(filterer),
		address:         address,
		deploymentBlock: deploymentBlock,
		abi:             parsed,
	}, nil
}

func (c *contract) FilterLogs(ctx context.Context, start, end uint64, name string) (LogIterator, error) {
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(start),
		ToBlock:   new(big.Int).SetUint64(end),
		Addresses: []common.Address{c.address},
		Topics:    [][]common.Hash{{c.abi.Events[string(name)].ID}},
	}
	return c.filterer.FilterLogs(ctx, query)
}

func (c *contract) DeploymentBlock() uint64 {
	return c.deploymentBlock
}

func (c *contract) ParseLog(out any, log types.Log, name string) error {
	if log.Topics[0] != c.abi.Events[name].ID {
		return fmt.Errorf("event signature mismatch")
	}
	if len(log.Data) > 0 {
		if err := c.abi.UnpackIntoInterface(out, name, log.Data); err != nil {
			return err
		}
	}
	var indexed abi.Arguments
	for _, arg := range c.abi.Events[name].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return abi.ParseTopics(out, indexed, log.Topics[1:])
}

func (c *contract) SubscribeFilterLogs(ctx context.Context, name string) (LogIterator, error) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{c.address},
		Topics:    [][]common.Hash{{c.abi.Events[string(name)].ID}},
	}
	return c.filterer.SubscribeFilterLogs(ctx, query)
}

func (c *contract) Abi() abi.ABI {
	return c.abi
}

func parseAbi(abiString string) (abi.ABI, error) {
	return abi.JSON(strings.NewReader(abiString))
}
