package l1

import (
	"context"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
)

// SettlementLayer is the interface l1.Client uses to follow the Ethereum
// L1 chain Starknet settles to: reading heights, watching and replaying
// LogStateUpdate events, and checking the chain id. GethSettlement is the
// implementation today; a hand-rolled Ethereum client (to drop the
// go-ethereum dependency) is intended to slot in behind this same
// interface, which is why l1.Client depends on the interface rather than a
// concrete client.
//
// Decoded events cross this boundary as StateUpdate, in juno's own types,
// so no go-ethereum type — nor any implementation's internal types — leaks
// into the L1 sync loop.
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

// StateUpdate is the decoded form of the Starknet core contract's
// LogStateUpdate event in juno's own types: the L2 head being committed,
// the L1 block where the commit landed, and a reorg flag. The
// SettlementLayer implementation does the felt.Felt conversion, so
// l1.Client never touches go-ethereum types.
type StateUpdate struct {
	// L2BlockNumber is the Starknet block number being committed.
	L2BlockNumber uint64
	// L2BlockHash is the Starknet block hash for that block.
	L2BlockHash *felt.Felt
	// StateRoot is the Starknet global state root after the block
	// (the "globalRoot" field in the on-chain event).
	StateRoot *felt.Felt
	// L1RefHeight is the L1 block number where the commit was observed.
	// Used by the L1 sync loop to gate writes on L1 finality.
	L1RefHeight uint64
	// Removed is set when L1 signals that the log was rolled back by a
	// reorg.
	Removed bool
}
