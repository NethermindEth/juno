package l1

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/NethermindEth/juno/l1/contract"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

type EthSubscriber struct {
	ethClient *ethclient.Client
	client    *rpc.Client
	filterer  *contract.StarknetFilterer
	listener  EventListener
}

var _ Subscriber = (*EthSubscriber)(nil)

func NewEthSubscriber(ethClientAddress string, coreContractAddress common.Address) (*EthSubscriber, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// TODO replace with our own client once we have one.
	// Geth pulls in a lot of dependencies that we don't use.
	client, err := rpc.DialContext(ctx, ethClientAddress)
	if err != nil {
		return nil, err
	}
	ethClient := ethclient.NewClient(client)
	filterer, err := contract.NewStarknetFilterer(coreContractAddress, ethClient)
	if err != nil {
		return nil, err
	}
	return &EthSubscriber{
		ethClient: ethClient,
		client:    client,
		filterer:  filterer,
		listener:  SelectiveListener{},
	}, nil
}

func (s *EthSubscriber) WatchLogStateUpdate(ctx context.Context, sink chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error) {
	return s.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, sink)
}

func (s *EthSubscriber) ChainID(ctx context.Context) (*big.Int, error) {
	reqTimer := time.Now()
	chainID, err := s.ethClient.ChainID(ctx)
	s.listener.OnL1Call("eth_chainId", time.Since(reqTimer))
	if err != nil {
		return nil, fmt.Errorf("get chain ID: %w", err)
	}
	return chainID, nil
}

func (s *EthSubscriber) FinalisedHeight(ctx context.Context) (uint64, error) {
	finalisedBlock := make(map[string]any, 0)
	reqTimer := time.Now()
	method := "eth_getBlockByNumber"
	err := s.client.CallContext(ctx, &finalisedBlock, method, "finalized", false)
	s.listener.OnL1Call(method, time.Since(reqTimer))
	if err != nil { //nolint:misspell
		return 0, fmt.Errorf("get finalised Ethereum block: %w", err)
	}

	number, ok := finalisedBlock["number"] //nolint:gosec
	if !ok {
		return 0, fmt.Errorf("number field not present in Ethereum block")
	}

	numberString, ok := number.(string)
	if !ok {
		return 0, fmt.Errorf("block number is not a string: %v", number)
	}

	numberString = strings.TrimPrefix(numberString, "0x")
	numberUint, err := strconv.ParseUint(numberString, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parse block number: %s", numberString)
	}

	return numberUint, nil
}

func (s *EthSubscriber) Close() {
	s.ethClient.Close()
}
