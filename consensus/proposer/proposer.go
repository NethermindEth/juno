package proposer

import (
	"context"
	"slices"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

const (
	transactionReceiverBufferSize = 1024            // TODO: Make this configurable
	retryTimeForFailedInit        = 1 * time.Second // TODO: Make this configurable
)

type Proposer[V types.Hashable[H], H types.Hash] interface {
	service.Service
	tendermint.Application[V, H]
	OnCommit(context.Context, types.Height, V)
	Submit(transactions []mempool.BroadcastedTransaction)
	Pending() *sync.Pending
}

type proposer[V types.Hashable[H], H types.Hash] struct {
	// Dependencies
	log           utils.Logger
	builder       *builder.Builder
	proposalStore *proposal.ProposalStore[H]
	nodeAddress   starknet.Address
	toValue       func(*felt.Felt) V
	// The current state of the pending block being built
	buildState atomic.Pointer[builder.BuildState]
	// Transactions that we have seen in order to re-run them if needed
	seenTransactions []mempool.BroadcastedTransaction
	// Triggers to commit a block and re-run dropped transactions
	commitTrigger chan map[H]struct{}
	// Triggers to finish building the block and return the proposal value
	valueTrigger chan chan<- V
	// Receives new transactions from the network
	transactionReceiver chan []mempool.BroadcastedTransaction
}

func New[V types.Hashable[H], H types.Hash](
	log utils.Logger,
	b *builder.Builder,
	proposalStore *proposal.ProposalStore[H],
	nodeAddress starknet.Address,
	toValue func(*felt.Felt) V,
) Proposer[V, H] {
	return &proposer[V, H]{
		log:                 log,
		builder:             b,
		proposalStore:       proposalStore,
		nodeAddress:         nodeAddress,
		toValue:             toValue,
		buildState:          atomic.Pointer[builder.BuildState]{},
		seenTransactions:    make([]mempool.BroadcastedTransaction, 0),
		commitTrigger:       make(chan map[H]struct{}, 1),
		valueTrigger:        make(chan chan<- V, 1),
		transactionReceiver: make(chan []mempool.BroadcastedTransaction, transactionReceiverBufferSize),
	}
}

func (p *proposer[V, H]) Run(ctx context.Context) error {
	p.init()

	receiver := p.transactionReceiver
	for {
		select {
		case <-ctx.Done():
			return nil

		case ignoredCommittedTransactions := <-p.commitTrigger:
			// Re-run the previous transactions if any
			if err := p.reRunTransactions(ignoredCommittedTransactions); err != nil {
				p.log.Errorw("Fail to re-run transactions", "error", err)
				p.init()
				continue
			}
			// Accept new transactions again
			receiver = p.transactionReceiver

		case response := <-p.valueTrigger:
			// Finalise the block and store the build result
			if err := p.finish(response); err != nil {
				p.log.Errorw("Fail to finish proposer", "error", err)
				p.init()
				continue
			}
			// Stop accepting new transactions
			receiver = nil

		case transactions := <-receiver:
			// Accept new transactions
			if err := p.receiveTransactions(transactions); err != nil {
				p.log.Errorw("Fail to receive transactions", "error", err)
				p.init()
				continue
			}
		}
	}
}

func (p *proposer[V, H]) OnCommit(ctx context.Context, height types.Height, value V) {
	proposal := p.proposalStore.Get(value.Hash())
	if proposal == nil {
		p.log.Errorw("Proposal not found", "hash", value.Hash())
		return
	}

	txHashSet := make(map[H]struct{})
	for _, tx := range proposal.Pending.Block.Transactions {
		txHashSet[H(*tx.Hash())] = struct{}{}
	}

	select {
	case <-ctx.Done():
		return
	case p.commitTrigger <- txHashSet:
	}
}

func (p *proposer[V, H]) Valid(value V) bool {
	return p.proposalStore.Get(value.Hash()) != nil
}

func (p *proposer[V, H]) Value() V {
	responseChannel := make(chan V)
	p.valueTrigger <- responseChannel
	return <-responseChannel
}

func (p *proposer[V, H]) Submit(transactions []mempool.BroadcastedTransaction) {
	p.transactionReceiver <- transactions
}

// Return the pending block currently guarded by the atomic pointer. The implementation assumes that
// the referenced value by the atomic pointer is immutable, which means the caller shouldn't modify
// any fields of the returned pending block.
func (p *proposer[V, H]) Pending() *sync.Pending {
	return p.buildState.Load().Pending
}

func (p *proposer[V, H]) init() {
	var buildState *builder.BuildState
	var err error
	for {
		buildParams := p.getBuildParams()
		if buildState, err = p.builder.InitPendingBlock(&buildParams); err != nil {
			p.log.Errorw("Fail to reinitialize proposer", "error", err)
			time.Sleep(retryTimeForFailedInit)
			continue
		}
		break
	}

	p.buildState.Store(buildState)
}

func (p *proposer[V, H]) getBuildParams() builder.BuildParams {
	return builder.BuildParams{
		Builder:           felt.Felt(p.nodeAddress),
		Timestamp:         uint64(time.Now().Unix()),
		L2GasPriceFRI:     felt.One,  // TODO: Implement this properly
		L1GasPriceWEI:     felt.One,  // TODO: Implement this properly
		L1DataGasPriceWEI: felt.One,  // TODO: Implement this properly
		EthToStrkRate:     felt.One,  // TODO: Implement this properly
		L1DAMode:          core.Blob, // TODO: Implement this properly
	}
}

func (p *proposer[V, H]) reRunTransactions(ignored map[H]struct{}) error {
	// Initialise the state back to pending block
	p.init()

	// Discard the transactions that we have already seen
	p.seenTransactions = slices.DeleteFunc(p.seenTransactions, func(tx mempool.BroadcastedTransaction) bool {
		_, ok := ignored[H(*tx.Transaction.Hash())]
		return ok
	})

	// If there are no transactions to run, we're done
	if len(p.seenTransactions) == 0 {
		return nil
	}

	// Run the transactions and discard them if we fail to run them
	if err := p.runTransactions(p.seenTransactions); err != nil {
		p.seenTransactions = nil
		return err
	}

	return nil
}

func (p *proposer[V, H]) receiveTransactions(transactions []mempool.BroadcastedTransaction) error {
	if err := p.runTransactions(transactions); err != nil {
		return err
	}

	p.seenTransactions = append(p.seenTransactions, transactions...)
	return nil
}

func (p *proposer[V, H]) runTransactions(transaction []mempool.BroadcastedTransaction) error {
	buildState := p.buildState.Load().Clone()
	err := p.builder.RunTxns(&buildState, transaction)
	if err != nil {
		return err
	}

	p.buildState.Store(&buildState)
	return nil
}

func (p *proposer[V, H]) finish(responseChannel chan<- V) error {
	buildState := p.buildState.Load().Clone()
	buildResult, err := p.builder.Finish(&buildState)
	if err != nil {
		return err
	}

	value := p.toValue(buildResult.Pending.Block.Hash)
	p.proposalStore.Store(value.Hash(), &buildResult)

	responseChannel <- value
	close(responseChannel)

	return nil
}
