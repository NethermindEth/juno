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
	}, nil
}

func (s *EthSubscriber) WatchLogStateUpdate(ctx context.Context, sink chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error) {
	return s.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, sink)
}

func (s *EthSubscriber) ChainID(ctx context.Context) (*big.Int, error) {
	return s.ethClient.ChainID(ctx)
}

func (s *EthSubscriber) FinalisedHeight(ctx context.Context) (uint64, error) {
	finalisedBlock := make(map[string]any, 0)
	if err := s.client.CallContext(ctx, &finalisedBlock, "eth_getBlockByNumber", "finalized", false); err != nil { //nolint:misspell
		return 0, fmt.Errorf("get finalised Ethereum block: %w", err)
	}

	//nolint:gosec
	number, ok := finalisedBlock["number"]
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
