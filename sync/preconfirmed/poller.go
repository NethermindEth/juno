package preconfirmed

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// DataSource is the narrow surface the Poller needs from the wire side. Any
// type implementing both methods (e.g. sync.DataSource) satisfies it.
type DataSource interface {
	PreConfirmedBlockLatest(
		ctx context.Context,
		identifier starknet.BlockIdentifier,
		txCount uint64,
	) (starknet.PreConfirmedUpdate, uint64, error)
	PreConfirmedBlockByNumber(
		ctx context.Context,
		blockNumber uint64,
		identifier starknet.BlockIdentifier,
		txCount uint64,
	) (starknet.PreConfirmedUpdate, error)
}

// Poller drives the pre-confirmed chain from a single goroutine.
//
// One tick reads as: poll the server's latest pre-confirmed, backfill any gap
// below it, then insert the latest. backfill is a no-op when there's no gap;
// otherwise it finalises the current mostRecent (re-polls its number to capture
// the last delta before the sequencer moved past it) and then walks
// explicit-number polls up to latest-1. Same-height polls (latest matches our
// mostRecent) skip backfill and land in insert as delta / preserve / replace.
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

func (p *Poller) Run(ctx context.Context) {
	if p.interval == 0 {
		p.logger.Info("Pre-confirmed block polling is disabled")
		return
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

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
	head, err := p.blockchain.HeadsHeader()
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return fmt.Errorf("reading heads header: %w", err)
		}
		head = nil
	}
	p.storage.AdvanceTo(head)
	if !p.atTip(head) {
		return nil
	}

	chain := p.storage.SnapshotForHead(head)
	var (
		mostRecent *pending.PreConfirmed
		identifier starknet.BlockIdentifier
		txCount    uint64
	)
	fromBlock := headPlusOne(head)

	if chain.Length() > 0 {
		if mostRecent = chain.Head(); mostRecent != nil {
			fromBlock = mostRecent.Block.Number
			identifier = starknet.BlockIdentifier(mostRecent.BlockIdentifier)
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
		err = p.backfill(ctx, head, fromBlock, identifier, txCount, updateBlockNum)
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
	return p.apply(update, updateBlockNum, txCount, head)
}

// backfill polls fromBlock with the given delta hints (identifier+txCount) to
// capture the final view of that block, then walks fromBlock+1..endExclusive-1
// with blank hints. The caller is responsible for deciding when backfill is
// needed; backfill itself performs no gap check.
func (p *Poller) backfill(
	ctx context.Context,
	head *core.Header,
	fromBlockNum uint64,
	identifier starknet.BlockIdentifier,
	txCount uint64,
	endExclusive uint64,
) error {
	update, err := p.dataSource.PreConfirmedBlockByNumber(ctx, fromBlockNum, identifier, txCount)
	if err != nil {
		return fmt.Errorf("polling pre-confirmed for number %d: %w", fromBlockNum, err)
	}

	if err := p.apply(update, fromBlockNum, txCount, head); err != nil {
		return fmt.Errorf("applying pre-confirmed at %d: %w", fromBlockNum, err)
	}

	for n := fromBlockNum + 1; n < endExclusive; n++ {
		update, err := p.dataSource.PreConfirmedBlockByNumber(ctx, n, 0, 0)
		if err != nil {
			return fmt.Errorf("polling pre-confirmed for number %d: %w", n, err)
		}

		if err := p.apply(update, n, 0, head); err != nil {
			return fmt.Errorf("applying pre-confirmed at %d: %w", n, err)
		}
	}
	return nil
}

// apply writes the update to storage and publishes the affected entry.
// Returns an error on apply failure so callers can abort mid-fill.
func (p *Poller) apply(
	update starknet.PreConfirmedUpdate,
	blockNumber uint64,
	baseTxCount uint64,
	head *core.Header,
) error {
	applied, err := p.storage.ApplyUpdate(update, blockNumber, baseTxCount, head)
	if err != nil {
		return fmt.Errorf("applying pre-confirmed update at block %d: %w", blockNumber, err)
	}
	if applied != nil {
		p.out.Send(applied)
	}

	return nil
}

func (p *Poller) atTip(head *core.Header) bool {
	highest := p.highestBlockHeader.Load()
	if highest == nil {
		return false
	}

	var headNum uint64
	if head != nil {
		headNum = head.Number
	}
	return highest.Number <= headNum
}
