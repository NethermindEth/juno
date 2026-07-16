package preconfirmed

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// DataSource is the narrow surface the Poller needs from the wire side. Any
// type implementing these methods (e.g. sync.DataSource) satisfies it.
type DataSource interface {
	PreConfirmedBlockLatest(
		ctx context.Context,
		identifier string,
		txCount uint64,
	) (starknet.PreConfirmedUpdate, uint64, error)
	PreConfirmedBlockByNumber(
		ctx context.Context,
		blockNumber uint64,
		identifier string,
		txCount uint64,
	) (starknet.PreConfirmedUpdate, error)
	Class(ctx context.Context, classHash *felt.Felt) (core.ClassDefinition, error)
}

// Poller drives the pre-confirmed chain from a single goroutine.
//
// One tick reads as: poll the server's latest pre-confirmed, backfill any gap
// below it, then insert the latest. backfill runs only when the tip jumped ahead;
// it re-fetches every slot from the old mostRecent up to latest-1 as a full block
// (also capturing their declared classes) and applies each. Same-height polls
// (latest matches our mostRecent) skip backfill and land in insert as
// delta / preserve / replace.
type Poller struct {
	dataSource         DataSource
	storage            *ChainStorage
	blockchain         *blockchain.Blockchain
	out                *feed.Feed[*pending.PreConfirmed]
	highestBlockHeader *atomic.Pointer[core.Header]
	interval           time.Duration
	logger             log.StructuredLogger
}

func NewPoller(
	dataSource DataSource,
	storage *ChainStorage,
	bc *blockchain.Blockchain,
	out *feed.Feed[*pending.PreConfirmed],
	highestBlockHeader *atomic.Pointer[core.Header],
	interval time.Duration,
	logger log.StructuredLogger,
) *Poller {
	return &Poller{
		dataSource:         dataSource,
		storage:            storage,
		blockchain:         bc,
		out:                out,
		highestBlockHeader: highestBlockHeader,
		interval:           interval,
		logger:             logger,
	}
}

// Run polls the sequencer every interval and builds the pre-confirmed chain from it.
// If the blockchain is empty (pre-genesis) then it will stall until the genesis block
// is synced. It only stops on context cancellation.
func (p *Poller) Run(ctx context.Context) {
	if p.interval == 0 {
		p.logger.Info("Pre-confirmed block polling is disabled")
		return
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Guard to prevent the poller from initially running when we are at the genesis
	// state. If the error is different than [db.ErrKeyNotFound] we assume the issue is
	// something else and continue execution.
	for {
		if _, err := p.blockchain.Height(); !errors.Is(err, db.ErrKeyNotFound) {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.tick(ctx); err != nil {
				p.logger.Warn("Pre-confirmed polling failed", zap.Error(err))
			}
		}
	}
}

func (p *Poller) tick(ctx context.Context) error {
	height, err := p.blockchain.Height()
	if err != nil {
		return fmt.Errorf("reading chain height: %w", err)
	}

	oldestPreConf := height + 1
	p.storage.AdvanceTo(oldestPreConf)
	if !p.atTip(height) {
		return nil
	}

	chain := p.storage.SnapshotForBlock(oldestPreConf)
	var (
		mostRecent *pending.PreConfirmed
		identifier string
		txCount    uint64
	)
	fromBlock := oldestPreConf

	if chain.Length() > 0 {
		if mostRecent = chain.Head(); mostRecent != nil {
			fromBlock = mostRecent.Block.Number
			identifier = mostRecent.BlockIdentifier
			txCount = uint64(len(mostRecent.Block.Transactions))
		}
	}

	update, updateBlockNum, err := p.dataSource.PreConfirmedBlockLatest(ctx, identifier, txCount)
	if err != nil {
		return fmt.Errorf("polling latest pre-confirmed: %w", err)
	}

	// NoChange and Delta both imply the server's identifier matched ours,
	// which means the same block (block_identifier is per-block-round).
	// Delta carries block_number on the wire; NoChange may omit it. Falling
	// back to fromBlock is only required for NoChange, but we keep Delta in
	// the switch as a defensive guard in case the wire ever omits it.
	switch update.(type) {
	case starknet.PreConfirmedNoChange, starknet.PreConfirmedDeltaUpdate:
		updateBlockNum = fromBlock
	}

	if updateBlockNum > fromBlock {
		err = p.backfill(ctx, oldestPreConf, mostRecent, fromBlock, identifier, txCount, updateBlockNum)
		if err != nil {
			return fmt.Errorf(
				"backfilling from %d to %d: %w",
				fromBlock, updateBlockNum, err,
			)
		}
	}

	// txCount is mostRecent's tx count; it's only semantically valid as
	// baseTxCount when blockNumber == mostRecent.Block.Number (Delta replay
	// onto the same block we already had). On a forward jump the server saw
	// an identifier mismatch and returned a Full update, whose ApplyUpdate
	// path ignores baseTxCount — so the stale value is harmless under current
	// semantics. Revisit if ApplyUpdate grows a branch that reads baseTxCount
	// for non-Delta updates.
	return p.apply(update, updateBlockNum, txCount, oldestPreConf, nil)
}

// backfill fills the gap [fromBlock, endExclusive), applying each slot with its own
// declared classes so they land on the exact block that declares them. The old tip
// (fromBlock) is re-polled with delta hints, resolving classes from both currentHead (the
// stored old tip, needed to recover classes it declared while it was the tip) and the
// re-poll; the intermediate slots are polled as full blocks. The caller decides when
// backfill is needed; it performs no gap check.
func (p *Poller) backfill(
	ctx context.Context,
	oldestPreConf uint64,
	currentHead *pending.PreConfirmed,
	fromBlockNum uint64,
	identifier string,
	txCount uint64,
	endExclusive uint64,
) error {
	update, err := p.dataSource.PreConfirmedBlockByNumber(ctx, fromBlockNum, identifier, txCount)
	if err != nil {
		return fmt.Errorf("polling pre-confirmed for number %d: %w", fromBlockNum, err)
	}
	newClasses, err := p.fetchDeclaredClasses(ctx, currentHead, update)
	if err != nil {
		return fmt.Errorf("fetching declared classes for pre-confirmed %d: %w", fromBlockNum, err)
	}
	if err := p.apply(update, fromBlockNum, txCount, oldestPreConf, newClasses); err != nil {
		return fmt.Errorf("applying pre-confirmed at %d: %w", fromBlockNum, err)
	}

	for n := fromBlockNum + 1; n < endExclusive; n++ {
		update, err := p.dataSource.PreConfirmedBlockByNumber(ctx, n, "", 0)
		if err != nil {
			return fmt.Errorf("polling pre-confirmed for number %d: %w", n, err)
		}
		newClasses, err := p.fetchDeclaredClasses(ctx, nil, update)
		if err != nil {
			return fmt.Errorf("fetching declared classes for pre-confirmed %d: %w", n, err)
		}
		if err := p.apply(update, n, 0, oldestPreConf, newClasses); err != nil {
			return fmt.Errorf("applying pre-confirmed at %d: %w", n, err)
		}
	}
	return nil
}

// apply writes the update to storage and publishes the affected entry. newClasses
// carries the declared-class definitions to register on the stored entry (backfill's
// most recent update only; nil elsewhere). Returns an error on apply failure so callers
// can abort mid-fill.
func (p *Poller) apply(
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	baseTxCount uint64,
	oldestPreConf uint64,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	applied, err := p.storage.ApplyUpdate(update, blockNumber, baseTxCount, oldestPreConf, newClasses)
	if err != nil {
		return fmt.Errorf("applying pre-confirmed update at block %d: %w", blockNumber, err)
	}
	// A NoChange only ever registers freshly-fetched classes onto an already-stored
	// (non-tip) entry during backfill; nothing changed for feed consumers, so it must
	// not publish even though ApplyUpdate returns the touched entry.
	if _, isNoChange := update.(starknet.PreConfirmedNoChange); isNoChange {
		return nil
	}
	if applied != nil {
		p.out.Send(applied)
	}

	return nil
}

// fetchDeclaredClasses fetches the definitions of every class declared by update — and,
// for a delta or no-change re-poll that continues storedTip's round, by storedTip too —
// deduping so each hash hits the data source at most once. storedTip is nil for a fresh
// slot with no prior stored view. Returns nil when nothing is declared.
func (p *Poller) fetchDeclaredClasses(
	ctx context.Context,
	storedTip *pending.PreConfirmed,
	update starknet.PreConfirmedUpdate,
) (map[felt.Felt]core.ClassDefinition, error) {
	// A delta or no-change re-poll continues storedTip's round, so the classes it already
	// declared belong to this same block; a full block is a fresh round carrying only its
	// own.
	var storedStateDiff *core.StateDiff
	switch update.(type) {
	case starknet.PreConfirmedDeltaUpdate, starknet.PreConfirmedNoChange:
		if storedTip != nil {
			storedStateDiff = storedTip.StateUpdate.StateDiff
		}
	}

	var updateStateDiffs []*starknet.StateDiff
	switch u := update.(type) {
	case starknet.PreConfirmedBlock:
		updateStateDiffs = u.TransactionStateDiffs
	case starknet.PreConfirmedDeltaUpdate:
		updateStateDiffs = u.TransactionStateDiffs
	}

	declaredCount := declaredClassCount(storedStateDiff, updateStateDiffs)
	if declaredCount == 0 {
		return nil, nil
	}

	classes := make(map[felt.Felt]core.ClassDefinition, declaredCount)
	for classHash := range declaredClassHashes(storedStateDiff, updateStateDiffs) {
		class, err := p.dataSource.Class(ctx, classHash)
		if err != nil {
			return nil, fmt.Errorf("fetching class %s: %w", classHash, err)
		}
		classes[*classHash] = class
	}
	return classes, nil
}

// declaredClassHashes yields every class hash declared across the stored and update state
// diffs, in stored-then-update order.
func declaredClassHashes(
	storedStateDiff *core.StateDiff,
	updateStateDiffs []*starknet.StateDiff,
) iter.Seq[*felt.Felt] {
	return func(yield func(*felt.Felt) bool) {
		if storedStateDiff != nil {
			for _, classHash := range storedStateDiff.DeclaredV0Classes {
				if !yield(classHash) {
					return
				}
			}
			for classHash := range storedStateDiff.DeclaredV1Classes {
				if !yield(&classHash) {
					return
				}
			}
		}
		for _, stateDiff := range updateStateDiffs {
			for _, classHash := range stateDiff.OldDeclaredContracts {
				if !yield(classHash) {
					return
				}
			}
			for i := range stateDiff.DeclaredClasses {
				if !yield(stateDiff.DeclaredClasses[i].ClassHash) {
					return
				}
			}
		}
	}
}

// declaredClassCount counts the classes declared across the stored and update state diffs.
// A class is declared at most once, so the sum is exact and sizes the fetch map without
// growing it.
func declaredClassCount(
	storedStateDiff *core.StateDiff,
	updateStateDiffs []*starknet.StateDiff,
) int {
	count := 0
	if storedStateDiff != nil {
		count += len(storedStateDiff.DeclaredV0Classes) + len(storedStateDiff.DeclaredV1Classes)
	}
	for _, stateDiff := range updateStateDiffs {
		count += len(stateDiff.OldDeclaredContracts) + len(stateDiff.DeclaredClasses)
	}
	return count
}

func (p *Poller) atTip(headNum uint64) bool {
	highest := p.highestBlockHeader.Load()
	return highest != nil && highest.Number <= headNum
}
