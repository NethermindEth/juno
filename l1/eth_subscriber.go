package l1

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/NethermindEth/juno/l1/contract"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
		client:    ethClient.Client(),
		filterer:  filterer,
		listener:  SelectiveListener{},
	}, nil
}

//go:generate mockgen -destination=../mocks/mock_ethclient.go -package=mocks  github.com/NethermindEth/juno/l1 EthClient
type EthClient interface {
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

func NewETHClient(ethClientAddress string) (EthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// TODO replace with our own client once we have one.
	// Geth pulls in a lot of dependencies that we don't use.
	client, err := rpc.DialContext(ctx, ethClientAddress)
	if err != nil {
		return nil, err
	}
	ethClient := ethclient.NewClient(client)
	return ethClient, nil
}

func (s *EthSubscriber) WatchLogStateUpdate(ctx context.Context, sink chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error) {
	return s.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, sink)
}

func (s *EthSubscriber) ChainID(ctx context.Context) (*big.Int, error) {
	reqTimer := time.Now()
	chainID, err := s.ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get chain ID: %w", err)
	}
	s.listener.OnL1Call("eth_chainId", time.Since(reqTimer))

	return chainID, nil
}

func (s *EthSubscriber) FinalisedHeight(ctx context.Context) (uint64, error) {
	const method = "eth_getBlockByNumber"
	reqTimer := time.Now()

	var raw json.RawMessage
	if err := s.client.CallContext(ctx, &raw, method, "finalized", false); err != nil { //nolint:misspell
		return 0, fmt.Errorf("get finalised Ethereum block: %w", err)
	}
	s.listener.OnL1Call(method, time.Since(reqTimer))

	var head *types.Header
	if err := json.Unmarshal(raw, &head); err != nil {
		return 0, err
	}

	if head == nil {
		return 0, fmt.Errorf("finalised block not found")
	}

	return head.Number.Uint64(), nil
}

func (s *EthSubscriber) Close() {
	s.ethClient.Close()
}
