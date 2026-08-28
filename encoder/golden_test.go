package encoder_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/l1/eth"
	bloom "github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

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
		{"*felt.Felt, the 37-byte shape", new(felt.Felt).SetUint64(7)},
		{"felt.Slice, two felts", felt.Slice[felt.Felt]{
			*new(felt.Felt).SetUint64(7),
			*new(felt.Felt).SetUint64(8),
		}},
		{"felt.Slice nil", felt.Slice[felt.Felt](nil)},
		{"encoder.RawMessage", encoder.RawMessage{0x83, 0x01, 0x02, 0x03}},
		{"DeclaredClassDefinition, a Sierra class", populatedDeclaredClassDefinition()},
		{"Header, populated", populatedHeader()},
		{"InvokeTransaction, populated", populatedInvokeTransaction()},
		{"TransactionReceipt, populated", populatedReceipt()},
	}
}

func goldenFelt(n uint64) *felt.Felt { return new(felt.Felt).SetUint64(n) }

func populatedDeclaredClassDefinition() core.DeclaredClassDefinition {
	return core.DeclaredClassDefinition{
		At: 777,
		Class: &core.SierraClass{
			Abi:             "some abi",
			AbiHash:         goldenFelt(1),
			Program:         []felt.Felt{*goldenFelt(2), *goldenFelt(3)},
			ProgramHash:     goldenFelt(4),
			SemanticVersion: "0.1.0",
			EntryPoints: core.SierraEntryPointsByType{
				Constructor: []core.SierraEntryPoint{{Index: 0, Selector: goldenFelt(5)}},
				External:    []core.SierraEntryPoint{{Index: 1, Selector: goldenFelt(6)}},
				L1Handler:   []core.SierraEntryPoint{{Index: 2, Selector: goldenFelt(7)}},
			},
		},
	}
}

func populatedHeader() core.Header {
	return core.Header{
		Hash:             goldenFelt(1),
		ParentHash:       goldenFelt(2),
		Number:           3,
		GlobalStateRoot:  goldenFelt(4),
		SequencerAddress: goldenFelt(5),
		TransactionCount: 6,
		EventCount:       7,
		Timestamp:        8,
		ProtocolVersion:  "0.13.2",
		EventsBloom:      bloom.New(64, 3),
		L1GasPriceETH:    goldenFelt(9),
		Signatures:       [][]*felt.Felt{{goldenFelt(10), goldenFelt(11)}},
		L1GasPriceSTRK:   goldenFelt(12),
		L1DAMode:         core.Blob,
		L1DataGasPrice:   &core.GasPrice{PriceInWei: goldenFelt(13), PriceInFri: goldenFelt(14)},
		L2GasPrice:       &core.GasPrice{PriceInWei: goldenFelt(15), PriceInFri: goldenFelt(16)},
	}
}

func populatedInvokeTransaction() core.InvokeTransaction {
	version := core.TransactionVersion(*goldenFelt(3))
	return core.InvokeTransaction{
		TransactionHash:       goldenFelt(1),
		CallData:              []felt.Felt{*goldenFelt(2)},
		TransactionSignature:  []felt.Felt{*goldenFelt(3), *goldenFelt(4)},
		MaxFee:                goldenFelt(5),
		ContractAddress:       goldenFelt(6),
		Version:               &version,
		EntryPointSelector:    goldenFelt(7),
		Nonce:                 goldenFelt(8),
		SenderAddress:         goldenFelt(9),
		Tip:                   10,
		PaymasterData:         []felt.Felt{*goldenFelt(11)},
		AccountDeploymentData: []felt.Felt{*goldenFelt(12)},
		NonceDAMode:           core.DAModeL2,
		FeeDAMode:             core.DAModeL1,
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas:     {MaxAmount: 13, MaxPricePerUnit: goldenFelt(14)},
			core.ResourceL2Gas:     {MaxAmount: 15, MaxPricePerUnit: goldenFelt(16)},
			core.ResourceL1DataGas: {MaxAmount: 17, MaxPricePerUnit: goldenFelt(18)},
		},
	}
}

func populatedReceipt() core.TransactionReceipt {
	return core.TransactionReceipt{
		Fee:             goldenFelt(1),
		FeeUnit:         core.STRK,
		TransactionHash: goldenFelt(2),
		Reverted:        true,
		RevertReason:    "some reason",
		Events: []*core.Event{{
			From: goldenFelt(3),
			Keys: []felt.Felt{*goldenFelt(4)},
			Data: []felt.Felt{*goldenFelt(5)},
		}},
		L2ToL1Message: []*core.L2ToL1Message{{
			From:    goldenFelt(6),
			Payload: []felt.Felt{*goldenFelt(7)},
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

// maps a struct with a value that is already on disk in some node.
//
//nolint:lll,nolintlint // Ignore lll for literals, nolintlint because main config doesn't check tests
var golden = map[string]string{
	"DeclareTransaction":       "da00010000ae6354697000654e6f6e6365f6664d6178466565f66756657273696f6ef669436c61737348617368f66946656544414d6f6465006b4e6f6e636544414d6f6465006d5061796d617374657244617461f66d53656e64657241646472657373f66e5265736f75726365426f756e6473f66f5472616e73616374696f6e48617368f671436f6d70696c6564436c61737348617368f6745472616e73616374696f6e5369676e6174757265f6754163636f756e744465706c6f796d656e7444617461f6",
	"DeployTransaction":        "da00010001a66756657273696f6ef669436c61737348617368f66f436f6e747261637441646472657373f66f5472616e73616374696f6e48617368f673436f6e7374727563746f7243616c6c44617461f673436f6e74726163744164647265737353616c74f6",
	"InvokeTransaction":        "da00010002af6354697000654e6f6e6365f6664d6178466565f66756657273696f6ef66843616c6c44617461f66946656544414d6f6465006b4e6f6e636544414d6f6465006d5061796d617374657244617461f66d53656e64657241646472657373f66e5265736f75726365426f756e6473f66f436f6e747261637441646472657373f66f5472616e73616374696f6e48617368f672456e747279506f696e7453656c6563746f72f6745472616e73616374696f6e5369676e6174757265f6754163636f756e744465706c6f796d656e7444617461f6",
	"L1HandlerTransaction":     "da00010003a6654e6f6e6365f66756657273696f6ef66843616c6c44617461f66f436f6e747261637441646472657373f66f5472616e73616374696f6e48617368f672456e747279506f696e7453656c6563746f72f6",
	"DeployAccountTransaction": "da00010004ae6354697000654e6f6e6365f6664d6178466565f66756657273696f6ef669436c61737348617368f66946656544414d6f6465006b4e6f6e636544414d6f6465006d5061796d617374657244617461f66e5265736f75726365426f756e6473f66f436f6e747261637441646472657373f66f5472616e73616374696f6e48617368f673436f6e7374727563746f7243616c6c44617461f673436f6e74726163744164647265737353616c74f6745472616e73616374696f6e5369676e6174757265f6",
	"DeprecatedCairoClass":     "da00010005a563416269f66750726f6772616d606945787465726e616c73f66a4c3148616e646c657273f66c436f6e7374727563746f7273f6",
	"SierraClass":              "da00010006a763416269606741626948617368f66750726f6772616df668436f6d70696c6564f66b456e747279506f696e7473a36845787465726e616cf6694c3148616e646c6572f66b436f6e7374727563746f72f66b50726f6772616d48617368f66f53656d616e74696356657273696f6e60",
	"DeletedNode":              "da00010007a0",
	"LeafNode":                 "da00010008a0",
	"NonLeafNode":              "da00010009a0",
	"JournalNodeSet":           "da0001000aa1654e6f646573f6",
	"DiffJournal":              "da0001000ba364526f6f74840000000065426c6f636b006a456e634e6f6465736574f6",
	"DiskJournal":              "da0001000ca36249440064526f6f7484000000006a456e634e6f6465736574f6",
	"DBJournal":                "da0001000da26756657273696f6e0069456e634c6179657273f6",
	"WALProposal":              "da0001000ea565726f756e64006576616c7565f666686569676874006673656e64657284000000006b76616c69645f726f756e6400",
	"WALPrevote":               "da0001000fa4626964f665726f756e640066686569676874006673656e6465728400000000",
	"WALPrecommit":             "da00010010a4626964f665726f756e640066686569676874006673656e6465728400000000",
	"WALTimeout":               "da0001001183000000",
	"Header":                   "b06448617368f6664e756d62657200684c3144414d6f646500686761737072696365f66954696d657374616d70006a4576656e74436f756e74006a4c324761735072696365f66a506172656e7448617368f66a5369676e617475726573f66b4576656e7473426c6f6f6df66c67617370726963657374726bf66e4c31446174614761735072696365f66f476c6f62616c5374617465526f6f74f66f50726f746f636f6c56657273696f6e607053657175656e63657241646472657373f6705472616e73616374696f6e436f756e7400",
	"TransactionReceipt":       "a963466565f6664576656e7473f667466565556e697400685265766572746564f46c526576657274526561736f6e606d4c31546f4c324d657373616765f66d4c32546f4c314d657373616765f66f5472616e73616374696f6e48617368f672457865637574696f6e5265736f7572636573f6",
	"[][]byte, the p2p shape":  "834301020342040540",
	"[]byte, the schema state": "43090807",
	"[][]byte nil":             "f6",

	"*felt.Felt, the 37-byte shape": "841bffffffffffffff211bffffffffffffffff1bffffffffffffffff1b07fffffffffff130",
	"felt.Slice, two felts": "82841bffffffffffffff211bffffffffffffffff1bffffffffffffffff1b07fffffffffff130" +
		"841bffffffffffffff011bffffffffffffffff1bffffffffffffffff1b07ffffffffffef10",
	"felt.Slice nil":     "f6",
	"encoder.RawMessage": "83010203",

	"DeclaredClassDefinition, a Sierra class": "5901bd0000000000000309da00010006a76341626968736f6d65206162696741626948617368841bffffffffffffffe11bffffffffffffffff1bffffffffffffffff1b07fffffffffffdf06750726f6772616d82841bffffffffffffffc11bffffffffffffffff1bffffffffffffffff1b07fffffffffffbd0841bffffffffffffffa11bffffffffffffffff1bffffffffffffffff1b07fffffffffff9b068436f6d70696c6564f66b456e747279506f696e7473a36845787465726e616c81a265496e646578016853656c6563746f72841bffffffffffffff411bffffffffffffffff1bffffffffffffffff1b07fffffffffff350694c3148616e646c657281a265496e646578026853656c6563746f72841bffffffffffffff211bffffffffffffffff1bffffffffffffffff1b07fffffffffff1306b436f6e7374727563746f7281a265496e646578006853656c6563746f72841bffffffffffffff611bffffffffffffffff1bffffffffffffffff1b07fffffffffff5706b50726f6772616d48617368841bffffffffffffff811bffffffffffffffff1bffffffffffffffff1b07fffffffffff7906f53656d616e74696356657273696f6e65302e312e30",
	"Header, populated":                       "b06448617368841bffffffffffffffe11bffffffffffffffff1bffffffffffffffff1b07fffffffffffdf0664e756d62657203684c3144414d6f646501686761737072696365841bfffffffffffffee11bffffffffffffffff1bffffffffffffffff1b07ffffffffffecf06954696d657374616d70086a4576656e74436f756e74076a4c324761735072696365a26a5072696365496e467269841bfffffffffffffe011bffffffffffffffff1bffffffffffffffff1b07ffffffffffde106a5072696365496e576569841bfffffffffffffe211bffffffffffffffff1bffffffffffffffff1b07ffffffffffe0306a506172656e7448617368841bffffffffffffffc11bffffffffffffffff1bffffffffffffffff1b07fffffffffffbd06a5369676e6174757265738182841bfffffffffffffec11bffffffffffffffff1bffffffffffffffff1b07ffffffffffead0841bfffffffffffffea11bffffffffffffffff1bffffffffffffffff1b07ffffffffffe8b06b4576656e7473426c6f6f6d582000000000000000400000000000000003000000000000004000000000000000006c67617370726963657374726b841bfffffffffffffe811bffffffffffffffff1bffffffffffffffff1b07ffffffffffe6906e4c31446174614761735072696365a26a5072696365496e467269841bfffffffffffffe411bffffffffffffffff1bffffffffffffffff1b07ffffffffffe2506a5072696365496e576569841bfffffffffffffe611bffffffffffffffff1bffffffffffffffff1b07ffffffffffe4706f476c6f62616c5374617465526f6f74841bffffffffffffff811bffffffffffffffff1bffffffffffffffff1b07fffffffffff7906f50726f746f636f6c56657273696f6e66302e31332e327053657175656e63657241646472657373841bffffffffffffff611bffffffffffffffff1bffffffffffffffff1b07fffffffffff570705472616e73616374696f6e436f756e7406",
	"InvokeTransaction, populated":            "da00010002af635469700a654e6f6e6365841bffffffffffffff011bffffffffffffffff1bffffffffffffffff1b07ffffffffffef10664d6178466565841bffffffffffffff611bffffffffffffffff1bffffffffffffffff1b07fffffffffff5706756657273696f6e841bffffffffffffffa11bffffffffffffffff1bffffffffffffffff1b07fffffffffff9b06843616c6c4461746181841bffffffffffffffc11bffffffffffffffff1bffffffffffffffff1b07fffffffffffbd06946656544414d6f6465006b4e6f6e636544414d6f6465016d5061796d61737465724461746181841bfffffffffffffea11bffffffffffffffff1bffffffffffffffff1b07ffffffffffe8b06d53656e64657241646472657373841bfffffffffffffee11bffffffffffffffff1bffffffffffffffff1b07ffffffffffecf06e5265736f75726365426f756e6473a301a2694d6178416d6f756e740d6f4d61785072696365506572556e6974841bfffffffffffffe411bffffffffffffffff1bffffffffffffffff1b07ffffffffffe25002a2694d6178416d6f756e740f6f4d61785072696365506572556e6974841bfffffffffffffe011bffffffffffffffff1bffffffffffffffff1b07ffffffffffde1003a2694d6178416d6f756e74116f4d61785072696365506572556e6974841bfffffffffffffdc11bffffffffffffffff1bffffffffffffffff1b07ffffffffffd9d06f436f6e747261637441646472657373841bffffffffffffff411bffffffffffffffff1bffffffffffffffff1b07fffffffffff3506f5472616e73616374696f6e48617368841bffffffffffffffe11bffffffffffffffff1bffffffffffffffff1b07fffffffffffdf072456e747279506f696e7453656c6563746f72841bffffffffffffff211bffffffffffffffff1bffffffffffffffff1b07fffffffffff130745472616e73616374696f6e5369676e617475726582841bffffffffffffffa11bffffffffffffffff1bffffffffffffffff1b07fffffffffff9b0841bffffffffffffff811bffffffffffffffff1bffffffffffffffff1b07fffffffffff790754163636f756e744465706c6f796d656e744461746181841bfffffffffffffe811bffffffffffffffff1bffffffffffffffff1b07ffffffffffe690",
	"TransactionReceipt, populated":           "a963466565841bffffffffffffffe11bffffffffffffffff1bffffffffffffffff1b07fffffffffffdf0664576656e747381a3644461746181841bffffffffffffff611bffffffffffffffff1bffffffffffffffff1b07fffffffffff5706446726f6d841bffffffffffffffa11bffffffffffffffff1bffffffffffffffff1b07fffffffffff9b0644b65797381841bffffffffffffff811bffffffffffffffff1bffffffffffffffff1b07fffffffffff79067466565556e697401685265766572746564f56c526576657274526561736f6e6b736f6d6520726561736f6e6d4c31546f4c324d657373616765f66d4c32546f4c314d65737361676581a362546f5401020000000000000000000000000000000000006446726f6d841bffffffffffffff411bffffffffffffffff1bffffffffffffffff1b07fffffffffff350675061796c6f616481841bffffffffffffff211bffffffffffffffff1bffffffffffffffff1b07fffffffffff1306f5472616e73616374696f6e48617368841bffffffffffffffc11bffffffffffffffff1bffffffffffffffff1b07fffffffffffbd072457865637574696f6e5265736f7572636573a56553746570730a6b4d656d6f7279486f6c6573097044617461417661696c6162696c697479a2654c314761730b694c31446174614761730c70546f74616c476173436f6e73756d6564a3654c314761730d654c3247617300694c31446174614761730e764275696c74696e496e7374616e6365436f756e746572ac6445634f700065456373646100664164644d6f6400664b656363616b00664d756c4d6f6400664f75747075740067426974776973650068506564657273656e0868506f736569646f6e006a52616e6765436865636b006c52616e6765436865636b3936006c5365676d656e744172656e6100",
}

func TestGoldenBytes(t *testing.T) {
	for _, c := range goldenCases() {
		t.Run(c.name, func(t *testing.T) {
			b, err := encoder.Marshal(c.value)
			require.NoError(t, err)

			want, ok := golden[c.name]
			require.Truef(t, ok, "no vector for %s", c.name)
			require.Equal(t, want, hex.EncodeToString(b))

			stored, err := hex.DecodeString(want)
			require.NoError(t, err)

			back := reflect.New(reflect.TypeOf(c.value))
			require.NoError(t, encoder.Unmarshal(stored, back.Interface()))
			require.Equal(t, c.value, back.Elem().Interface())
		})
	}
	require.Len(t, golden, len(goldenCases()), "a case lost its vector")
}
