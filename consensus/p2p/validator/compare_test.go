package validator

import (
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func TestCompareFeltField(t *testing.T) {
	name := "test-field"

	notExpected := new(felt.Felt).SetUint64(12345)
	expected := new(felt.Felt).SetUint64(67890)
	computed := new(felt.Felt).SetUint64(67890)

	t.Run("EqualFields", func(t *testing.T) {
		err := compareFeltField(name, expected, computed)
		if err != nil {
			t.Errorf("expected no error for equal fields, got: %v", err)
		}
	})

	t.Run("UnequalFields", func(t *testing.T) {
		err := compareFeltField(name, notExpected, computed)
		if err == nil {
			t.Errorf("expected error for unequal fields, got nil")
		} else {
			expected := fmt.Sprintf("%s commitment mismatch: received=%s computed=%s", name, notExpected, computed)
			if err.Error() != expected {
				t.Errorf("unexpected error message: got %q, want %q", err.Error(), expected)
			}
		}
	})
}

// TODO: Write tests to actually test the `ProposalCommitment` function.
func TestCompareProposalCommitment(t *testing.T) {
	proposer := felt.NewUnsafeFromString[felt.Felt]("1")

	stateDiffCommitment := new(felt.Felt).SetUint64(1)
	transactionCommitment := new(felt.Felt).SetUint64(2)
	eventCommitment := new(felt.Felt).SetUint64(3)
	receiptCommitment := new(felt.Felt).SetUint64(4)

	blockNumber := uint64(100)

	newDefaultProposalCommitment := func() *types.ProposalCommitment {
		return &types.ProposalCommitment{
			BlockNumber:           blockNumber,
			ParentCommitment:      *new(felt.Felt).SetUint64(111),
			Builder:               *proposer,
			Timestamp:             1000,
			ProtocolVersion:       *builder.CurrentStarknetVersion,
			ConcatenatedCounts:    *new(felt.Felt).SetUint64(1),
			StateDiffCommitment:   *stateDiffCommitment,
			TransactionCommitment: *transactionCommitment,
			EventCommitment:       *eventCommitment,
			ReceiptCommitment:     *receiptCommitment,
			L1DAMode:              core.Blob,
		}
	}

	h := &core.Header{
		Number:           blockNumber,
		ParentHash:       felt.NewUnsafeFromString[felt.Felt]("111"),
		SequencerAddress: proposer,
		Timestamp:        1000,
		ProtocolVersion:  builder.CurrentStarknetVersion.String(),
		L1DAMode:         core.Blob,
	}
	t.Run("ValidCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		err := compareProposalCommitment(expected, p)
		require.NoError(t, err)
	})

	t.Run("MismatchedBlockNumber", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.BlockNumber = blockNumber + 1
		err := compareProposalCommitment(expected, p)
		expectedErr := fmt.Sprintf("block number mismatch: received=%d computed=%d", p.BlockNumber, h.Number)
		require.EqualError(t, err, expectedErr)
		p.BlockNumber = blockNumber
	})

	t.Run("MismatchedParentCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.ParentCommitment = *felt.NewUnsafeFromString[felt.Felt]("222")
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("FutureTimestamp", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.Timestamp = 2000
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("UnsupportedProtocolVersion", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.ProtocolVersion = *semver.New(0, 123, 0, "", "")
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedStateDiffCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.StateDiffCommitment = *other
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedTransactionCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.TransactionCommitment = *other
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedEventCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.EventCommitment = *other
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedReceiptCommitment", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		other := &felt.Felt{}
		other.SetUint64(99)
		p.ReceiptCommitment = *other
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedConcatenatedCounts", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.ConcatenatedCounts = *felt.NewUnsafeFromString[felt.Felt]("2")
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL1GasPriceFRI", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L1GasPriceFRI = felt.FromUint64[felt.Felt](3)
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL1DataGasPriceFRI", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L1DataGasPriceFRI = felt.FromUint64[felt.Felt](4)
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL2GasPriceFRI", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L2GasPriceFRI = felt.FromUint64[felt.Felt](5)
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL2GasUsed", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L2GasUsed = felt.FromUint64[felt.Felt](6)
		require.Error(t, compareProposalCommitment(expected, p))
	})

	t.Run("MismatchedL1DAMode", func(t *testing.T) {
		p := newDefaultProposalCommitment()
		expected := newDefaultProposalCommitment()
		p.L1DAMode = core.Calldata
		require.Error(t, compareProposalCommitment(expected, p))
	})
}
