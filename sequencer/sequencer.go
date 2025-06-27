package sequencer

import (
	"context"
	"errors"
	syncLock "sync"
	"time"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/plugin"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
)

var (
	_                     service.Service = (*Sequencer)(nil)
	_                     sync.Reader     = (*Sequencer)(nil)
	NumTxnsToBatchExecute                 = 10
)

type Sequencer struct {
	builder          *builder.Builder
	sequencerAddress *felt.Felt
	privKey          *ecdsa.PrivateKey
	log              utils.Logger
	blockTime        time.Duration
	mempool          *mempool.Pool

	subNewHeads     *feed.Feed[*core.Block]
	subPendingBlock *feed.Feed[*core.Block]
	subReorgFeed    *feed.Feed[*sync.ReorgBlockRange]
	plugin          plugin.JunoPlugin

	mu syncLock.RWMutex
}

func New(
	builder *builder.Builder,
	mempool *mempool.Pool,
	sequencerAddress *felt.Felt,
	privKey *ecdsa.PrivateKey,
	blockTime time.Duration,
	log utils.Logger,
) Sequencer {
	return Sequencer{
		builder:          builder,
		mempool:          mempool,
		sequencerAddress: sequencerAddress,
		privKey:          privKey,
		log:              log,
		blockTime:        blockTime,
		subNewHeads:      feed.New[*core.Block](),
		subPendingBlock:  feed.New[*core.Block](),
		subReorgFeed:     feed.New[*sync.ReorgBlockRange](),
	}
}

func (s *Sequencer) WithPlugin(junoPlugin plugin.JunoPlugin) *Sequencer {
	s.plugin = junoPlugin
	return s
}

func (s *Sequencer) Run(ctx context.Context) error {
	defer s.mempool.Close()

	// Clear pending state on shutdown
	defer func() {
		if pErr := s.builder.ClearPending(); pErr != nil {
			s.log.Errorw("clearing pending", "err", pErr)
		}
	}()

	if err := s.builder.InitPendingBlock(s.sequencerAddress); err != nil {
		return err
	}

	doneListen := make(chan struct{})
	go func() {
		if pErr := s.listenPool(ctx); pErr != nil {
			if pErr != mempool.ErrTxnPoolEmpty {
				s.log.Warnw("listening pool", "err", pErr)
			}
		}
		close(doneListen)
	}()

	for {
		select {
		case <-ctx.Done():
			<-doneListen
			return nil
		case <-time.After(s.blockTime):
			s.mu.Lock()

			pending, err := s.Pending()
			if err != nil {
				s.log.Infof("Failed to get pending block")
			}
			if err := s.builder.Finalise(pending, utils.Sign(s.privKey), s.privKey); err != nil {
				return err
			}
			s.log.Infof("Finalised new block")
			if s.plugin != nil {
				err := s.plugin.NewBlock(pending.Block, pending.StateUpdate, pending.NewClasses)
				if err != nil {
					s.log.Errorw("error sending new block to plugin", err)
				}
			}
			// push the new head to the feed
			s.subNewHeads.Send(pending.Block)

			if err := s.builder.ClearPending(); err != nil {
				return err
			}
			if err := s.builder.InitPendingBlock(s.sequencerAddress); err != nil {
				return err
			}
			s.mu.Unlock()
		}
	}
}

// listenPool waits until the mempool has transactions, then
// executes them one by one until the mempool is empty.
func (s *Sequencer) listenPool(ctx context.Context) error {
	for {
		if err := s.depletePool(ctx); err != nil {
			if !errors.Is(err, mempool.ErrTxnPoolEmpty) {
				return err
			}
		}

		// push the pending block to the feed
		s.subPendingBlock.Send(s.builder.PendingBlock())
		select {
		case <-ctx.Done():
			return nil
		// We wait for the mempool to get more txns before we continue
		case <-s.mempool.Wait():
			continue
		}
	}
}

// depletePool pops all available transactions from the mempool,
// and executes them in sequence, applying the state changes
// to the pending state
func (s *Sequencer) depletePool(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		userTxns, err := s.mempool.PopBatch(NumTxnsToBatchExecute)
		if err != nil {
			return err
		}
		s.log.Debugw("running txns", userTxns)
		if err = s.builder.RunTxns(userTxns); err != nil {
			s.log.Debugw("failed running txn", "err", err.Error())
			var txnExecutionError vm.TransactionExecutionError
			if !errors.As(err, &txnExecutionError) {
				return err
			}
		}
		s.log.Debugw("running txns success")
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func (s *Sequencer) Pending() (*sync.Pending, error) {
	return s.builder.Pending()
}

func (s *Sequencer) PendingBlock() *core.Block {
	return s.builder.PendingBlock()
}

func (s *Sequencer) PendingState() (state.StateReader, error) {
	return s.builder.PendingState()
}

func (s *Sequencer) HighestBlockHeader() *core.Header {
	return nil // Not relevant for Sequencer. Todo: clean Reader
}

func (s *Sequencer) StartingBlockNumber() (uint64, error) {
	return 0, nil // Not relevant for Sequencer. Todo: clean Reader
}

// The builder has no reorg logic (centralised sequencer that can't reorg)
func (s *Sequencer) SubscribeReorg() sync.ReorgSubscription {
	return sync.ReorgSubscription{Subscription: s.subReorgFeed.Subscribe()}
}

func (s *Sequencer) SubscribeNewHeads() sync.NewHeadSubscription {
	return sync.NewHeadSubscription{Subscription: s.subNewHeads.Subscribe()}
}

func (s *Sequencer) SubscribePending() sync.PendingSubscription {
	return sync.PendingSubscription{Subscription: s.subPendingBlock.Subscribe()}
}
