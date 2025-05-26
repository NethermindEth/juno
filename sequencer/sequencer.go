package sequencer

import (
	"context"
	"errors"
	"time"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/plugin"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var (
	_                     service.Service = (*Sequencer)(nil)
	_                     sync.Reader     = (*Sequencer)(nil)
	NumTxnsToBatchExecute                 = 10
	ErrPendingParentHash                  = errors.New("pending block parent hash does not match chain head")
)

type Sequencer struct {
	builder         *builder.Builder
	log             utils.Logger
	blockTime       time.Duration
	subNewHeads     *feed.Feed[*core.Block]
	subPendingBlock *feed.Feed[*core.Block]
	subReorgFeed    *feed.Feed[*sync.ReorgBlockRange]
	plugin          plugin.JunoPlugin
}

func New(
	builder *builder.Builder,
	blockTime time.Duration,
	plugin plugin.JunoPlugin,
	log utils.Logger,
) Sequencer {
	return Sequencer{
		builder:         builder,
		log:             log,
		blockTime:       blockTime,
		subNewHeads:     feed.New[*core.Block](),
		subPendingBlock: feed.New[*core.Block](),
		subReorgFeed:    feed.New[*sync.ReorgBlockRange](),
		plugin:          plugin,
	}
}

// Sign returns the builder's signature over data.
func Sign(privKey ecdsa.PrivateKey, blockHash, stateDiffCommitment *felt.Felt) ([]*felt.Felt, error) {
	data := crypto.PoseidonArray(blockHash, stateDiffCommitment).Bytes()
	signatureBytes, err := privKey.Sign(data[:], nil)
	if err != nil {
		return nil, err
	}
	sig := make([]*felt.Felt, 0)
	for start := 0; start < len(signatureBytes); {
		step := len(signatureBytes[start:])
		if step > felt.Bytes {
			step = felt.Bytes
		}
		sig = append(sig, new(felt.Felt).SetBytes(signatureBytes[start:step]))
		start += step
	}
	return sig, nil
}

func (s *Sequencer) Run(ctx context.Context) error {
	defer s.builder.Mempool.Close()

	// Clear pending state on shutdown
	defer func() {
		if pErr := s.builder.ClearPending(); pErr != nil {
			s.log.Errorw("clearing pending", "err", pErr)
		}
	}()

	if err := s.builder.InitPendingBlock(); err != nil {
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
			err := s.builder.Finalise(Sign)
			s.log.Infof("Finalised new block")
			if err != nil {
				return err
			}
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
		case <-s.builder.Mempool.Wait():
			continue
		}
	}
}

// depletePool pops all available transactions from the mempool,
// and executes them in sequence, applying the state changes
// to the pending state
func (s *Sequencer) depletePool(ctx context.Context) error {
	blockHashToBeRevealed, err := s.builder.GetRevealedBlockHash()
	if err != nil {
		return err
	}
	for {

		userTxns, err := s.builder.Mempool.PopBatch(NumTxnsToBatchExecute)
		if err != nil {
			return err
		}
		s.log.Debugw("running txns", userTxns)
		if err = s.builder.RunTxns(userTxns, blockHashToBeRevealed); err != nil {
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
