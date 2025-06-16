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

var ErrProposalFinMismatch = fmt.Errorf("proposal fin commitment doesn't match that generated locally")

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
	builder     *builder.Builder // Builder manages the pending block and state
	commitments *core.BlockCommitments
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](builder *builder.Builder) Validator[V, H, A] {
	return &validator[V, H, A]{
		builder: builder,
	}
}

// ProposalInit initialises the pending header according to ProposalInit
func (v *validator[V, H, A]) ProposalInit(pInit *types.ProposalInit) error {
	return v.builder.ProposalInit(pInit)
}

// BlockInfo sets the pending header according to BlockInfo
func (v *validator[V, H, A]) BlockInfo(blockInfo *types.BlockInfo) {
	v.builder.SetBlockInfo(blockInfo)
}

// TransactionBatch executes the provided transactions, and stores the result in the pending state
func (v *validator[V, H, A]) TransactionBatch(txns []types.Transaction) error {
	txnsToExecute := make([]mempool.BroadcastedTransaction, len(txns))
	for i := range txnsToExecute {
		txnsToExecute[i] = mempool.BroadcastedTransaction{
			Transaction:   txns[i].Transaction,
			DeclaredClass: txns[i].Class,
			PaidFeeOnL1:   txns[i].PaidFeeOnL1,
		}
	}

	if err := v.builder.ExecuteTxns(txnsToExecute); err != nil {
		return err
	}

	return nil
}

// ProposalCommitment checks the set of proposed commitments against those generated locally.
func (v *validator[V, H, A]) ProposalCommitment(proCom *types.ProposalCommitment) error {
	commitments, concatCount, err := v.builder.ExecutePending()
	if err != nil {
		return err
	}
	// Starknet consensus requires zero values for empty blocks
	if concatCount.IsZero() {
		commitments = &core.BlockCommitments{
			TransactionCommitment: new(felt.Felt).SetUint64(0),
			EventCommitment:       new(felt.Felt).SetUint64(0),
			ReceiptCommitment:     new(felt.Felt).SetUint64(0),
			StateDiffCommitment:   new(felt.Felt).SetUint64(0),
		}
	}
	pendingBlock := v.builder.PendingBlock()
	v.commitments = commitments
	if err := compareProposalCommitment(proCom, pendingBlock.Header, commitments, concatCount); err != nil {
		return err
	}
	return nil
}

// ProposalFin executes the provided transactions, and stores the result in the pending state
func (v *validator[V, H, A]) ProposalFin(proposalFin types.ProposalFin) error {
	pendingBlock := v.builder.PendingBlock()

	proposerCommitmentFelt := felt.Felt(proposalFin)
	if !proposerCommitmentFelt.Equal(pendingBlock.Hash) {
		return ErrProposalFinMismatch
	}
	return nil
}

// Todo: the validator interface assumes that the msgs are prevalidated before it is called.
// This is an issue because proto3 may leave messages empty, whereas starknet requires them
// to be present
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

	// Todo: ask the SN guys about the precise checks we should perform with the timestamps
	if p.Timestamp > h.Timestamp {
		return fmt.Errorf("invalid timestamp: proposal timestamp (%d) is later than header timestamp (%d)", p.Timestamp, h.Timestamp)
	}

	if !p.ProtocolVersion.LessThanEqual(blockchain.SupportedStarknetVersion) {
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
