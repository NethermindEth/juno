// Package adaptfeeder adapts feeder gateway responses to core types.
package adaptfeeder

import (
	"context"
	"errors"
	"strconv"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
)

const latestID = "latest"

type Feeder struct {
	client feeder.Reader
}

func New(client feeder.Reader) *Feeder {
	return &Feeder{
		client: client,
	}
}

// BlockHeaderLatest gets only the block hash and number for the latest block from the feeder,
// using the headerOnly=true parameter to minimise bandwidth.
func (f *Feeder) BlockHeaderLatest(ctx context.Context) (core.Header, error) {
	response, err := f.client.BlockHeader(ctx, latestID)
	if err != nil {
		return core.Header{}, err
	}
	//nolint:exhaustruct_v5 // the header-only endpoint returns only the hash and number
	return core.Header{
		Hash:   response.Hash,
		Number: response.Number,
	}, nil
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

		// A deprecated compiled class yields no CASM; pass nil so the adapter
		// treats the Sierra class as having no compiled counterpart.
		var compiledClass *starknet.CasmClass
		if cErr == nil {
			compiledClass = &casmClass
		}

		return sn2core.AdaptSierraClass(response.Sierra, compiledClass)
	case response.DeprecatedCairo != nil:
		return sn2core.AdaptDeprecatedCairoClass(response.DeprecatedCairo)
	default:
		return nil, errors.New("empty class")
	}
}

func (f *Feeder) stateUpdateWithBlock(ctx context.Context, blockID string) (*core.StateUpdate, *core.Block, error) {
	resp, err := f.client.StateUpdateWithBlockAndSignature(ctx, blockID)
	if err != nil {
		return nil, nil, err
	}

	adaptedState, err := sn2core.AdaptStateUpdate(resp.StateUpdate)
	if err != nil {
		return nil, nil, err
	}

	adaptedBlock, err := sn2core.AdaptBlock(resp.Block, resp.Signature)
	if err != nil {
		return nil, nil, err
	}

	return adaptedState, adaptedBlock, nil
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
// height and returns a delta-aware update. blockIdentifier and
// knownTransactionCount tell the server what the caller already has so the
// server can return a no-change marker, only the transactions appended since
// knownTransactionCount, or the full block when the round identifier no
// longer matches. Set both to zero values to get a full block.
func (f *Feeder) PreConfirmedBlockByNumber(
	ctx context.Context,
	blockNumber uint64,
	blockIdentifier string,
	knownTransactionCount uint64,
) (starknet.PreConfirmedUpdate, error) {
	return f.client.PreConfirmedBlockWithIdentifier(
		ctx,
		strconv.FormatUint(blockNumber, 10),
		blockIdentifier,
		knownTransactionCount,
	)
}

// PreConfirmedBlockLatest fetches whichever pre_confirmed block the sequencer is
// currently exposing as latest.
// The returned block number is the height the response describes.
func (f *Feeder) PreConfirmedBlockLatest(
	ctx context.Context,
	blockIdentifier string,
	knownTransactionCount uint64,
) (starknet.PreConfirmedUpdate, uint64, error) {
	return f.client.PreConfirmedBlockLatest(ctx, blockIdentifier, knownTransactionCount)
}
