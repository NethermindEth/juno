// TODO(remove once migrated from geth): once l1.Subscriber.TransactionReceipt returns
// *eth.Receipt directly, the adapter and this whole file go away.
package rpccore

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/l1/eth"
	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

// gethTxReceiptFetcher matches the subset of l1.Subscriber that the
// RPC layer cares about. Declared structurally to avoid pulling the
// whole l1 package into the rpccore dependency graph.
type gethTxReceiptFetcher interface {
	TransactionReceipt(ctx context.Context, txHash gethCommon.Hash) (*gethTypes.Receipt, error)
}

// EthReceiptAdapter wraps an l1.Subscriber (returns *geth/types.Receipt)
// and exposes the L1Client interface (returns *eth.Receipt). It exists
// only because the L1 subscriber interface still speaks geth types.
type EthReceiptAdapter struct {
	Sub gethTxReceiptFetcher
}

// Statically assert the adapter satisfies L1Client.
var _ L1Client = (*EthReceiptAdapter)(nil)

func (a *EthReceiptAdapter) TransactionReceipt(
	ctx context.Context, txHash eth.Hash,
) (*eth.Receipt, error) {
	r, err := a.Sub.TransactionReceipt(ctx, gethCommon.Hash(txHash))
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("transaction receipt not found")
	}
	out := &eth.Receipt{Logs: make([]eth.Log, 0, len(r.Logs))}
	for _, gl := range r.Logs {
		if gl == nil {
			continue
		}
		topics := make([]eth.Hash, len(gl.Topics))
		for i, t := range gl.Topics {
			topics[i] = eth.Hash(t)
		}
		out.Logs = append(out.Logs, eth.Log{
			Topics:      topics,
			Data:        eth.DataBytes(gl.Data),
			BlockNumber: eth.HexU64(gl.BlockNumber),
			Removed:     gl.Removed,
		})
	}
	return out, nil
}
