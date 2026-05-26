package feeder

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/starknetdata"
)

const (
	verificationInterval = 30 * time.Minute
	verificationTimeout  = 2 * time.Second
)

// FeederAdapter wraps a [Feeder] and provides a transitional implementation of
// [starknetdata.StarknetData] for networks that have not yet upgraded to support
// the new "includeSignature=true" argument for the "get_state_update" endpoint.
//
// A background verification loop ([Run]) periodically probes the new endpoint
// and atomically flips an internal flag once it becomes available, so the
// transition is automatic and requires no restart.
//
// When the upstream feeder gateway does support the new argument, [FeederAdapter]
// delegates directly to the wrapped [Feeder]. Until then, it falls back to
// the old approach with two separate requests.
type FeederAdapter struct {
	*Feeder

	// isFeederUpdated is set to true once the upstream feeder is confirmed
	// to support StateUpdateWithBlockAndSignature.
	isFeederUpdated atomic.Bool
}

var _ starknetdata.StarknetData = (*FeederAdapter)(nil)

func NewFeederAdaper(feeder *Feeder) *FeederAdapter {
	return &FeederAdapter{
		Feeder: feeder,
	}
}

// Complies with the Service interface
func (f *FeederAdapter) Run(ctx context.Context) error {
	if isFeederUpdated := f.runVerificationLoop(ctx); isFeederUpdated {
		f.isFeederUpdated.Store(true)
	}

	// [service.Service] requires Run to block until ctx is cancelled; otherwise
	// [node.Node.StartService] would treat the early return as a crash and shut
	// down the whole node via its deferred cancel().
	<-ctx.Done()
	return nil
}

// runVerificationLoop runs verifyFeederUpdate immediately and then every
// [verificationInterval] until the new endpoint is confirmed or ctx is done.
// It only returns if the feeder has been updated (true) or ctx is done (false).
func (f *FeederAdapter) runVerificationLoop(ctx context.Context) bool {
	if isFeederUpdated := f.verifyFeederUpdate(ctx); isFeederUpdated {
		return true
	}

	ticker := time.NewTicker(verificationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if isFeederUpdated := f.verifyFeederUpdate(ctx); isFeederUpdated {
				return true
			}
		}
	}
}

func (f *FeederAdapter) verifyFeederUpdate(ctx context.Context) bool {
	timeoutCtx, cancel := context.WithTimeout(ctx, verificationTimeout)
	defer cancel()

	_, err := f.Feeder.client.StateUpdateWithBlockAndSignature(timeoutCtx, latestID)
	return err == nil
}

// StateUpdateWithBlock returns the state update and block for the given block number.
// If the upstream feeder supports the new combined endpoint ("get_state_update" with
// the new "includeSignature=true" argument), it delegates to [Feeder.StateUpdateWithBlock].
// Otherwise, it falls back to two separate requests (the "old" way): one for the state
// update with block and another for the block signature.
func (f *FeederAdapter) StateUpdateWithBlock(
	ctx context.Context, blockNumber uint64,
) (*core.StateUpdate, *core.Block, error) {
	if f.isFeederUpdated.Load() {
		return f.Feeder.StateUpdateWithBlock(ctx, blockNumber)
	}

	response, err := f.Feeder.client.StateUpdateWithBlock(ctx, strconv.FormatUint(blockNumber, 10))
	if err != nil {
		return nil, nil, err
	}
	sig, err := f.Feeder.client.Signature(ctx, strconv.FormatUint(blockNumber, 10))
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
