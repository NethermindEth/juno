package rpc2core_test

import (
	"cmp"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/adapters/rpc2core"
	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixtureBlock struct {
	network *networks.Network
	number  uint64
}

func (f fixtureBlock) String() string {
	return f.network.String() + " block " + strconv.FormatUint(f.number, 10)
}

var (
	mainnet0                  = fixtureBlock{&networks.Mainnet, 0}
	mainnet2889               = fixtureBlock{&networks.Mainnet, 2889}
	mainnet16697              = fixtureBlock{&networks.Mainnet, 16697}
	mainnet763497             = fixtureBlock{&networks.Mainnet, 763497}
	goerli485004              = fixtureBlock{&networks.Goerli, 485004}
	integration283364         = fixtureBlock{&networks.Integration, 283364}
	integration330363         = fixtureBlock{&networks.Integration, 330363}
	sepolia56377              = fixtureBlock{&networks.Sepolia, 56377}
	sepolia4072139            = fixtureBlock{&networks.Sepolia, 4072139}
	sepoliaIntegration35749   = fixtureBlock{&networks.SepoliaIntegration, 35749}
	sepoliaIntegration64164   = fixtureBlock{&networks.SepoliaIntegration, 64164}
	sepoliaIntegration1164608 = fixtureBlock{&networks.SepoliaIntegration, 1164608}
)

func coreBlock(t *testing.T, fixture fixtureBlock) *core.Block {
	t.Helper()
	block, err := feederGateway(t, fixture.network).BlockByNumber(t.Context(), fixture.number)
	require.NoError(t, err)
	return block
}

// serveBlock builds the response Handler.BlockWithReceipts serves for a block.
func serveBlock(block *core.Block) *rpcv10.BlockWithReceipts {
	transactions := make([]rpcv10.TransactionWithReceipt, len(block.Transactions))
	for i, transaction := range block.Transactions {
		served := rpcv10.AdaptTransaction(transaction, false)
		served.Hash = nil
		transactions[i] = rpcv10.TransactionWithReceipt{
			Transaction: served,
			Receipt:     rpcv10.AdaptReceipt(block.Receipts[i], transaction, rpcv10.TxnAcceptedOnL2),
		}
	}
	return &rpcv10.BlockWithReceipts{
		Status:       rpcv10.BlockAcceptedL2,
		BlockHeader:  rpcv10.AdaptBlockHeader(block.Header, &core.BlockCommitments{}),
		Transactions: transactions,
	}
}

func servedBlock(t *testing.T, block *core.Block) *rpcv10.BlockWithReceipts {
	t.Helper()
	return overWire(t, serveBlock(block))
}

// expectedBlock is what a client recovers from a served block. The wire format carries no
// consensus signatures, no L1->L2 message, no proof facts unless requested and no execution
// resources beyond the gas totals; the server serves an absent sequencer as zero, absent gas
// prices as one, an absent L1 handler nonce as zero, no nonce for version 0 transactions and
// the fee unit of the transaction version.
func expectedBlock(block *core.Block) *core.Block {
	header := *block.Header
	header.Signatures = [][]*felt.Felt{}
	header.SequencerAddress = cmp.Or(header.SequencerAddress, &felt.Zero)
	header.L1GasPriceETH = cmp.Or(header.L1GasPriceETH, &felt.One)
	header.L1GasPriceSTRK = cmp.Or(header.L1GasPriceSTRK, &felt.One)
	header.L1DataGasPrice = servedGasPrice(header.L1DataGasPrice)
	header.L2GasPrice = servedGasPrice(header.L2GasPrice)

	transactions := make([]core.Transaction, len(block.Transactions))
	receipts := make([]*core.TransactionReceipt, len(block.Receipts))
	for i, transaction := range block.Transactions {
		transactions[i] = expectedTransaction(transaction)
		receipts[i] = expectedReceipt(block.Receipts[i], transaction)
	}
	return &core.Block{Header: &header, Transactions: transactions, Receipts: receipts}
}

func servedGasPrice(price *core.GasPrice) *core.GasPrice {
	if price == nil {
		return &core.GasPrice{PriceInWei: &felt.One, PriceInFri: &felt.One}
	}
	return &core.GasPrice{
		PriceInWei: cmp.Or(price.PriceInWei, &felt.One),
		PriceInFri: cmp.Or(price.PriceInFri, &felt.One),
	}
}

func expectedTransaction(transaction core.Transaction) core.Transaction {
	switch v := transaction.(type) {
	case *core.InvokeTransaction:
		expected := *v
		expected.ProofFacts = nil
		if v.Version.Is(0) {
			expected.Nonce = nil
		}
		return &expected
	case *core.DeclareTransaction:
		expected := *v
		if v.Version.Is(0) {
			expected.Nonce = nil
		}
		return &expected
	case *core.L1HandlerTransaction:
		expected := *v
		expected.Nonce = cmp.Or(v.Nonce, &felt.Zero)
		return &expected
	default:
		return transaction
	}
}

func expectedReceipt(
	receipt *core.TransactionReceipt,
	transaction core.Transaction,
) *core.TransactionReceipt {
	expected := *receipt
	expected.L1ToL2Message = nil
	expected.ExecutionResources = &core.ExecutionResources{
		TotalGasConsumed: servedGasConsumed(receipt.ExecutionResources),
	}
	if transaction.TxVersion().Is(3) {
		expected.FeeUnit = core.STRK
	}
	return &expected
}

func servedGasConsumed(resources *core.ExecutionResources) *core.GasConsumed {
	if resources == nil || resources.TotalGasConsumed == nil {
		return &core.GasConsumed{}
	}
	return resources.TotalGasConsumed
}

func assertBlockRoundTrip(t *testing.T, network *networks.Network, block *core.Block) {
	t.Helper()
	adapted, err := rpc2core.AdaptBlock(servedBlock(t, block), network)
	require.NoError(t, err)
	assert.Equal(t, expectedBlock(block), adapted)
	require.NoError(t, core.VerifyTransactions(adapted.Transactions, network, adapted.ProtocolVersion))
}

func TestAdaptBlock(t *testing.T) {
	fixtures := []fixtureBlock{
		mainnet0,                  // deploy and invoke v0 without a protocol version or gas prices
		mainnet2889,               // declare v0 and L1 handler
		goerli485004,              // deploy v1, deploy account v1 and invoke v1
		mainnet16697,              // declare v1
		integration283364,         // declare v2
		integration330363,         // invoke v3 whose l1_data_gas bound was absent when submitted
		mainnet763497,             // invoke v3 without l1_data_gas next to an L1 handler
		sepoliaIntegration35749,   // deploy account v3
		sepoliaIntegration64164,   // v3 with an l1_data_gas bound
		sepolia56377,              // reverted transactions
		sepolia4072139,            // proof facts
		sepoliaIntegration1164608, // no transactions
	}

	for _, fixture := range fixtures {
		t.Run(fixture.String(), func(t *testing.T) {
			assertBlockRoundTrip(t, fixture.network, coreBlock(t, fixture))
		})
	}
}

// singleTransactionBlock wraps a transaction in a block whose header has every field the
// wire format requires, for transactions that only exist as standalone fixtures.
func singleTransactionBlock(transaction core.Transaction) *core.Block {
	receipts := []*core.TransactionReceipt{{
		Fee:                &felt.One,
		FeeUnit:            core.STRK,
		Events:             []*core.Event{},
		ExecutionResources: &core.ExecutionResources{TotalGasConsumed: &core.GasConsumed{}},
		L2ToL1Message:      []*core.L2ToL1Message{},
		TransactionHash:    transaction.Hash(),
	}}
	return &core.Block{
		Header: &core.Header{
			Hash:             &felt.One,
			ParentHash:       &felt.Zero,
			Number:           1,
			GlobalStateRoot:  &felt.Zero,
			SequencerAddress: &felt.One,
			TransactionCount: 1,
			ProtocolVersion:  "0.13.4",
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    &felt.One,
			L1GasPriceSTRK:   &felt.One,
			L1DAMode:         core.Blob,
			L1DataGasPrice:   &core.GasPrice{PriceInWei: &felt.One, PriceInFri: &felt.One},
			L2GasPrice:       &core.GasPrice{PriceInWei: &felt.One, PriceInFri: &felt.One},
			Signatures:       [][]*felt.Felt{},
		},
		Transactions: []core.Transaction{transaction},
		Receipts:     receipts,
	}
}

// No block fixture holds a declare v3, so the only one Juno can verify is adapted on its own.
// It is read from disk because the client method that fetches it is deprecated.
func TestAdaptDeclareV3Transaction(t *testing.T) {
	network := &networks.Sepolia
	data, err := os.ReadFile(
		"../../clients/feeder/testdata/sepolia/transaction/" +
			"0x30c852c522274765e1d681bc8a84ce7c41118370ef2ba7d18a427ed29f5b155.json",
	)
	require.NoError(t, err)
	var response struct {
		Transaction starknet.Transaction `json:"transaction"`
	}
	require.NoError(t, json.Unmarshal(data, &response))
	transaction, err := sn2core.AdaptTransaction(&response.Transaction)
	require.NoError(t, err)
	require.IsType(t, &core.DeclareTransaction{}, transaction)
	require.True(t, transaction.TxVersion().Is(3))

	assertBlockRoundTrip(t, network, singleTransactionBlock(transaction))
}

func transactionOfType(
	t *testing.T,
	block *rpcv10.BlockWithReceipts,
	transactionType rpcv10.TransactionType,
	version uint64,
) *rpcv10.TransactionWithReceipt {
	t.Helper()
	for i := range block.Transactions {
		transaction := &block.Transactions[i]
		served := &transaction.Transaction
		if served.Type == transactionType && served.Version.Uint64() == version {
			return transaction
		}
	}
	t.Fatalf("no %s v%d transaction", transactionType, version)
	return nil
}

func TestAdaptBlockErrors(t *testing.T) {
	_, err := rpc2core.AdaptBlock(nil, &networks.Mainnet)
	require.EqualError(t, err, "nil block")

	tests := []struct {
		name    string
		fixture fixtureBlock
		mutate  func(t *testing.T, block *rpcv10.BlockWithReceipts)
		err     string
	}{
		{
			name:    "header missing block_number",
			fixture: sepoliaIntegration35749,
			mutate:  func(t *testing.T, block *rpcv10.BlockWithReceipts) { block.Number = nil },
			err:     "block header is missing block_number",
		},
		{
			name:    "header missing block_hash",
			fixture: sepoliaIntegration35749,
			mutate:  func(t *testing.T, block *rpcv10.BlockWithReceipts) { block.Hash = nil },
			err:     "block 35749 header is missing block_hash",
		},
		{
			name:    "header missing l1_data_gas_price.price_in_fri",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				block.L1DataGasPrice.InFri = nil
			},
			err: "block 35749 header is missing l1_data_gas_price.price_in_fri",
		},
		{
			name:    "header unknown l1_da_mode",
			fixture: sepoliaIntegration35749,
			mutate:  func(t *testing.T, block *rpcv10.BlockWithReceipts) { block.L1DAMode = 7 },
			err:     "block 35749 header has unknown l1_da_mode 7",
		},
		{
			name:    "receipt missing transaction_hash",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				block.Transactions[0].Receipt.Hash = nil
			},
			err: "block 35749 transaction 0: receipt is missing transaction_hash",
		},
		{
			name:    "transaction missing version",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 1).Transaction.Version = nil
			},
			err: "is missing version",
		},
		{
			name:    "transaction unknown type",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 1).Transaction.Type = rpcv10.Invalid
			},
			err: "has unknown type <unknown>",
		},
		{
			name:    "invoke missing calldata",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 1).Transaction.CallData = nil
			},
			err: "is missing calldata",
		},
		{
			name:    "invoke missing signature",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 1).Transaction.Signature = nil
			},
			err: "is missing signature",
		},
		{
			name:    "invoke v0 missing contract_address",
			fixture: mainnet2889,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 0).Transaction.ContractAddress = nil
			},
			err: "is missing contract_address",
		},
		{
			name:    "invoke v0 missing entry_point_selector",
			fixture: mainnet2889,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 0).Transaction.EntryPointSelector = nil
			},
			err: "is missing entry_point_selector",
		},
		{
			name:    "invoke v0 missing max_fee",
			fixture: mainnet2889,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 0).Transaction.MaxFee = nil
			},
			err: "is missing max_fee",
		},
		{
			name:    "invoke v1 missing sender_address",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 1).Transaction.SenderAddress = nil
			},
			err: "is missing sender_address",
		},
		{
			name:    "invoke v1 missing nonce",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 1).Transaction.Nonce = nil
			},
			err: "is missing nonce",
		},
		{
			name:    "invoke v3 missing resource_bounds",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnInvoke, 3).Transaction.ResourceBounds = nil
			},
			err: "is missing resource_bounds",
		},
		{
			name:    "invoke v3 missing resource_bounds.l2_gas.max_price_per_unit",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				invoke := transactionOfType(t, block, rpcv10.TxnInvoke, 3)
				invoke.Transaction.ResourceBounds.L2Gas.MaxPricePerUnit = nil
			},
			err: "is missing resource_bounds.l2_gas.max_price_per_unit",
		},
		{
			name:    "invoke with unsupported version cannot be hashed",
			fixture: integration330363,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				invoke := transactionOfType(t, block, rpcv10.TxnInvoke, 3)
				invoke.Transaction.Version = felt.NewFromUint64[felt.Felt](5)
			},
			err: "hashing transaction",
		},
		{
			name:    "deploy missing constructor_calldata",
			fixture: mainnet2889,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeploy, 0).Transaction.ConstructorCallData = nil
			},
			err: "is missing constructor_calldata",
		},
		{
			name:    "deploy missing contract_address in its receipt",
			fixture: mainnet2889,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeploy, 0).Receipt.ContractAddress = nil
			},
			err: "is missing contract_address in its receipt",
		},
		{
			name:    "deploy account missing class_hash",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeployAccount, 3).Transaction.ClassHash = nil
			},
			err: "is missing class_hash",
		},
		{
			name:    "deploy account missing contract_address_salt",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeployAccount, 3).Transaction.ContractAddressSalt = nil
			},
			err: "is missing contract_address_salt",
		},
		{
			name:    "deploy account v1 missing max_fee",
			fixture: mainnet16697,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeployAccount, 1).Transaction.MaxFee = nil
			},
			err: "is missing max_fee",
		},
		{
			name:    "declare missing signature",
			fixture: mainnet2889,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeclare, 0).Transaction.Signature = nil
			},
			err: "is missing signature",
		},
		{
			name:    "declare v1 missing class_hash",
			fixture: mainnet16697,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeclare, 1).Transaction.ClassHash = nil
			},
			err: "is missing class_hash",
		},
		{
			name:    "declare v1 missing nonce",
			fixture: mainnet16697,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeclare, 1).Transaction.Nonce = nil
			},
			err: "is missing nonce",
		},
		{
			name:    "declare v2 missing compiled_class_hash",
			fixture: integration283364,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnDeclare, 2).Transaction.CompiledClassHash = nil
			},
			err: "is missing compiled_class_hash",
		},
		{
			name:    "l1 handler missing calldata",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnL1Handler, 0).Transaction.CallData = nil
			},
			err: "is missing calldata",
		},
		{
			name:    "l1 handler empty calldata",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				l1Handler := transactionOfType(t, block, rpcv10.TxnL1Handler, 0)
				l1Handler.Transaction.CallData = &felt.Slice[felt.Felt]{}
			},
			err: "is missing calldata",
		},
		{
			name:    "l1 handler missing entry_point_selector",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				transactionOfType(t, block, rpcv10.TxnL1Handler, 0).Transaction.EntryPointSelector = nil
			},
			err: "is missing entry_point_selector",
		},
		{
			name:    "receipt missing actual_fee.amount",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				block.Transactions[0].Receipt.ActualFee.Amount = nil
			},
			err: "is missing actual_fee.amount",
		},
		{
			name:    "receipt null event",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				block.Transactions[0].Receipt.Events = []*rpcv10.Event{nil}
			},
			err: "is missing events[0]",
		},
		{
			name:    "receipt event missing from_address",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				block.Transactions[0].Receipt.Events = []*rpcv10.Event{{}}
			},
			err: "is missing events[0].from_address",
		},
		{
			name:    "receipt message missing from_address",
			fixture: sepoliaIntegration35749,
			mutate: func(t *testing.T, block *rpcv10.BlockWithReceipts) {
				block.Transactions[0].Receipt.MessagesSent = []rpcv10.MsgToL1{{}}
			},
			err: "is missing messages_sent[0].from_address",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block := servedBlock(t, coreBlock(t, test.fixture))
			test.mutate(t, block)

			adapted, err := rpc2core.AdaptBlock(block, test.fixture.network)
			require.ErrorContains(t, err, test.err)
			assert.Nil(t, adapted)
		})
	}
}
