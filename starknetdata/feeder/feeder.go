package feeder

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknetdata"
)

var _ starknetdata.StarknetData = (*Feeder)(nil)

const (
	latestID  = "latest"
	pendingID = "pending"
)

type Feeder struct {
	client feeder.Reader
}

func New(client feeder.Reader) *Feeder {
	return &Feeder{
		client: client,
	}
}

// BlockByNumber gets the block for a given block number from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	return f.block(ctx, strconv.FormatUint(blockNumber, 10))
}

// BlockLatest gets the latest block from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockLatest(ctx context.Context) (*core.Block, error) {
	return f.block(ctx, latestID)
}

// BlockHeaderLatest gets only the block hash and number for the latest block from the feeder,
// using the headerOnly=true parameter to minimise bandwidth.
func (f *Feeder) BlockHeaderLatest(ctx context.Context) (core.Header, error) {
	response, err := f.client.BlockHeader(ctx, latestID)
	if err != nil {
		return core.Header{}, err
	}
	return core.Header{
		Hash:   response.Hash,
		Number: response.Number,
	}, nil
}

// BlockPreLatest gets the pre-latest (pending) block from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockPreLatest(ctx context.Context) (*core.Block, error) {
	return f.block(ctx, pendingID)
}

func (f *Feeder) block(ctx context.Context, blockID string) (*core.Block, error) {
	response, err := f.client.Block(ctx, blockID)
	if err != nil {
		return nil, err
	}

	if blockID == pendingID && response.Status != "PENDING" {
		return nil, errors.New("no pending block")
	}

	var signature []*felt.Felt
	if blockID != pendingID {
		sig, sErr := f.client.Signature(ctx, blockID)
		if sErr != nil {
			return nil, fmt.Errorf("get signature for block %q: %v", blockID, sErr)
		}
		signature = sig.Signature
	}

	return sn2core.AdaptBlock(response, signature)
}

// Deprecated: Transaction gets the transaction for a given transaction hash from the feeder,
// then adapts it to the appropriate core.Transaction types.
// Uses the old get_transaction endpoint; prefer get_transaction_status for status-only queries.
func (f *Feeder) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := f.client.Transaction(ctx, transactionHash)
	if err != nil {
		return nil, err
	}
	tx, err := sn2core.AdaptTransaction(response.Transaction)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// Class gets the class for a given class hash from the feeder,
// then adapts it to the core.Class type.
func (f *Feeder) Class(ctx context.Context, classHash *felt.Felt) (core.ClassDefinition, error) {
	response, err := f.client.ClassDefinition(ctx, classHash)
	if err != nil {
		return nil, err
	}

	switch {
	case response.Sierra != nil:
		casmClass, cErr := f.client.CasmClassDefinition(ctx, classHash)
		if cErr != nil && !errors.Is(cErr, feeder.ErrDeprecatedCompiledClass) {
			return nil, cErr
		}

		return sn2core.AdaptSierraClass(response.Sierra, casmClass)
	case response.DeprecatedCairo != nil:
		return sn2core.AdaptDeprecatedCairoClass(response.DeprecatedCairo)
	default:
		return nil, errors.New("empty class")
	}
}

func (f *Feeder) stateUpdate(ctx context.Context, blockID string) (*core.StateUpdate, error) {
	response, err := f.client.StateUpdate(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return sn2core.AdaptStateUpdate(response)
}

// StateUpdate gets the state update for a given block number from the feeder,
// then adapts it to the core.StateUpdate type.
func (f *Feeder) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return f.stateUpdate(ctx, strconv.FormatUint(blockNumber, 10))
}

// StateUpdatePending gets the state update for the pending block from the feeder,
// then adapts it to the core.StateUpdate type.
func (f *Feeder) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	return f.stateUpdate(ctx, pendingID)
}

func (f *Feeder) stateUpdateWithBlock(ctx context.Context, blockID string) (*core.StateUpdate, *core.Block, error) {
	var (
		stateUpBlock starknet.StateUpdateWithBlock
		signature    []*felt.Felt
	)

	if blockID == pendingID {
		resp, err := f.client.StateUpdateWithBlock(ctx, blockID)
		if err != nil {
			return nil, nil, err
		}
		stateUpBlock = *resp
	} else {
		resp, err := f.client.StateUpdateWithBlockAndSignature(ctx, blockID)
		if err != nil {
			return nil, nil, err
		}
		stateUpBlock.Block = resp.Block
		stateUpBlock.StateUpdate = resp.StateUpdate
		signature = resp.Signature
	}

	var adaptedState *core.StateUpdate
	var adaptedBlock *core.Block
	var err error

	if adaptedState, err = sn2core.AdaptStateUpdate(stateUpBlock.StateUpdate); err != nil {
		return nil, nil, err
	}

	if adaptedBlock, err = sn2core.AdaptBlock(stateUpBlock.Block, signature); err != nil {
		return nil, nil, err
	}

	return adaptedState, adaptedBlock, nil
}

// StateUpdatePendingWithBlock gets both pending state update and pending block from the feeder,
// then adapts them to the core.StateUpdate and core.Block types respectively
func (f *Feeder) StateUpdatePendingWithBlock(
	ctx context.Context,
) (*core.StateUpdate, *core.Block, error) {
	return f.stateUpdateWithBlock(ctx, pendingID)
}

// StateUpdateWithBlock gets both state update and block for a given block number from the feeder,
// then adapts them to the core.StateUpdate and core.Block types respectively
func (f *Feeder) StateUpdateWithBlock(
	ctx context.Context,
	blockNumber uint64,
) (*core.StateUpdate, *core.Block, error) {
	return f.stateUpdateWithBlock(ctx, strconv.FormatUint(blockNumber, 10))
}

// PreConfirmedBlockByNumber fetches the pre_confirmed block at the given
// height and returns a delta-aware update. knownBlockIdentifier and
// knownTransactionCount tell the server what the caller already has so the
// server can return a no-change marker, only the transactions appended since
// knownTransactionCount, or the full block when the round identifier no
// longer matches. Set both to zero values to get a full block.
func (f *Feeder) PreConfirmedBlockByNumber(
	ctx context.Context,
	blockNumber uint64,
	knownBlockIdentifier string,
	knownTransactionCount uint64,
) (pending.PreConfirmedUpdate, error) {
	response, err := f.client.PreConfirmedBlock(
		ctx,
		strconv.FormatUint(blockNumber, 10),
		knownBlockIdentifier,
		knownTransactionCount,
	)
	if err != nil {
		return pending.PreConfirmedUpdate{}, err
	}

	if !response.Changed {
		return pending.PreConfirmedUpdate{
			Mode:            pending.PreConfirmedNoChange,
			BlockIdentifier: knownBlockIdentifier,
		}, nil
	}

	// only present on full responses
	if response.SequencerAddress != nil {
		full, adaptErr := sn2core.AdaptPreConfirmedBlock(response, blockNumber)
		if adaptErr != nil {
			return pending.PreConfirmedUpdate{}, adaptErr
		}
		return pending.PreConfirmedUpdate{
			Mode:            pending.PreConfirmedFull,
			BlockIdentifier: response.KnownBlockIdentifier,
			Full:            &full,
		}, nil
	}

	// otherwise it's a delta response
	txs, receipts, stateDiffs, candidateTxs, adaptErr := sn2core.AdaptPreConfirmedDelta(response)
	if adaptErr != nil {
		return pending.PreConfirmedUpdate{}, adaptErr
	}
	return pending.PreConfirmedUpdate{
		Mode:               pending.PreConfirmedDelta,
		BlockIdentifier:    response.KnownBlockIdentifier,
		AppendTransactions: txs,
		AppendReceipts:     receipts,
		AppendStateDiffs:   stateDiffs,
		AppendCandidateTxs: candidateTxs,
	}, nil
}
