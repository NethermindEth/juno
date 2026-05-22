package feeder

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/starknetdata"
)

const (
	verificationInterval = 30 * time.Minute
	verificationTimeout  = 2 * time.Second
)

// MigrationFeeder wraps a [Feeder] and provides a transitional implementation of
// [starknetdata.StarknetData] for networks that have not yet upgraded to support
// the new "includeSignature=true" argument for the "get_state_update" endpoint.
//
// A background verification loop ([Run]) periodically probes the new endpoint
// and atomically flips an internal flag once it becomes available, so the
// transition is automatic and requires no restart.
//
// When the upstream feeder gateway does support the new argument, [MigrationFeeder]
// delegates directly to the wrapped [Feeder]. Until then, it falls back to
// the old approach with two separate requests.
type MigrationFeeder struct {
	feeder *Feeder

	// isFeederUpdated is set to true once the upstream feeder is confirmed
	// to support StateUpdateWithBlockAndSignature.
	isFeederUpdated atomic.Bool
}

var _ starknetdata.StarknetData = (*MigrationFeeder)(nil)

func NewMigrationFeeder(feeder *Feeder) *MigrationFeeder {
	return &MigrationFeeder{
		feeder: feeder,
	}
}

// Complies with the Service interface
func (f *MigrationFeeder) Run(ctx context.Context) error {
	f.startVerificationLoop(ctx)

	// [service.Service] requires Run to block until ctx is cancelled; otherwise
	// [node.Node.StartService] would treat the early return as a crash and shut
	// down the whole node via its deferred cancel().
	<-ctx.Done()
	return nil
}

// startVerificationLoop runs verifyFeederUpdate immediately and then every
// [verificationInterval] until the new endpoint is confirmed or ctx is done.
func (f *MigrationFeeder) startVerificationLoop(ctx context.Context) {
	f.verifyFeederUpdate(ctx)
	if f.isFeederUpdated.Load() {
		return
	}

	ticker := time.NewTicker(verificationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.verifyFeederUpdate(ctx)
			if f.isFeederUpdated.Load() {
				return
			}
		}
	}
}

func (f *MigrationFeeder) verifyFeederUpdate(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, verificationTimeout)
	defer cancel()

	resp, err := f.feeder.client.StateUpdateWithBlockAndSignature(timeoutCtx, "0")
	if err == nil && resp != nil && len(resp.Signature) > 0 {
		f.isFeederUpdated.Store(true)
	}
}

// StateUpdateWithBlock returns the state update and block for the given block number.
// If the upstream feeder supports the new combined endpoint ("get_state_update" with
// the new "includeSignature=true" argument), it delegates to [Feeder.StateUpdateWithBlock].
// Otherwise, it falls back to two separate requests (the "old" way): one for the state
// update with block and another for the block signature.
func (f *MigrationFeeder) StateUpdateWithBlock(
	ctx context.Context, blockNumber uint64,
) (*core.StateUpdate, *core.Block, error) {
	if f.isFeederUpdated.Load() {
		return f.feeder.StateUpdateWithBlock(ctx, blockNumber)
	}

	response, err := f.feeder.client.StateUpdateWithBlock(ctx, strconv.FormatUint(blockNumber, 10))
	if err != nil {
		return nil, nil, err
	}
	sig, err := f.feeder.client.Signature(ctx, strconv.FormatUint(blockNumber, 10))
	if err != nil {
		return nil, nil, err
	}
	var adaptedState *core.StateUpdate
	var adaptedBlock *core.Block

	if adaptedState, err = sn2core.AdaptStateUpdate(response.StateUpdate); err != nil {
		return nil, nil, err
	}

	if adaptedBlock, err = sn2core.AdaptBlock(response.Block, sig.Signature); err != nil {
		return nil, nil, err
	}

	return adaptedState, adaptedBlock, nil
}

//////////////// Only mirroring the other feeder methods ////////////////

func (f *MigrationFeeder) BlockByNumber(
	ctx context.Context, blockNumber uint64,
) (*core.Block, error) {
	return f.feeder.BlockByNumber(ctx, blockNumber)
}

func (f *MigrationFeeder) BlockHeaderLatest(ctx context.Context) (core.Header, error) {
	return f.feeder.BlockHeaderLatest(ctx)
}

func (f *MigrationFeeder) BlockLatest(ctx context.Context) (*core.Block, error) {
	return f.feeder.BlockLatest(ctx)
}

func (f *MigrationFeeder) BlockPreLatest(ctx context.Context) (*core.Block, error) {
	return f.feeder.BlockPreLatest(ctx)
}

func (f *MigrationFeeder) Class(
	ctx context.Context, classHash *felt.Felt,
) (core.ClassDefinition, error) {
	return f.feeder.Class(ctx, classHash)
}

func (f *MigrationFeeder) PreConfirmedBlockByNumber(
	ctx context.Context, blockNumber uint64,
) (pending.PreConfirmed, error) {
	return f.feeder.PreConfirmedBlockByNumber(ctx, blockNumber)
}

func (f *MigrationFeeder) StateUpdate(
	ctx context.Context, blockNumber uint64,
) (*core.StateUpdate, error) {
	return f.feeder.StateUpdate(ctx, blockNumber)
}

func (f *MigrationFeeder) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	return f.feeder.StateUpdatePending(ctx)
}

func (f *MigrationFeeder) StateUpdatePendingWithBlock(
	ctx context.Context,
) (*core.StateUpdate, *core.Block, error) {
	return f.feeder.StateUpdatePendingWithBlock(ctx)
}

func (f *MigrationFeeder) Transaction(
	ctx context.Context, transactionHash *felt.Felt,
) (core.Transaction, error) {
	return f.feeder.Transaction(ctx, transactionHash)
}
