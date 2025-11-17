package sequencer

import (
	"context"
	"errors"
	syncLock "sync"
	"time"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"

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
	buildState       *builder.BuildState
	sequencerAddress *felt.Felt
	privKey          *ecdsa.PrivateKey
	log              utils.Logger
	blockTime        time.Duration
	mempool          *mempool.SequencerMempool

	subNewHeads          *feed.Feed[*core.Block]
	subPendingData       *feed.Feed[core.PendingData]
	subReorgFeed         *feed.Feed[*sync.ReorgBlockRange]
	subPreConfirmedBlock *feed.Feed[*core.PreConfirmed]
	subPreLatest         *feed.Feed[*core.PreLatest]
	plugin               plugin.JunoPlugin

	mu syncLock.RWMutex
}

func New(
	b *builder.Builder,
	mempool *mempool.SequencerMempool,
	sequencerAddress *felt.Felt,
	privKey *ecdsa.PrivateKey,
	blockTime time.Duration,
	log utils.Logger,
) Sequencer {
	return Sequencer{
		builder:              b,
		buildState:           &builder.BuildState{},
		mempool:              mempool,
		sequencerAddress:     sequencerAddress,
		privKey:              privKey,
		log:                  log,
		blockTime:            blockTime,
		subNewHeads:          feed.New[*core.Block](),
		subPendingData:       feed.New[core.PendingData](),
		subReorgFeed:         feed.New[*sync.ReorgBlockRange](),
		subPreConfirmedBlock: feed.New[*core.PreConfirmed](),
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
		if pErr := s.buildState.ClearPending(); pErr != nil {
			s.log.Errorw("clearing pending", "err", pErr)
		}
	}()

	if err := s.initPendingBlock(); err != nil {
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

			if err := s.initPendingBlock(); err != nil {
				return err
			}
			s.mu.Unlock()
		}
	}
}

func (s *Sequencer) initPendingBlock() error {
	buildParams := builder.BuildParams{
		Builder:           *s.sequencerAddress,
		L2GasPriceFRI:     felt.One,
		L1GasPriceWEI:     felt.One,
		L1DataGasPriceWEI: felt.One,
		EthToStrkRate:     felt.One,
		L1DAMode:          core.Calldata,
	}

	var err error
	if err = s.buildState.ClearPending(); err != nil {
		return err
	}

	if s.buildState, err = s.builder.InitPreconfirmedBlock(&buildParams); err != nil {
		return err
	}

	return nil
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
		pending := core.NewPending(s.buildState.PendingBlock(), nil, nil)
		s.subPendingData.Send(&pending)
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
		if err = s.builder.RunTxns(s.buildState, userTxns); err != nil {
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

func (s *Sequencer) Pending() (*core.PreConfirmed, error) {
	return s.buildState.Preconfirmed, nil
}

func (s *Sequencer) PendingBlock() *core.Block {
	return s.buildState.PendingBlock()
}

func (s *Sequencer) PendingState() (core.CommonStateReader, func() error, error) {
	return s.builder.PendingState(s.buildState)
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

func (s *Sequencer) SubscribePendingData() sync.PendingDataSubscription {
	return sync.PendingDataSubscription{Subscription: s.subPendingData.Subscribe()}
}

func (s *Sequencer) SubscribePreLatest() sync.PreLatestDataSubscription {
	return sync.PreLatestDataSubscription{Subscription: s.subPreLatest.Subscribe()}
}

func (s *Sequencer) PendingData() (core.PendingData, error) {
	return nil, nil
}
