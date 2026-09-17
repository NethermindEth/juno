package cbor_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/l1/eth"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/NethermindEth/juno/utils/cbor/v1"
	bloom "github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

const goldenFile = "testdata/on_disk_bytes.json"

func loadGoldenBytes(t *testing.T, path string) map[string]string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var vectors map[string]string
	require.NoError(t, json.Unmarshal(data, &vectors))
	return vectors
}

// goldenCases pins the bytes the node writes today.
func goldenCases() []struct {
	name  string
	value any
} {
	return []struct {
		name  string
		value any
	}{
		{"DeclareTransaction", core.DeclareTransaction{}},
		{"DeployTransaction", core.DeployTransaction{}},
		{"InvokeTransaction", core.InvokeTransaction{}},
		{"L1HandlerTransaction", core.L1HandlerTransaction{}},
		{"DeployAccountTransaction", core.DeployAccountTransaction{}},
		{"DeprecatedCairoClass", core.DeprecatedCairoClass{}},
		{"SierraClass", core.SierraClass{}},
		{"DeletedNode", trienode.DeletedNode{}},
		{"LeafNode", trienode.LeafNode{}},
		{"NonLeafNode", trienode.NonLeafNode{}},
		{"JournalNodeSet", pathdb.JournalNodeSet{}},
		{"DiffJournal", pathdb.DiffJournal{}},
		{"DiskJournal", pathdb.DiskJournal{}},
		{"DBJournal", pathdb.DBJournal{}},
		{"WALProposal", starknet.WALProposal{}},
		{"WALPrevote", starknet.WALPrevote{}},
		{"WALPrecommit", starknet.WALPrecommit{}},
		{"WALTimeout", starknet.WALTimeout{}},
		{"Header", core.Header{}},
		{"TransactionReceipt", core.TransactionReceipt{}},
		{"[][]byte, the p2p shape", [][]byte{{1, 2, 3}, {4, 5}, {}}},
		{"[]byte, the schema state", []byte{9, 8, 7}},
		{"[][]byte nil", [][]byte(nil)},
		{"*felt.Felt, the 37-byte shape", felt.NewFromUint64[felt.Felt](7)},
		{"felt.Slice, two felts", felt.Slice[felt.Felt]{
			felt.FromUint64[felt.Felt](7),
			felt.FromUint64[felt.Felt](8),
		}},
		{"felt.Slice nil", felt.Slice[felt.Felt](nil)},
		{"cbor.RawMessage", cbor.RawMessage{0x83, 0x01, 0x02, 0x03}},
		{"DeclaredClassDefinition, a Sierra class", populatedDeclaredClassDefinition()},
		{"Header, populated", populatedHeader()},
		{"InvokeTransaction, populated", populatedInvokeTransaction()},
		{"TransactionReceipt, populated", populatedReceipt()},
	}
}

func populatedDeclaredClassDefinition() core.DeclaredClassDefinition {
	return core.DeclaredClassDefinition{
		At: 777,
		Class: &core.SierraClass{
			Abi:             "some abi",
			AbiHash:         felt.NewFromUint64[felt.Felt](1),
			Program:         []felt.Felt{felt.FromUint64[felt.Felt](2), felt.FromUint64[felt.Felt](3)},
			ProgramHash:     felt.NewFromUint64[felt.Felt](4),
			SemanticVersion: "0.1.0",
			EntryPoints: core.SierraEntryPointsByType{
				Constructor: []core.SierraEntryPoint{{Index: 0, Selector: felt.NewFromUint64[felt.Felt](5)}},
				External:    []core.SierraEntryPoint{{Index: 1, Selector: felt.NewFromUint64[felt.Felt](6)}},
				L1Handler:   []core.SierraEntryPoint{{Index: 2, Selector: felt.NewFromUint64[felt.Felt](7)}},
			},
		},
	}
}

func populatedHeader() core.Header {
	return core.Header{
		Hash:             felt.NewFromUint64[felt.Felt](1),
		ParentHash:       felt.NewFromUint64[felt.Felt](2),
		Number:           3,
		GlobalStateRoot:  felt.NewFromUint64[felt.Felt](4),
		SequencerAddress: felt.NewFromUint64[felt.Felt](5),
		TransactionCount: 6,
		EventCount:       7,
		Timestamp:        8,
		ProtocolVersion:  "0.13.2",
		EventsBloom:      bloom.New(64, 3),
		L1GasPriceETH:    felt.NewFromUint64[felt.Felt](9),
		Signatures: [][]*felt.Felt{{
			felt.NewFromUint64[felt.Felt](10),
			felt.NewFromUint64[felt.Felt](11),
		}},
		L1GasPriceSTRK: felt.NewFromUint64[felt.Felt](12),
		L1DAMode:       core.Blob,
		L1DataGasPrice: &core.GasPrice{
			PriceInWei: felt.NewFromUint64[felt.Felt](13),
			PriceInFri: felt.NewFromUint64[felt.Felt](14),
		},
		L2GasPrice: &core.GasPrice{
			PriceInWei: felt.NewFromUint64[felt.Felt](15),
			PriceInFri: felt.NewFromUint64[felt.Felt](16),
		},
	}
}

func populatedInvokeTransaction() core.InvokeTransaction {
	version := core.TransactionVersion(felt.FromUint64[felt.Felt](3))
	return core.InvokeTransaction{
		TransactionHash:       felt.NewFromUint64[felt.Felt](1),
		CallData:              []felt.Felt{felt.FromUint64[felt.Felt](2)},
		TransactionSignature:  []felt.Felt{felt.FromUint64[felt.Felt](3), felt.FromUint64[felt.Felt](4)},
		MaxFee:                felt.NewFromUint64[felt.Felt](5),
		ContractAddress:       felt.NewFromUint64[felt.Felt](6),
		Version:               &version,
		EntryPointSelector:    felt.NewFromUint64[felt.Felt](7),
		Nonce:                 felt.NewFromUint64[felt.Felt](8),
		SenderAddress:         felt.NewFromUint64[felt.Felt](9),
		Tip:                   10,
		PaymasterData:         []felt.Felt{felt.FromUint64[felt.Felt](11)},
		AccountDeploymentData: []felt.Felt{felt.FromUint64[felt.Felt](12)},
		NonceDAMode:           core.DAModeL2,
		FeeDAMode:             core.DAModeL1,
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas:     {MaxAmount: 13, MaxPricePerUnit: felt.NewFromUint64[felt.Felt](14)},
			core.ResourceL2Gas:     {MaxAmount: 15, MaxPricePerUnit: felt.NewFromUint64[felt.Felt](16)},
			core.ResourceL1DataGas: {MaxAmount: 17, MaxPricePerUnit: felt.NewFromUint64[felt.Felt](18)},
		},
	}
}

func populatedReceipt() core.TransactionReceipt {
	return core.TransactionReceipt{
		Fee:             felt.NewFromUint64[felt.Felt](1),
		FeeUnit:         core.STRK,
		TransactionHash: felt.NewFromUint64[felt.Felt](2),
		Reverted:        true,
		RevertReason:    "some reason",
		Events: []*core.Event{{
			From: felt.NewFromUint64[felt.Felt](3),
			Keys: []felt.Felt{felt.FromUint64[felt.Felt](4)},
			Data: []felt.Felt{felt.FromUint64[felt.Felt](5)},
		}},
		L2ToL1Message: []*core.L2ToL1Message{{
			From:    felt.NewFromUint64[felt.Felt](6),
			Payload: []felt.Felt{felt.FromUint64[felt.Felt](7)},
			To:      eth.Address{0x1, 0x2},
		}},
		ExecutionResources: &core.ExecutionResources{
			BuiltinInstanceCounter: core.BuiltinInstanceCounter{Pedersen: 8},
			MemoryHoles:            9,
			Steps:                  10,
			DataAvailability:       &core.DataAvailability{L1Gas: 11, L1DataGas: 12},
			TotalGasConsumed:       &core.GasConsumed{L1Gas: 13, L1DataGas: 14},
		},
	}
}

func TestGoldenBytes(t *testing.T) {
	golden := loadGoldenBytes(t, goldenFile)

	for _, c := range goldenCases() {
		t.Run(c.name, func(t *testing.T) {
			b, err := cbor.Marshal(c.value)
			require.NoError(t, err)

			want, ok := golden[c.name]
			require.Truef(t, ok, "no vector for %q, the encoder wrote %s", c.name, hex.EncodeToString(b))
			require.Equal(t, want, hex.EncodeToString(b))

			stored, err := hex.DecodeString(want)
			require.NoError(t, err)

			back := reflect.New(reflect.TypeOf(c.value))
			require.NoError(t, cbor.Unmarshal(stored, back.Interface()))
			require.Equal(t, c.value, back.Elem().Interface())
		})
	}
	require.Equal(t, len(goldenCases()), len(golden), "a case lost its vector")
}
