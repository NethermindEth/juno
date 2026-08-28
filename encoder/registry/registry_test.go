package registry_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/encoder/cborlite"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A tagged type is only reachable through a field whose static type is an interface.
type transactionHolder struct{ Txn core.Transaction }

func registeredTransactions(hash *felt.Felt) []core.Transaction {
	return []core.Transaction{
		&core.InvokeTransaction{TransactionHash: hash},
		&core.DeclareTransaction{TransactionHash: hash},
		&core.DeployTransaction{TransactionHash: hash},
		&core.L1HandlerTransaction{TransactionHash: hash},
		&core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{TransactionHash: hash},
		},
	}
}

// TestBothDecodersLearnTheSameTags fails if a registered type reaches only the generic
// decoder. A tag cborlite does not know makes it decline, which is silent on its own: the
// caller falls back and only the clock says anything.
func TestBothDecodersLearnTheSameTags(t *testing.T) {
	hash := felt.NewFromUint64[felt.Felt](7)

	// One type that cborlite reads end to end is enough to prove the tag arrived, since
	// an unregistered one declines before reading a single field.
	data, err := encoder.Marshal(transactionHolder{Txn: &core.InvokeTransaction{
		TransactionHash: hash,
	}})
	require.NoError(t, err)

	var got transactionHolder
	require.NoError(t, cborlite.Unmarshal(data, &got),
		"core.InvokeTransaction is registered with the generic decoder but not with cborlite")
	require.IsType(t, &core.InvokeTransaction{}, got.Txn)
	assert.Equal(t, hash.String(), got.Txn.Hash().String())
}

// TestNoRegisteredTransactionDecodesWrong is the invariant that matters for the fallback:
// cborlite may decline a type, but it must never report success and hand back something
// else. An embedded field used to do exactly that, decoding to a zero value in silence.
func TestNoRegisteredTransactionDecodesWrong(t *testing.T) {
	hash := felt.NewFromUint64[felt.Felt](7)

	for _, txn := range registeredTransactions(hash) {
		data, err := encoder.Marshal(transactionHolder{Txn: txn})
		require.NoError(t, err)

		var got transactionHolder
		if err := cborlite.Unmarshal(data, &got); err != nil {
			// Declining is fine, the caller falls back to the generic decoder.
			continue
		}
		assert.Equal(t, txn, got.Txn, "%T decoded without an error but came back wrong", txn)
	}
}
