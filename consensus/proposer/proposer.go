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
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

const (
	transactionReceiverBufferSize = 1024            // TODO: Make this configurable
	retryTimeForFailedInit        = 1 * time.Second // TODO: Make this configurable
)

type request[D, R any] struct {
	data     D
	response chan R
}

type (
	commitRequest[H types.Hash]                     = request[map[H]struct{}, struct{}]
	valueRequest[V types.Hashable[H], H types.Hash] = request[struct{}, V]
)

type Proposer[V types.Hashable[H], H types.Hash] interface {
	service.Service
	tendermint.Application[V, H]
	mempool.Pool
	OnCommit(context.Context, types.Height, V)
	Submit(context.Context, []mempool.BroadcastedTransaction)
	Preconfirmed() *core.PreConfirmed
}

type proposer[V types.Hashable[H], H types.Hash] struct {
	// Dependencies
	log           utils.Logger
	builder       *builder.Builder
	proposalStore *proposal.ProposalStore[H]
	nodeAddress   starknet.Address
	toValue       func(*felt.Felt) V
	// The current state of the preconfirmed block being built
	buildState atomic.Pointer[builder.BuildState]
	// This is used to send the last value to the tendermint application. Only used after context cancellation
	lastValue chan V
	// Transactions that we have seen in order to re-run them if needed
	seenTransactions []mempool.BroadcastedTransaction
	// Triggers to commit a block and re-run dropped transactions
	commitTrigger chan commitRequest[H]
	// Triggers to finish building the block and return the proposal value
	valueTrigger chan valueRequest[V, H]
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
	p := &proposer[V, H]{
		log:                 log,
		builder:             b,
		proposalStore:       proposalStore,
		nodeAddress:         nodeAddress,
		toValue:             toValue,
		buildState:          atomic.Pointer[builder.BuildState]{},
		lastValue:           make(chan V, 1),
		seenTransactions:    make([]mempool.BroadcastedTransaction, 0),
		commitTrigger:       make(chan commitRequest[H], 1),
		valueTrigger:        make(chan valueRequest[V, H], 1),
		transactionReceiver: make(chan []mempool.BroadcastedTransaction, transactionReceiverBufferSize),
	}
	// note(rdr): This is here because I don't want to add a nil check on PreConfirmed and keep
	// the function low cost. This solves the problem when calling `Run` which then
	// calls `init()` which initialises `buildState`.
	// If at the same time, in a different go-routine, `Preconfirmed()` gets called, while
	// `buildState` hasn't been loaded yet, a nil pointer deference error would be thrown.
	// This doesn't happen in production code (since there isn't any) but it happens in tests.
	// When we write production consensus code this should be revisited.
	p.buildState.Store(&builder.BuildState{})
	return p
}

func (p *proposer[V, H]) Run(ctx context.Context) error {
	p.init()

	receiver := p.transactionReceiver
	for {
		select {
		case <-ctx.Done():
			// Finish the block and record the last value in case the application needs it
			p.finish(p.lastValue)
			return nil

		case request := <-p.commitTrigger:
			// Re-run the previous transactions if any
			p.reRunTransactions(request)
			// Accept new transactions again
			receiver = p.transactionReceiver

		case request := <-p.valueTrigger:
			// Finalise the block and store the build result
			p.finish(request.response)
			// Stop accepting new transactions
			receiver = nil

		case transactions := <-receiver:
			// Accept new transactions
			p.receiveTransactions(transactions)
		}
	}
}

func (p *proposer[V, H]) OnCommit(ctx context.Context, height types.Height, value V) {
	proposal := p.proposalStore.Get(value.Hash())
	if proposal == nil {
		// todo(rdr): Hash should be able to be printed (perhaps make it implement Stringer)
		p.log.Error("Proposal not found", zap.Any("hash", value.Hash()))
		return
	}

	txHashSet := make(map[H]struct{})
	for _, tx := range proposal.Preconfirmed.Block.Transactions {
		txHashSet[H(*tx.Hash())] = struct{}{}
	}

	ask(p.commitTrigger, txHashSet, ctx.Done())
}

func (p *proposer[V, H]) Valid(value V) bool {
	return p.proposalStore.Get(value.Hash()) != nil
}

func (p *proposer[V, H]) Value() V {
	return ask(p.valueTrigger, struct{}{}, p.lastValue)
}

func (p *proposer[V, H]) Submit(ctx context.Context, transactions []mempool.BroadcastedTransaction) {
	select {
	case <-ctx.Done():
	case p.transactionReceiver <- transactions:
	}
}

func (p *proposer[V, H]) Push(ctx context.Context, transaction *mempool.BroadcastedTransaction) error {
	p.Submit(ctx, []mempool.BroadcastedTransaction{*transaction})
	return nil
}

// Return the preconfirmed block currently guarded by the atomic pointer. The implementation
// assumes that the referenced value by the atomic pointer is immutable, which means the caller
// shouldn't modify any fields of the returned preconfirmed block.
func (p *proposer[V, H]) Preconfirmed() *core.PreConfirmed {
	return p.buildState.Load().Preconfirmed
}

func (p *proposer[V, H]) init() {
	var buildState *builder.BuildState
	var err error
	for {
		buildParams := p.getBuildParams()
		if buildState, err = p.builder.InitPreconfirmedBlock(&buildParams); err != nil {
			p.log.Error("Fail to reinitialize proposer", zap.Error(err))
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

func (p *proposer[V, H]) reRunTransactions(request commitRequest[H]) {
	defer close(request.response)
	ignoredCommittedTransactions := request.data

	// Initialise the state back to preconfirmed block
	p.init()

	// Discard the transactions that we have already seen
	p.seenTransactions = slices.DeleteFunc(p.seenTransactions, func(tx mempool.BroadcastedTransaction) bool {
		_, ok := ignoredCommittedTransactions[H(*tx.Transaction.Hash())]
		return ok
	})

	// If there are no transactions to run, we're done
	if len(p.seenTransactions) == 0 {
		return
	}

	// Run the transactions and discard them if we fail to run them
	if err := p.runTransactions(p.seenTransactions); err != nil {
		p.log.Error("Fail to re-run transactions", zap.Error(err))
		p.seenTransactions = nil
	}
}

func (p *proposer[V, H]) receiveTransactions(transactions []mempool.BroadcastedTransaction) {
	if err := p.runTransactions(transactions); err != nil {
		p.log.Error("Fail to receive transactions", zap.Error(err))
		return
	}

	p.seenTransactions = append(p.seenTransactions, transactions...)
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

func (p *proposer[V, H]) finish(responseChannel chan<- V) {
	defer close(responseChannel)
	buildState := p.buildState.Load().Clone()
	buildResult, err := p.builder.Finish(&buildState)
	for err != nil {
		p.log.Error("Fail to finish proposer", zap.Error(err))
		p.init()
		buildResult, err = p.builder.Finish(&buildState)
	}

	value := p.toValue(buildResult.Preconfirmed.Block.Hash)
	p.proposalStore.Store(value.Hash(), &buildResult)

	responseChannel <- value
}

// ask is a helper function to send a request to a long running loop listening on the channel.
// If fallback is available, it will return the value from the fallback channel.
// Otherwise, the long running loop will process the request and send a response back on a channel.
func ask[D, R any](channel chan request[D, R], data D, fallback <-chan R) R {
	request := request[D, R]{
		data:     data,
		response: make(chan R),
	}

	select {
	case res := <-fallback:
		return res
	case channel <- request:
	}

	select {
	case res := <-fallback:
		return res
	case result := <-request.response:
		return result
	}
}
