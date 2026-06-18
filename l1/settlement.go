package l1

import (
	"context"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/log"
)

// watchForwarderBuffer is the per-subscription buffer between the
// contract decoder and the StateUpdate sink consumed by l1.Client.
// Shared across SettlementLayer implementations.
const watchForwarderBuffer = 64

// SettlementOption configures a SettlementLayer implementation at
// construction time. Both NewGethSettlement and NewEthSettlement accept
// the same option type since the configurable surface (currently just
// a logger) is impl-agnostic.
type SettlementOption func(*settlementOptions)

type settlementOptions struct {
	logger log.StructuredLogger
}

// WithSettlementLogger attaches a logger forwarded to the settlement.
// Surfaces transport-level warnings at debug level.
func WithSettlementLogger(l log.StructuredLogger) SettlementOption {
	return func(o *settlementOptions) { o.logger = l }
}

// SettlementLayer is the chain-neutral interface l1.Client consumes to
// follow whatever layer Starknet settles to. Today the only
// implementation lives in l1/eth/client, talking to an Ethereum
// execution-layer node; the abstraction exists so a future settlement
// backend (Bitcoin, Celestia, an alt-L1) needs only this interface,
// without dragging Ethereum types through the L1 sync loop.
//
// L1 is in the interface name today for parity with the legacy
// Subscriber it replaces; the methods themselves are chain-agnostic.
//
//go:generate mockgen -destination=../mocks/mock_settlement_layer.go -package=mocks github.com/NethermindEth/juno/l1 SettlementLayer
type SettlementLayer interface {
	ChainID(ctx context.Context) (*big.Int, error)
	FinalisedHeight(ctx context.Context) (uint64, error)
	LatestHeight(ctx context.Context) (uint64, error)
	WatchStateUpdate(ctx context.Context, sink chan<- *StateUpdate) (eth.Subscription, error)
	FilterStateUpdate(ctx context.Context, from, to uint64) ([]*StateUpdate, error)
	Close()
}

// StateUpdate is a settlement-layer-agnostic view of a Starknet
// LogStateUpdate-style event: the L2 head being committed, the
// settlement-layer position where the commit landed, and a reorg flag.
// felt.Felt conversion happens inside the settlement implementation so
// l1.Client never touches Ethereum-flavoured types.
type StateUpdate struct {
	// L2BlockNumber is the Starknet block number being committed.
	L2BlockNumber uint64
	// L2BlockHash is the Starknet block hash for that block.
	L2BlockHash *felt.Felt
	// StateRoot is the Starknet global state root after the block
	// (the "globalRoot" field in the on-chain event).
	StateRoot *felt.Felt
	// L1RefHeight is the settlement-layer block number where the
	// commit was observed. Used by the L1 sync loop to gate writes on
	// settlement-layer finality.
	L1RefHeight uint64
	// Removed is set when the settlement layer signals that the log
	// was rolled back by a reorg.
	Removed bool
}
