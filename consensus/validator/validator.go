package validator

import (
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
)

const defaultTxnPoolSize int = 1024

// Validator is used to validate new proposals. There are two potential flows, and functions must be called in order:
// Flow 1) Non-empty proposal - ProposalInit(), BlockInfo(), TransactionBatch(), ProposalCommitment(), ProposalFin()
// Flow 2) Empty proposal - ProposalInit(), ProposalCommitment(), ProposalFin()
//
//go:generate mockgen -destination=../mocks/mock_validator.go -package=mocks github.com/NethermindEth/juno/consensus/validator Validator
type Validator[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// ProposalInit initialises the pending state according to the proposal
	ProposalInit(pInit *types.ProposalInit) error

	// BlockInfo sets the pending header according to the proposal header
	BlockInfo(blockInfo *types.BlockInfo)

	// TransactionBatch executes the provided transactions, and stores the result in the pending state
	TransactionBatch(txn []types.Transaction) error

	// ProposalCommitment checks the set of proposed commitments against those generated locally.
	ProposalCommitment(proposalCommitment *types.ProposalCommitment) error

	// ProposalFin compares the provided commitment with that generated locally
	ProposalFin(proposalFin types.ProposalFin) error
}

type validator[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	builder           *builder.Builder    // Builder manages the pending block and state
	txnPool           []types.Transaction // Txns may not arrive in order
	latestExecutedTxn int                 // Tracks which txn we should execute next
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](builder *builder.Builder) Validator[V, H, A] {
	return &validator[V, H, A]{
		builder: builder,
		txnPool: make([]types.Transaction, defaultTxnPoolSize),
	}
}

// ProposalInit initialises the pending header according to ProposalInit
func (a *validator[V, H, A]) ProposalInit(pInit *types.ProposalInit) error {
	// reset txnPool
	for i := range a.latestExecutedTxn {
		a.txnPool[i] = types.Transaction{}
	}
	a.latestExecutedTxn = 0

	return a.builder.ProposalInit(pInit)
}

// BlockInfo sets the pending header according to BlockInfo
func (a *validator[V, H, A]) BlockInfo(blockInfo *types.BlockInfo) {
	a.builder.SetBlockInfo(blockInfo)
}

// TransactionBatch executes the provided transactions, and stores the result in the pending state
func (a *validator[V, H, A]) TransactionBatch(txns []types.Transaction) error {
	// Resize txnPool if necessary
	maxID := 0
	for i := range txns {
		if txns[i].Index > maxID {
			maxID = txns[i].Index
		}
	}
	if maxID >= len(a.txnPool) {
		newPool := make([]types.Transaction, 2*defaultTxnPoolSize)
		copy(newPool, a.txnPool)
		a.txnPool = newPool
	}

	// Insert txns into the pool
	for i := range txns {
		a.txnPool[txns[i].Index] = txns[i]
	}

	// Get set of txns that we can execute
	start, end := a.latestExecutedTxn, a.latestExecutedTxn
	for a.txnPool[end+1].Transaction != nil {
		end++
	}

	txnsToExecute := make([]mempool.BroadcastedTransaction, end-start+1)
	for i := range txnsToExecute {
		txnsToExecute[i] = mempool.BroadcastedTransaction{
			Transaction:   txns[start].Transaction,
			DeclaredClass: txns[start].Class,
			PaidFeeOnL1:   txns[start].PaidFeeOnL1,
		}
	}

	if err := a.builder.ExecuteTxns(txnsToExecute); err != nil {
		return err
	}
	a.latestExecutedTxn = end + 1
	return nil
}

// ProposalCommitment checks the set of proposed commitments against those generated locally.
func (a *validator[V, H, A]) ProposalCommitment(proCom *types.ProposalCommitment) error {
	commitments, concatCount, err := a.builder.ExecutePending()
	if err != nil {
		return err
	}
	pendingBlock := a.builder.PendingBlock()

	if err := compareProposalCommitment(proCom, pendingBlock.Header, commitments, concatCount); err != nil {
		return err
	}
	return nil
}

// ProposalFin executes the provided transactions, and stores the result in the pending state
func (a *validator[V, H, A]) ProposalFin(proposalFin types.ProposalFin) error {
	pendingBlock := a.builder.PendingBlock()

	proposerCommitmentFelt := felt.Felt(proposalFin)
	if !proposerCommitmentFelt.Equal(pendingBlock.Hash) {
		return fmt.Errorf("proposal fin: commitments do not match: expected=%s actual=%s",
			proposerCommitmentFelt.String(), pendingBlock.Hash.String())
	}
	return a.builder.Finalise(nil)
}

func compareFeltField(name string, a, b *felt.Felt) error {
	if a.Equal(b) {
		return nil
	}
	return fmt.Errorf("%s commitment mismatch: proposal=%s commitments=%s", name, a, b)
}

// Todo: there are fields in ProposalCommitment that we don't check against. Some of these fields
// will be dropped in the finalised spec, so I don't think we should worry about them until then
//  1. Some fields we can't get / compute: VersionConstantCommitment, NextL2GasPriceFRI
//  2. The gas prices. Currently the spec sets eth gas prices, but in v1, these will be dropped
//     for fri prices.
func compareProposalCommitment(
	p *types.ProposalCommitment,
	h *core.Header,
	c *core.BlockCommitments,
	concatCount *felt.Felt,
) error {
	if p.BlockNumber != h.Number {
		return fmt.Errorf("block number mismatch: proposal=%d header=%d", p.BlockNumber, h.Number)
	}

	if !p.ParentCommitment.Equal(h.ParentHash) {
		return fmt.Errorf("parent hash mismatch: proposal=%s header=%s", p.ParentCommitment.String(), h.ParentHash.String())
	}

	if err := compareFeltField("proposer address", &p.Builder, h.SequencerAddress); err != nil {
		return err
	}

	if p.Timestamp > h.Timestamp {
		return fmt.Errorf("invalid timestamp: proposal timestamp (%d) is later than header timestamp (%d)", p.Timestamp, h.Timestamp)
	}

	if !p.ProtocolVersion.Equal(blockchain.SupportedStarknetVersion) {
		return fmt.Errorf("protocol version mismatch: proposal=%s header=%s", p.ProtocolVersion, h.ProtocolVersion)
	}

	if err := compareFeltField("concat counts", &p.ConcatenatedCounts, concatCount); err != nil {
		return err
	}

	if err := compareFeltField("state diff", &p.StateDiffCommitment, c.StateDiffCommitment); err != nil {
		return err
	}
	if err := compareFeltField("transaction", &p.TransactionCommitment, c.TransactionCommitment); err != nil {
		return err
	}
	if err := compareFeltField("event", &p.EventCommitment, c.EventCommitment); err != nil {
		return err
	}
	if err := compareFeltField("receipt", &p.ReceiptCommitment, c.ReceiptCommitment); err != nil {
		return err
	}

	if p.L1DAMode != h.L1DAMode {
		return fmt.Errorf("L1 DA mode mismatch: proposal=%d header=%d", p.L1DAMode, h.L1DAMode)
	}

	return nil
}
