package rpcv10

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	gomock "go.uber.org/mock/gomock"
)

// ------------------------------------------------------------------
// Test infrastructure: stub compiler, stub mempool, fixture loaders.
// ------------------------------------------------------------------

// stubCompiler returns a minimal CasmClass without spawning a child process.
// Sn2core.AdaptSierraClass requires Prime to parse as a big.Int.
type stubCompiler struct{}

func (stubCompiler) Compile(
	_ context.Context, _ *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	return &starknet.CasmClass{
		Prime: "0x800000000000011000000000000000000000000000000000000000000000001",
	}, nil
}

// realFFICompiler returns the in-process Sierra->CASM compiler used by the
// REAL_COMPILER benchmark.
func realFFICompiler() compiler.Compiler { return compiler.NewUnsafe() }

// noopMempool always accepts a Push without any side effect.
type noopMempool struct{}

func (noopMempool) Push(context.Context, *mempool.BroadcastedTransaction) error {
	return nil
}

const sierraClassFixturePath = "../../clients/feeder/testdata/sepolia-integration/class/0x941a2dc3ab607819fdc929bea95831a2e0c1aab2f2f34b3a23c55cebc8a040.json"

func loadSierraClassRaw(tb testing.TB) []byte {
	tb.Helper()
	abs, err := filepath.Abs(sierraClassFixturePath)
	if err != nil {
		tb.Fatalf("resolve sierra class path: %v", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		tb.Fatalf("read sierra class fixture: %v", err)
	}
	return data
}

const benchInvokeV3JSON = `{
	"type": "INVOKE",
	"version": "0x3",
	"sender_address": "0xf9e998b2853e6d01f3ae3c598c754c1b9a7bd398fec7657de022f3b778679",
	"calldata": [
		"0x1",
		"0x41a78e741e5af2fec34b695679bc6891742439f7afb8484ecd7766661ad02bf",
		"0x1987cbd17808b9a23693d4de7e246a443cfe37e6e7fbaeabd7d7e6532b07c3d",
		"0x4",
		"0x16342ade8a7cc8296920731bc34b5a6530f5ee1dc1bfd3cc83cb3f519d6530a",
		"0x65d7d6a3cd92f5d836fc410db222801cf70c6966bf5c0dc4d25699def10f4e9",
		"0x1",
		"0x0"
	],
	"signature": ["0x1", "0x2"],
	"nonce": "0x77d8",
	"resource_bounds": {
		"l1_gas":      {"max_amount": "0x100", "max_price_per_unit": "0x1"},
		"l2_gas":      {"max_amount": "0x100", "max_price_per_unit": "0x1"},
		"l1_data_gas": {"max_amount": "0x100", "max_price_per_unit": "0x1"}
	},
	"tip": "0x0",
	"paymaster_data": [],
	"nonce_data_availability_mode": "L1",
	"fee_data_availability_mode": "L1",
	"account_deployment_data": []
}`

const benchDeployAccountV3JSON = `{
	"type": "DEPLOY_ACCOUNT",
	"version": "0x3",
	"signature": [
		"0x63c0e0fe22d6e82187b84e06f33644f7dc6edce494a317bfcdd0bb57ab862fa",
		"0x6219aa7d091eac96f07d7d195f12eff9a8786af85ddf41028428ee8f510e75e"
	],
	"nonce": "0x1",
	"contract_address_salt": "0x520b540d51c06e1539cbc42e93a37cbef534082c75a3991179cfac83da67fdb",
	"constructor_calldata": [
		"0x33444ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2",
		"0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463"
	],
	"class_hash": "0x26ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918",
	"resource_bounds": {
		"l1_gas":      {"max_amount": "0x6fde2b4eb000", "max_price_per_unit": "0x6fde2b4eb000"},
		"l2_gas":      {"max_amount": "0x6fde2b4eb000", "max_price_per_unit": "0x6fde2b4eb000"},
		"l1_data_gas": {"max_amount": "0x6fde2b4eb000", "max_price_per_unit": "0x6fde2b4eb000"}
	},
	"tip": "0x1",
	"paymaster_data": [],
	"nonce_data_availability_mode": "L1",
	"fee_data_availability_mode": "L2"
}`

const benchDeclareV3HeaderJSON = `{
	"type": "DECLARE",
	"version": "0x3",
	"signature": ["0x1", "0x2"],
	"nonce": "0x1",
	"sender_address": "0x2fab82e4aef1d8664874e1f194951856d48463c3e6bf9a8c68e234a629a6f50",
	"compiled_class_hash": "0x1add56d64bebf8140f3b8a38bdf102b7874437f0c861ab4ca7526ec33b4d0f8",
	"resource_bounds": {
		"l1_gas":      {"max_amount": "0x186a0", "max_price_per_unit": "0x2540be400"},
		"l2_gas":      {"max_amount": "0x0",    "max_price_per_unit": "0x0"},
		"l1_data_gas": {"max_amount": "0x186a0", "max_price_per_unit": "0x2540be400"}
	},
	"tip": "0x0",
	"paymaster_data": [],
	"account_deployment_data": [],
	"nonce_data_availability_mode": "L1",
	"fee_data_availability_mode": "L1"`

func buildDeclareBroadcastJSON(sierraClassBytes []byte) []byte {
	out := make([]byte, 0, len(benchDeclareV3HeaderJSON)+len(sierraClassBytes)+32)
	out = append(out, benchDeclareV3HeaderJSON...)
	out = append(out, `, "contract_class": `...)
	out = append(out, sierraClassBytes...)
	out = append(out, '}')
	return out
}

func loadBroadcastTxn(tb testing.TB, raw []byte) *BroadcastedTransaction {
	tb.Helper()
	tx := &BroadcastedTransaction{}
	if err := json.Unmarshal(raw, tx); err != nil {
		tb.Fatalf("unmarshal broadcast txn: %v", err)
	}
	return tx
}

// ------------------------------------------------------------------
// Handler builders for the two AddTransaction paths.
// ------------------------------------------------------------------

// newMempoolHandler wires a Handler that takes the addToMempool branch.
// receivedTransactionFeed is wired because it is enabled by default in the
// running node, so it must be present in every benchmark iteration.
// Compiler is a parameter so the same builder is reused for stub and FFI.
func newMempoolHandler(b *testing.B, c compiler.Compiler) *Handler {
	b.Helper()
	ctrl := gomock.NewController(b)
	mockReader := mocks.NewMockReader(ctrl)
	mockReader.EXPECT().Network().Return(&networks.Sepolia).AnyTimes()

	return New(mockReader, nil, nil, log.NewNopZapLogger()).
		WithCompiler(c).
		WithMempool(noopMempool{}).
		WithReceivedTransactionFeed(feed.New[core.Transaction]())
}

// newGatewayHandler wires a Handler that takes the pushToFeederGateway branch
// (no mempool). The mock gateway returns a canned, well-formed response.
// receivedTransactionFeed is wired because it is enabled by default in the
// running node, so it must be present in every benchmark iteration.
// Compiler is a parameter so the same builder is reused for stub and FFI.
func newGatewayHandler(b *testing.B, c compiler.Compiler) *Handler {
	b.Helper()
	ctrl := gomock.NewController(b)
	mockReader := mocks.NewMockReader(ctrl)
	mockReader.EXPECT().Network().Return(&networks.Sepolia).AnyTimes()

	mockGateway := mocks.NewMockGateway(ctrl)
	mockGateway.EXPECT().
		AddTransaction(gomock.Any(), gomock.Any()).
		Return(json.RawMessage(`{"transaction_hash":"0x1"}`), nil).
		AnyTimes()

	return New(mockReader, nil, nil, log.NewNopZapLogger()).
		WithCompiler(c).
		WithGateway(mockGateway).
		WithReceivedTransactionFeed(feed.New[core.Transaction]())
}

// ------------------------------------------------------------------
// AddTransaction benchmarks — full method call, mempool path.
// ------------------------------------------------------------------

func BenchmarkAddTxn_Mempool_Invoke(b *testing.B) {
	tx := loadBroadcastTxn(b, []byte(benchInvokeV3JSON))
	h := newMempoolHandler(b, stubCompiler{})
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := h.AddTransaction(ctx, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAddTxn_Mempool_DeployAccount(b *testing.B) {
	tx := loadBroadcastTxn(b, []byte(benchDeployAccountV3JSON))
	h := newMempoolHandler(b, stubCompiler{})
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := h.AddTransaction(ctx, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAddTxn_Mempool_Declare(b *testing.B) {
	sierraRaw := loadSierraClassRaw(b)
	tx := loadBroadcastTxn(b, buildDeclareBroadcastJSON(sierraRaw))
	h := newMempoolHandler(b, stubCompiler{})
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := h.AddTransaction(ctx, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ------------------------------------------------------------------
// AddTransaction benchmarks — full method call, feeder gateway path.
// ------------------------------------------------------------------

func BenchmarkAddTxn_Gateway_Invoke(b *testing.B) {
	tx := loadBroadcastTxn(b, []byte(benchInvokeV3JSON))
	h := newGatewayHandler(b, stubCompiler{})
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := h.AddTransaction(ctx, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAddTxn_Gateway_DeployAccount(b *testing.B) {
	tx := loadBroadcastTxn(b, []byte(benchDeployAccountV3JSON))
	h := newGatewayHandler(b, stubCompiler{})
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := h.AddTransaction(ctx, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAddTxn_Gateway_Declare(b *testing.B) {
	sierraRaw := loadSierraClassRaw(b)
	tx := loadBroadcastTxn(b, buildDeclareBroadcastJSON(sierraRaw))
	h := newGatewayHandler(b, stubCompiler{})
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := h.AddTransaction(ctx, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ------------------------------------------------------------------
// REAL COMPILER benchmarks — Declare via each AddTransaction path with
// the in-process FFI Sierra->CASM compiler. mempool + feed both enabled
// (default node config): v9 compiles Sierra TWICE per submission, v10
// compiles ONCE.
// ------------------------------------------------------------------

func BenchmarkAddTxn_Declare_RealCompiler_Mempool(b *testing.B) {
	sierraRaw := loadSierraClassRaw(b)
	tx := loadBroadcastTxn(b, buildDeclareBroadcastJSON(sierraRaw))
	h := newMempoolHandler(b, realFFICompiler())
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := h.AddTransaction(ctx, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAddTxn_Declare_RealCompiler_Gateway(b *testing.B) {
	sierraRaw := loadSierraClassRaw(b)
	tx := loadBroadcastTxn(b, buildDeclareBroadcastJSON(sierraRaw))
	h := newGatewayHandler(b, realFFICompiler())
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := h.AddTransaction(ctx, tx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
