package builder

import (
	"context"
	"errors"
	"fmt"
	musync "sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
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
	_                     service.Service = (*Builder)(nil)
	_                     sync.Reader     = (*Builder)(nil)
	NumTxnsToBatchExecute                 = 10
	ErrPendingParentHash                  = errors.New("pending block parent hash does not match chain head")
)

type Builder struct {
	ownAddress  felt.Felt
	privKey     *ecdsa.PrivateKey
	blockTime   time.Duration
	disableFees bool

	bc              *blockchain.Blockchain
	db              db.DB
	vm              vm.VM
	log             utils.Logger
	subNewHeads     *feed.Feed[*core.Block]
	subPendingBlock *feed.Feed[*core.Block]
	subReorgFeed    *feed.Feed[*sync.ReorgBlockRange]
	mempool         *mempool.Pool
	plugin          plugin.JunoPlugin

	pendingBlock atomic.Pointer[sync.Pending]
	headState    core.StateReader
	headCloser   blockchain.StateCloser

	finaliseMutex musync.RWMutex
}

func New(privKey *ecdsa.PrivateKey, ownAddr *felt.Felt, bc *blockchain.Blockchain, vm vm.VM,
	blockTime time.Duration, mempool *mempool.Pool, log utils.Logger, disableFees bool, database db.DB,
) Builder {
	return Builder{
		ownAddress: *ownAddr,
		privKey:    privKey,
		blockTime:  blockTime,
		log:        log,

		disableFees:     disableFees,
		bc:              bc,
		db:              database,
		mempool:         mempool,
		vm:              vm,
		subNewHeads:     feed.New[*core.Block](),
		subPendingBlock: feed.New[*core.Block](),
		subReorgFeed:    feed.New[*sync.ReorgBlockRange](),
		finaliseMutex:   musync.RWMutex{},
	}
}

func (b *Builder) WithPlugin(junoPlugin plugin.JunoPlugin) *Builder {
	b.plugin = junoPlugin
	return b
}

func (b *Builder) Pending() (*sync.Pending, error) {
	p := b.pendingBlock.Load()
	if p == nil {
		return nil, sync.ErrPendingBlockNotFound
	}

	expectedParentHash := &felt.Zero
	if head, err := b.bc.HeadsHeader(); err == nil {
		expectedParentHash = head.Hash
	}
	if p.Block.ParentHash.Equal(expectedParentHash) {
		return p, nil
	}

	return nil, ErrPendingParentHash
}

func (b *Builder) PendingBlock() *core.Block {
	pending, err := b.Pending()
	if err != nil {
		return nil
	}
	return pending.Block
}

func (b *Builder) PendingState() (core.StateReader, func() error, error) {
	txn, err := b.db.NewTransaction(false)
	if err != nil {
		return nil, nil, err
	}

	pending, err := b.Pending()
	if err != nil {
		return nil, nil, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return sync.NewPendingState(pending.StateUpdate.StateDiff, pending.NewClasses, b.headState), txn.Discard, nil
}

func (b *Builder) Run(ctx context.Context) error {
	defer b.mempool.Close()

	// Clear pending state on shutdown
	defer func() {
		if pErr := b.ClearPending(); pErr != nil {
			b.log.Errorw("clearing pending", "err", pErr)
		}
	}()

	if err := b.InitPendingBlock(); err != nil {
		return err
	}

	doneListen := make(chan struct{})
	go func() {
		if pErr := b.listenPool(ctx); pErr != nil {
			if pErr != mempool.ErrTxnPoolEmpty {
				b.log.Warnw("listening pool", "err", pErr)
			}
		}
		close(doneListen)
	}()

	for {
		select {
		case <-ctx.Done():
			<-doneListen
			return nil
		case <-time.After(b.blockTime):
			err := b.Finalise(b.Sign)
			b.log.Infof("Finalised new block")
			if err != nil {
				return err
			}
		}
	}
}

func (b *Builder) ClearPending() error {
	b.pendingBlock.Store(&sync.Pending{})

	if b.headState != nil {
		if err := b.headCloser(); err != nil {
			return err
		}
		b.headState = nil
		b.headCloser = nil
	}
	return nil
}

func (b *Builder) InitPendingBlock() error {
	header, err := b.bc.HeadsHeader()
	if err != nil {
		return err
	}
	pendingBlock := core.Block{
		Header: &core.Header{
			ParentHash:       header.Hash,
			Number:           header.Number + 1,
			SequencerAddress: &b.ownAddress,
			L1GasPriceETH:    felt.One.Clone(),
			L1GasPriceSTRK:   felt.One.Clone(),
			L1DAMode:         core.Calldata,
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: felt.One.Clone(),
				PriceInFri: felt.One.Clone(),
			},
		},
		Transactions: []core.Transaction{},
		Receipts:     []*core.TransactionReceipt{},
	}
	newClasses := make(map[felt.Felt]core.Class)
	emptyStateDiff := core.EmptyStateDiff()
	su := core.StateUpdate{
		StateDiff: &emptyStateDiff,
	}
	pending := sync.Pending{
		Block:       &pendingBlock,
		StateUpdate: &su,
		NewClasses:  newClasses,
	}
	b.pendingBlock.Store(&pending)
	b.headState, b.headCloser, err = b.bc.HeadState()
	return err
}

// Finalise the pending block and initialise the next one
func (b *Builder) Finalise(signFunc blockchain.BlockSignFunc) error {
	b.finaliseMutex.Lock()
	defer b.finaliseMutex.Unlock()

	pending, err := b.Pending()
	if err != nil {
		return err
	}
	if err := b.bc.Finalise(pending.Block, pending.StateUpdate, pending.NewClasses, b.Sign); err != nil {
		return err
	}
	b.log.Infow("Finalised block", "number", pending.Block.Number, "hash",
		pending.Block.Hash.ShortString(), "state", pending.Block.GlobalStateRoot.ShortString())

	if b.plugin != nil {
		err := b.plugin.NewBlock(pending.Block, pending.StateUpdate, pending.NewClasses)
		if err != nil {
			b.log.Errorw("error sending new block to plugin", err)
		}
	}
	// push the new block head to the feed
	b.subNewHeads.Send(b.PendingBlock())
	if err := b.ClearPending(); err != nil {
		return err
	}
	return b.InitPendingBlock()
}

func (b *Builder) StartingBlockNumber() (uint64, error) {
	return 0, nil
}

func (b *Builder) HighestBlockHeader() *core.Header {
	return nil
}

// Sign returns the builder's signature over data.
func (b *Builder) Sign(blockHash, stateDiffCommitment *felt.Felt) ([]*felt.Felt, error) {
	data := crypto.PoseidonArray(blockHash, stateDiffCommitment).Bytes()
	signatureBytes, err := b.privKey.Sign(data[:], nil)
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

// listenPool waits until the mempool has transactions, then
// executes them one by one until the mempool is empty.
func (b *Builder) listenPool(ctx context.Context) error {
	for {
		if err := b.depletePool(ctx); err != nil {
			if !errors.Is(err, mempool.ErrTxnPoolEmpty) {
				return err
			}
		}

		// push the pending block to the feed
		b.subPendingBlock.Send(b.PendingBlock())
		select {
		case <-ctx.Done():
			return nil
		// We wait for the mempool to get more txns before we continue
		case <-b.mempool.Wait():
			continue
		}
	}
}

// depletePool pops all available transactions from the mempool,
// and executes them in sequence, applying the state changes
// to the pending state
func (b *Builder) depletePool(ctx context.Context) error {
	blockHashToBeRevealed, err := b.getRevealedBlockHash()
	if err != nil {
		return err
	}
	for {
		b.finaliseMutex.RLock()
		userTxns, err := b.mempool.PopBatch(NumTxnsToBatchExecute)
		if err != nil {
			b.finaliseMutex.RUnlock()
			return err
		}
		b.log.Debugw("running txns", userTxns)
		if err = b.runTxns(userTxns, blockHashToBeRevealed); err != nil {
			b.log.Debugw("failed running txn", "err", err.Error())
			var txnExecutionError vm.TransactionExecutionError
			if !errors.As(err, &txnExecutionError) {
				b.finaliseMutex.RUnlock()
				return err
			}
		}
		b.log.Debugw("running txns success")
		b.finaliseMutex.RUnlock()
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func (b *Builder) getRevealedBlockHash() (*felt.Felt, error) {
	blockHeight, err := b.bc.Height()
	if err != nil {
		return nil, err
	}
	const blockHashLag = 10
	if blockHeight < blockHashLag {
		return nil, nil
	}

	header, err := b.bc.BlockHeaderByNumber(blockHeight - blockHashLag)
	if err != nil {
		return nil, err
	}
	return header.Hash, nil
}

// runTxns executes the provided transaction and applies the state changes
// to the pending state
func (b *Builder) runTxns(txns []mempool.BroadcastedTransaction, blockHashToBeRevealed *felt.Felt) error {
	// Get the pending state
	pending, err := b.Pending()
	if err != nil {
		return err
	}

	// Create a state writer for the transaction execution
	state := sync.NewPendingStateWriter(pending.StateUpdate.StateDiff, pending.NewClasses, b.headState)

	// Prepare declared classes, if any
	var declaredClasses []core.Class
	paidFeesOnL1 := []*felt.Felt{}
	coreTxns := make([]core.Transaction, len(txns))
	for i, txn := range txns {
		if txn.DeclaredClass != nil {
			declaredClasses = append(declaredClasses, txn.DeclaredClass)
		}
		if txn.PaidFeeOnL1 != nil {
			paidFeesOnL1 = append(paidFeesOnL1, txn.PaidFeeOnL1)
		}
		coreTxns[i] = txn.Transaction
	}

	// Execute the transaction
	vmResults, err := b.vm.Execute(
		coreTxns,
		declaredClasses,
		paidFeesOnL1,
		&vm.BlockInfo{
			Header:                pending.Block.Header,
			BlockHashToBeRevealed: blockHashToBeRevealed,
		},
		state,
		b.bc.Network(),
		false, false, false, true, false)
	if err != nil {
		return err
	}

	// Handle declared classes for declare transactions
	for i, trace := range vmResults.Traces {
		if trace.StateDiff.DeclaredClasses != nil ||
			trace.StateDiff.DeprecatedDeclaredClasses != nil {
			if err := b.processClassDeclaration(&txns[i], &state); err != nil {
				return err
			}
		}
	}

	// Adapt results to core type (which use reference types)
	receipts := make([]*core.TransactionReceipt, len(txns))
	mergedStateDiff := vm2core.AdaptStateDiff(vmResults.Traces[0].StateDiff)
	for i, trace := range vmResults.Traces {
		adaptedStateDiff := vm2core.AdaptStateDiff(trace.StateDiff)
		mergedStateDiff.Merge(&adaptedStateDiff)
		adaptedReceipt := vm2core.Receipt(vmResults.OverallFees[i], txns[i].Transaction, &vmResults.Traces[i], &vmResults.Receipts[i])
		receipts[i] = &adaptedReceipt
	}

	// Update pending block with transaction results
	updatePendingBlock(pending, receipts, coreTxns, mergedStateDiff)

	return b.StorePending(pending)
}

// processClassDeclaration handles class declaration storage for declare transactions
func (b *Builder) processClassDeclaration(txn *mempool.BroadcastedTransaction, state *sync.PendingStateWriter) error {
	if t, ok := (txn.Transaction).(*core.DeclareTransaction); ok {
		if err := state.SetContractClass(t.ClassHash, txn.DeclaredClass); err != nil {
			b.log.Errorw("failed to set contract class", "err", err)
			return err
		}

		if t.CompiledClassHash != nil {
			if err := state.SetCompiledClassHash(t.ClassHash, t.CompiledClassHash); err != nil {
				b.log.Errorw("failed to SetCompiledClassHash", "err", err)
				return err
			}
		}
	}
	return nil
}

// updatePendingBlock updates the pending block with transaction results
func updatePendingBlock(
	pending *sync.Pending,
	receipts []*core.TransactionReceipt,
	transactions []core.Transaction,
	stateDiff core.StateDiff,
) {
	pending.Block.Receipts = append(pending.Block.Receipts, receipts...)
	pending.Block.Transactions = append(pending.Block.Transactions, transactions...)
	pending.Block.TransactionCount += uint64(len(transactions))
	for _, receipt := range receipts {
		pending.Block.EventCount += uint64(len(receipt.Events))
	}
	pending.StateUpdate.StateDiff.Merge(&stateDiff)
}

// StorePending stores a pending block given that it is for the next height
func (b *Builder) StorePending(newPending *sync.Pending) error {
	expectedParentHash := new(felt.Felt)
	h, err := b.bc.HeadsHeader()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	} else if err == nil {
		expectedParentHash = h.Hash
	}

	if !expectedParentHash.Equal(newPending.Block.ParentHash) {
		return fmt.Errorf("store pending: %w", blockchain.ErrParentDoesNotMatchHead)
	}
	b.pendingBlock.Store(newPending)
	return nil
}

// The builder has no reorg logic (centralised sequencer that can't reorg)
func (b *Builder) SubscribeReorg() sync.ReorgSubscription {
	return sync.ReorgSubscription{Subscription: b.subReorgFeed.Subscribe()}
}

func (b *Builder) SubscribeNewHeads() sync.NewHeadSubscription {
	return sync.NewHeadSubscription{Subscription: b.subNewHeads.Subscribe()}
}

func (b *Builder) SubscribePending() sync.PendingSubscription {
	return sync.PendingSubscription{Subscription: b.subPendingBlock.Subscribe()}
}
