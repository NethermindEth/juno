package l1

import (
	"context"

	"github.com/NethermindEth/juno/l1/contract"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
)

type EthSubscriber struct {
	ethClient *ethclient.Client
	filterer  *contract.StarknetFilterer
}

var _ Subscriber = (*EthSubscriber)(nil)

func NewEthSubscriber(ethClientAddress string, coreContractAddress common.Address) (*EthSubscriber, error) {
	ethClient, err := ethclient.Dial(ethClientAddress)
	if err != nil {
		return nil, err
	}
	filterer, err := contract.NewStarknetFilterer(coreContractAddress, ethClient)
	if err != nil {
		return nil, err
	}
	return &EthSubscriber{
		ethClient: ethClient,
		filterer:  filterer,
	}, nil
}

func (s *EthSubscriber) WatchHeader(ctx context.Context, sink chan<- *types.Header) (event.Subscription, error) {
	return s.ethClient.SubscribeNewHead(ctx, sink)
}

func (s *EthSubscriber) WatchLogStateUpdate(ctx context.Context, sink chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error) {
	return s.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, sink)
}

func (s *EthSubscriber) Close() {
	s.ethClient.Close()
}
