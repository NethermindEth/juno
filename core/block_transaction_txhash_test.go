package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	_ "github.com/NethermindEth/juno/encoder/registry" // register transaction CBOR tags
	"github.com/stretchr/testify/require"
)

// allTypeTransactions returns one transaction of each type, each with a distinct hash and
// type-specific fields set so the encoding exercises the fields the projection must skip.
func allTypeTransactions() []core.Transaction {
	return []core.Transaction{
		&core.DeployTransaction{
			TransactionHash:     felt.NewFromUint64[felt.Felt](1),
			ContractAddressSalt: felt.NewFromUint64[felt.Felt](10),
			ContractAddress:     felt.NewFromUint64[felt.Felt](11),
			ClassHash:           felt.NewFromUint64[felt.Felt](12),
			ConstructorCallData: []felt.Felt{felt.FromUint64[felt.Felt](13), felt.FromUint64[felt.Felt](14)},
			Version:             felt.NewFromUint64[core.TransactionVersion](0),
		},
		&core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     felt.NewFromUint64[felt.Felt](2),
				ContractAddressSalt: felt.NewFromUint64[felt.Felt](20),
				ClassHash:           felt.NewFromUint64[felt.Felt](21),
				ConstructorCallData: []felt.Felt{felt.FromUint64[felt.Felt](22)},
				Version:             felt.NewFromUint64[core.TransactionVersion](3),
			},
			MaxFee: felt.NewFromUint64[felt.Felt](23),
			TransactionSignature: []felt.Felt{
				felt.FromUint64[felt.Felt](24), felt.FromUint64[felt.Felt](25),
			},
			Nonce:         felt.NewFromUint64[felt.Felt](26),
			Tip:           7,
			PaymasterData: []felt.Felt{felt.FromUint64[felt.Felt](27)},
		},
		&core.InvokeTransaction{
			TransactionHash: felt.NewFromUint64[felt.Felt](3),
			CallData: []felt.Felt{
				felt.FromUint64[felt.Felt](30), felt.FromUint64[felt.Felt](31),
			},
			TransactionSignature: []felt.Felt{felt.FromUint64[felt.Felt](32)},
			ContractAddress:      felt.NewFromUint64[felt.Felt](33),
			Version:              felt.NewFromUint64[core.TransactionVersion](3),
			Nonce:                felt.NewFromUint64[felt.Felt](34),
			SenderAddress:        felt.NewFromUint64[felt.Felt](35),
			Tip:                  9,
			PaymasterData:        []felt.Felt{felt.FromUint64[felt.Felt](36)},
		},
		&core.DeclareTransaction{
			TransactionHash:      felt.NewFromUint64[felt.Felt](4),
			ClassHash:            felt.NewFromUint64[felt.Felt](40),
			SenderAddress:        felt.NewFromUint64[felt.Felt](41),
			MaxFee:               felt.NewFromUint64[felt.Felt](42),
			TransactionSignature: []felt.Felt{felt.FromUint64[felt.Felt](43)},
			Nonce:                felt.NewFromUint64[felt.Felt](44),
			Version:              felt.NewFromUint64[core.TransactionVersion](3),
			CompiledClassHash:    felt.NewFromUint64[felt.Felt](45),
		},
		&core.L1HandlerTransaction{
			TransactionHash:    felt.NewFromUint64[felt.Felt](5),
			ContractAddress:    felt.NewFromUint64[felt.Felt](50),
			EntryPointSelector: felt.NewFromUint64[felt.Felt](51),
			Nonce:              felt.NewFromUint64[felt.Felt](52),
			CallData:           []felt.Felt{felt.FromUint64[felt.Felt](53), felt.FromUint64[felt.Felt](54)},
			Version:            felt.NewFromUint64[core.TransactionVersion](0),
		},
	}
}

// matchingReceipts returns a receipt per transaction, each carrying its transaction's hash as
// production always writes.
func matchingReceipts(transactions []core.Transaction) []*core.TransactionReceipt {
	receipts := make([]*core.TransactionReceipt, len(transactions))
	for i, transaction := range transactions {
		receipts[i] = &core.TransactionReceipt{
			Fee:             felt.NewFromUint64[felt.Felt](uint64(100 + i)),
			TransactionHash: transaction.Hash(),
			Events: []*core.Event{{
				From: felt.NewFromUint64[felt.Felt](1),
				Keys: []felt.Felt{felt.FromUint64[felt.Felt](2)},
				Data: []felt.Felt{felt.FromUint64[felt.Felt](3)},
			}},
		}
	}
	return receipts
}

func transactionHashesOf(transactions []core.Transaction) []felt.Felt {
	hashes := make([]felt.Felt, len(transactions))
	for i, transaction := range transactions {
		hashes[i] = *transaction.Hash()
	}
	return hashes
}

// transactionHashesOfBlock reads the hashes through the public partial-serializer path, the same
// route a DB read takes.
func transactionHashesOfBlock(
	t *testing.T,
	blockTransactions core.BlockTransactions,
) ([]felt.Felt, error) {
	t.Helper()
	data, err := core.BlockTransactionsSerializer{}.Marshal(&blockTransactions)
	require.NoError(t, err)

	var hashes []felt.Felt
	err = core.BlockTransactionsAllTransactionHashesPartialSerializer.UnmarshalPartial(
		struct{}{}, data, &hashes,
	)
	return hashes, err
}

// TestTransactionHashesAllTxTypes verifies that reading hashes from the transaction section returns
// the correct hash for a block containing every transaction type. The sampled real blocks used
// elsewhere are Deploy/Invoke-heavy; this closes the coverage gap for Declare, DeployAccount, and
// L1Handler, whose distinct field sets the union projection must cover.
func TestTransactionHashesAllTxTypes(t *testing.T) {
	transactions := allTypeTransactions()
	blockTransactions, err := core.NewBlockTransactions(
		transactions,
		matchingReceipts(transactions),
	)
	require.NoError(t, err)

	hashes, err := transactionHashesOfBlock(t, blockTransactions)
	require.NoError(t, err)
	require.Equal(t, transactionHashesOf(transactions), hashes)
}

// TestTransactionHashesMatchFullDecode pins the partial read to the full decode it replaces: the
// hashes must equal what the pre-optimisation handler produced from fully decoded transactions.
func TestTransactionHashesMatchFullDecode(t *testing.T) {
	transactions := allTypeTransactions()
	blockTransactions, err := core.NewBlockTransactions(
		transactions,
		matchingReceipts(transactions),
	)
	require.NoError(t, err)

	decoded, err := blockTransactions.Transactions().All()
	require.NoError(t, err)

	hashes, err := transactionHashesOfBlock(t, blockTransactions)
	require.NoError(t, err)
	require.Equal(t, transactionHashesOf(decoded), hashes)
}

// TestTransactionHashesRejectMissingHash guards the reused projection: a transaction stored without
// a hash decodes as a no-op into the value-typed felt, so without a reset it would silently return
// the preceding transaction's hash instead of failing.
func TestTransactionHashesRejectMissingHash(t *testing.T) {
	blockTransactions, err := core.NewBlockTransactions(
		[]core.Transaction{
			&core.InvokeTransaction{TransactionHash: felt.NewFromUint64[felt.Felt](111)},
			&core.InvokeTransaction{TransactionHash: nil},
		},
		[]*core.TransactionReceipt{
			{TransactionHash: felt.NewFromUint64[felt.Felt](111)},
			{TransactionHash: nil},
		},
	)
	require.NoError(t, err)

	hashes, err := transactionHashesOfBlock(t, blockTransactions)
	require.ErrorContains(t, err, "missing TransactionHash in transaction 1")
	require.Nil(t, hashes)
}

// TestTransactionHashesWithoutReceiptSection covers a block whose receipt section is absent: the
// transaction section then runs to the end of the data, and every hash must still be read.
func TestTransactionHashesWithoutReceiptSection(t *testing.T) {
	transactions := allTypeTransactions()
	blockTransactions, err := core.NewBlockTransactions(transactions, nil)
	require.NoError(t, err)
	require.Empty(t, blockTransactions.Indexes.Receipts)

	hashes, err := transactionHashesOfBlock(t, blockTransactions)
	require.NoError(t, err)
	require.Equal(t, transactionHashesOf(transactions), hashes)
}
