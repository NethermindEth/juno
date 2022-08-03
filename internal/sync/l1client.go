package sync

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	common "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type L1Client interface {
	ChainID(ctx context.Context) (*big.Int, error)
	BlockNumber(ctx context.Context) (uint64, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]ethtypes.Log, error)
	SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- ethtypes.Log) (ethereum.Subscription, error)
	TransactionByHash(ctx context.Context, hash common.Hash) (tx *ethtypes.Transaction, isPending bool, err error)
}
