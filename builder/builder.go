package builder

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
)

var (
	NumTxnsToBatchExecute = 10
	ErrPendingParentHash  = errors.New("pending block parent hash does not match chain head")
)

type Builder struct {
	pendingBlock  atomic.Pointer[sync.Pending]
	vm            vm.VM
	l2GasConsumed uint64
	blockchain    *blockchain.Blockchain
	headState     core.StateReader
	headCloser    blockchain.StateCloser
	log           utils.Logger
	disableFees   bool
}

func New(
	bc *blockchain.Blockchain,
	vm vm.VM,
	log utils.Logger,
	disableFees bool,
) Builder {
	return Builder{
		log:         log,
		blockchain:  bc,
		disableFees: disableFees,
		vm:          vm,
	}
}

func (b *Builder) L2GasConsumed() uint64 {
	return b.l2GasConsumed
}

func (b *Builder) Finalise(pending *sync.Pending, signer utils.BlockSignFunc, privateKey *ecdsa.PrivateKey) error {
	return b.blockchain.Finalise(pending.Block, pending.StateUpdate, pending.NewClasses, signer)
}

func (b *Builder) Pending() (*sync.Pending, error) {
	p := b.pendingBlock.Load()
	if p == nil {
		return nil, sync.ErrPendingBlockNotFound
	}
	expectedParentHash := &felt.Zero
	if head, err := b.blockchain.HeadsHeader(); err == nil {
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
	pending, err := b.Pending()
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove the state closer once we refactor the state
	return sync.NewPendingState(pending.StateUpdate.StateDiff, pending.NewClasses, b.headState), func() error { return nil }, nil
}

func (b *Builder) ClearPending() error {
	b.l2GasConsumed = 0
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

func (b *Builder) InitPendingBlock(sequencerAddress *felt.Felt) error {
	header, err := b.blockchain.HeadsHeader()
	if err != nil {
		return err
	}
	pendingBlock := core.Block{
		Header: &core.Header{
			ParentHash:       header.Hash,
			Number:           header.Number + 1,
			SequencerAddress: sequencerAddress,
			ProtocolVersion:  blockchain.SupportedStarknetVersion.String(),
			L1GasPriceETH:    felt.One.Clone(),
			L1GasPriceSTRK:   felt.One.Clone(),
			L1DAMode:         core.Calldata,
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: felt.One.Clone(),
				PriceInFri: felt.One.Clone(),
			},
			L2GasPrice: &core.GasPrice{
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
	b.headState, b.headCloser, err = b.blockchain.HeadState()
	return err
}

func (b *Builder) GetRevealedBlockHash() (*felt.Felt, error) {
	blockHeight, err := b.blockchain.Height()
	if err != nil {
		return nil, err
	}
	const blockHashLag = 10
	if blockHeight < blockHashLag {
		return nil, nil
	}

	header, err := b.blockchain.BlockHeaderByNumber(blockHeight - blockHashLag)
	if err != nil {
		return nil, err
	}
	return header.Hash, nil
}

// RunTxns executes the provided transaction and applies the state changes
// to the pending state
func (b *Builder) RunTxns(txns []mempool.BroadcastedTransaction, blockHashToBeRevealed *felt.Felt) (err error) {
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
		b.blockchain.Network(),
		b.disableFees, false, false, true, false)
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

	for i := range vmResults.GasConsumed {
		b.l2GasConsumed += vmResults.GasConsumed[i].L2Gas
	}
	return b.storePending(pending)
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
func (b *Builder) storePending(newPending *sync.Pending) error {
	expectedParentHash := new(felt.Felt)
	h, err := b.blockchain.HeadsHeader()
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

func (b *Builder) ProposalInit(pInit *types.ProposalInit) error {
	header, err := b.blockchain.HeadsHeader()
	if err != nil {
		return err
	}
	if header.Number+1 != pInit.BlockNum {
		return fmt.Errorf("proposed block number is not head.Number +1")
	}

	pendingBlock := core.Block{
		Header: &core.Header{
			Number:           pInit.BlockNum,
			SequencerAddress: &pInit.Proposer,
			ParentHash:       header.Hash,
			// Todo: we need a mapping of protocolversion to block versions from SN
			ProtocolVersion: blockchain.SupportedStarknetVersion.String(),
			// Todo: once the spec is finalised, handle these fields (if they still exist)
			// OldStateRoot, VersionConstantCommitment, NextL2GasPriceFRI
			// Note: we use the header values by default, since the proposer only
			// sends over a subset of the gas prices (eg for L1DataGasPrice it
			// only sends the L1DataGasPriceWEI, but not the price in FRI, but
			// we need both for the block hash)
			L1GasPriceETH:  header.L1GasPriceETH,
			L1GasPriceSTRK: header.L1GasPriceSTRK,
			L1DataGasPrice: header.L1DataGasPrice,
			L2GasPrice:     header.L2GasPrice,
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
	b.headState, b.headCloser, err = b.blockchain.HeadState()
	return err
}

func (b *Builder) SetBlockInfo(blockInfo *types.BlockInfo) {
	pending := b.pendingBlock.Load()
	pending.Block.Header.Number = blockInfo.BlockNumber
	pending.Block.Header.SequencerAddress = &blockInfo.Builder
	pending.Block.Header.Timestamp = blockInfo.Timestamp
	pending.Block.Header.L2GasPrice.PriceInFri = &blockInfo.L2GasPriceFRI
	pending.Block.Header.L1GasPriceETH = &blockInfo.L1GasPriceWEI
	pending.Block.Header.L1DataGasPrice.PriceInWei = &blockInfo.L1DataGasPriceWEI
	pending.Block.Header.L1DAMode = blockInfo.L1DAMode
	b.pendingBlock.Store(pending)
}

func (b *Builder) ExecuteTxns(txns []mempool.BroadcastedTransaction) error {
	b.log.Debugw("calling ExecuteTxns")
	blockHashToBeRevealed, err := b.GetRevealedBlockHash()
	if err != nil {
		return err
	}

	if err := b.RunTxns(txns, blockHashToBeRevealed); err != nil {
		b.log.Debugw("failed running txn", "err", err.Error())
		return err
	}
	b.log.Debugw("running txns success")
	return nil
}

// ExecutePending updates the pending block and state-update
func (b *Builder) ExecutePending() (*core.BlockCommitments, *felt.Felt, error) {
	pending, err := b.Pending()
	if err != nil {
		return nil, nil, err
	}
	simulateResult, err := b.blockchain.Simulate(pending.Block, pending.StateUpdate, pending.NewClasses, nil)
	b.pendingBlock.Store(pending)
	return simulateResult.BlockCommitments, &simulateResult.ConcatCount, err
}

// StoredExecutedPending stores the executed pending block
func (b *Builder) StoredExecutedPending(commitments *core.BlockCommitments) error {
	pending, err := b.Pending()
	if err != nil {
		return err
	}
	return b.blockchain.StoreSimulated(pending.Block, pending.StateUpdate, pending.NewClasses, commitments, nil)
}
