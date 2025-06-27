package builder

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
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
	NumTxnsToBatchExecute  = 10
	ErrPendingParentHash   = errors.New("pending block parent hash does not match chain head")
	CurrentStarknetVersion = semver.MustParse("0.14.0")
)

type BuildResult struct {
	Pending            sync.Pending
	ProposalCommitment types.ProposalCommitment
}

type Builder struct {
	// Builder dependencies
	vm           vm.VM
	blockchain   *blockchain.Blockchain
	headState    core.StateReader
	headCloser   blockchain.StateCloser
	log          utils.Logger
	disableFees  bool
	skipValidate bool // allows us to modify txn fields without having to re-calculate the signature etc

	// Builder state
	// TODO: move to a builder state struct
	pendingBlock      *sync.Pending
	l2GasConsumed     uint64
	revealedBlockHash *felt.Felt
}

func New(
	bc *blockchain.Blockchain,
	vm vm.VM,
	log utils.Logger,
	disableFees,
	skipValidate bool,
) Builder {
	return Builder{
		log:          log,
		blockchain:   bc,
		disableFees:  disableFees,
		skipValidate: skipValidate,
		vm:           vm,
	}
}

func (b *Builder) Finalise(pending *sync.Pending, signer utils.BlockSignFunc, privateKey *ecdsa.PrivateKey) error {
	return b.blockchain.Finalise(pending.Block, pending.StateUpdate, pending.NewClasses, signer)
}

func (b *Builder) Pending() (*sync.Pending, error) {
	if b.pendingBlock == nil {
		return nil, sync.ErrPendingBlockNotFound
	}
	expectedParentHash := &felt.Zero
	if head, err := b.blockchain.HeadsHeader(); err == nil {
		expectedParentHash = head.Hash
	}

	if b.pendingBlock.Block.ParentHash.Equal(expectedParentHash) {
		return b.pendingBlock, nil
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
	b.pendingBlock = &sync.Pending{}

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

	b.revealedBlockHash, err = b.getRevealedBlockHash(header.Number)
	if err != nil {
		return err
	}

	pendingBlock := core.Block{
		Header: &core.Header{
			ParentHash:       header.Hash,
			Number:           header.Number + 1,
			SequencerAddress: sequencerAddress,
			ProtocolVersion:  CurrentStarknetVersion.String(),
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
	b.pendingBlock = &pending
	b.headState, b.headCloser, err = b.blockchain.HeadState()
	return err
}

func (b *Builder) getRevealedBlockHash(blockHeight uint64) (*felt.Felt, error) {
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
func (b *Builder) RunTxns(txns []mempool.BroadcastedTransaction) (err error) {
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
			BlockHashToBeRevealed: b.revealedBlockHash,
		},
		state,
		b.blockchain.Network(),
		b.disableFees, b.skipValidate, false, true, false)
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
	b.pendingBlock = newPending
	return nil
}

func (b *Builder) ProposalInit(pInit *types.ProposalInit) error {
	header, err := b.blockchain.HeadsHeader()
	if err != nil {
		return err
	}
	if header.Number+1 != uint64(pInit.BlockNum) {
		return fmt.Errorf("proposed block number is not head.Number +1")
	}

	pendingBlock := core.Block{
		Header: &core.Header{
			Number:           uint64(pInit.BlockNum),
			SequencerAddress: &pInit.Proposer,
			ParentHash:       header.Hash,
			// Todo: we need a mapping of protocolversion to block versions from SN
			ProtocolVersion: CurrentStarknetVersion.String(),
			// Todo: once the spec is finalised, handle these fields (if they still exist)
			// OldStateRoot, VersionConstantCommitment, NextL2GasPriceFRI
			// Note: All values should be set to 0 according to the specs
			L1GasPriceETH:  felt.Zero.Clone(),
			L1GasPriceSTRK: felt.Zero.Clone(),
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: felt.Zero.Clone(),
				PriceInFri: felt.Zero.Clone(),
			},
			L2GasPrice: &core.GasPrice{
				PriceInWei: felt.Zero.Clone(),
				PriceInFri: felt.Zero.Clone(),
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
	b.pendingBlock = &pending
	b.headState, b.headCloser, err = b.blockchain.HeadState()
	return err
}

func (b *Builder) SetBlockInfo(blockInfo *types.BlockInfo) {
	b.pendingBlock.Block.Header.Number = blockInfo.BlockNumber
	b.pendingBlock.Block.Header.SequencerAddress = &blockInfo.Builder
	b.pendingBlock.Block.Header.Timestamp = blockInfo.Timestamp
	b.pendingBlock.Block.Header.L2GasPrice.PriceInFri = &blockInfo.L2GasPriceFRI
	b.pendingBlock.Block.Header.L2GasPrice.PriceInWei = new(felt.Felt).Div(&blockInfo.L2GasPriceFRI, &blockInfo.EthToStrkRate)
	b.pendingBlock.Block.Header.L1GasPriceETH = &blockInfo.L1GasPriceWEI
	b.pendingBlock.Block.Header.L1GasPriceSTRK = new(felt.Felt).Mul(&blockInfo.L1GasPriceWEI, &blockInfo.EthToStrkRate)
	b.pendingBlock.Block.Header.L1DataGasPrice.PriceInWei = &blockInfo.L1DataGasPriceWEI
	b.pendingBlock.Block.Header.L1DataGasPrice.PriceInFri = new(felt.Felt).Mul(&blockInfo.L1DataGasPriceWEI, &blockInfo.EthToStrkRate)
	b.pendingBlock.Block.Header.L1DAMode = blockInfo.L1DAMode
}

func (b *Builder) Finish() (BuildResult, error) {
	pending, err := b.Pending()
	if err != nil {
		return BuildResult{}, err
	}

	simulatedResult, err := b.blockchain.Simulate(pending.Block, pending.StateUpdate, pending.NewClasses, nil)
	if err != nil {
		return BuildResult{}, err
	}

	if simulatedResult.ConcatCount.IsZero() {
		simulatedResult.BlockCommitments = &core.BlockCommitments{
			TransactionCommitment: new(felt.Felt).SetUint64(0),
			EventCommitment:       new(felt.Felt).SetUint64(0),
			ReceiptCommitment:     new(felt.Felt).SetUint64(0),
			StateDiffCommitment:   new(felt.Felt).SetUint64(0),
		}
	}

	version, err := semver.NewVersion(pending.Block.ProtocolVersion)
	if err != nil {
		return BuildResult{}, err
	}

	// Todo: we ignore some values until the spec is Finalised: VersionConstantCommitment, NextL2GasPriceFRI
	buildResult := BuildResult{
		Pending: *pending,
		ProposalCommitment: types.ProposalCommitment{
			BlockNumber:           pending.Block.Number,
			Builder:               *pending.Block.SequencerAddress,
			ParentCommitment:      *pending.Block.ParentHash,
			Timestamp:             pending.Block.Timestamp,
			ProtocolVersion:       *version,
			OldStateRoot:          *pending.StateUpdate.OldRoot,
			StateDiffCommitment:   *simulatedResult.BlockCommitments.StateDiffCommitment,
			TransactionCommitment: *simulatedResult.BlockCommitments.TransactionCommitment,
			EventCommitment:       *simulatedResult.BlockCommitments.EventCommitment,
			ReceiptCommitment:     *simulatedResult.BlockCommitments.ReceiptCommitment,
			ConcatenatedCounts:    simulatedResult.ConcatCount,
			L1GasPriceFRI:         *pending.Block.L1GasPriceSTRK,
			L1DataGasPriceFRI:     *pending.Block.L1DataGasPrice.PriceInFri,
			L2GasPriceFRI:         *pending.Block.L2GasPrice.PriceInFri,
			L2GasUsed:             *new(felt.Felt).SetUint64(b.l2GasConsumed),
			L1DAMode:              pending.Block.L1DAMode,
		},
	}

	return buildResult, nil
}
