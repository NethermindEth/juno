package rpcv10

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
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

//nolint:lll // File paths
const (
	invokeTxnPath        = "../../clients/feeder/testdata/sepolia/transaction/0x76b52e17bc09064bd986ead34263e6305ef3cecfb3ae9e19b86bf4f1a1a20ea.json"
	declareTxnPath       = "../../clients/feeder/testdata/sepolia/transaction/0x30c852c522274765e1d681bc8a84ce7c41118370ef2ba7d18a427ed29f5b155.json"
	deployAccountTxnPath = "../../clients/feeder/testdata/sepolia/transaction/0x32413f8cee053089d6d7026a72e4108262ca3cfe868dd9159bc1dd160aec975.json"

	sierraClassPath = "../../clients/feeder/testdata/sepolia/class/0x3cc90db763e736ca9b6c581ea4008408842b1a125947ab087438676a7e40b7b.json"
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

// noopMempool always accepts a Push without any side effect.
type noopMempool struct{}

func (noopMempool) Push(context.Context, *mempool.BroadcastedTransaction) error {
	return nil
}

type benchcase struct {
	name string
	tx   *BroadcastedTransaction
}

func getBenchmarkCases(tb testing.TB) []benchcase {
	tb.Helper()
	return []benchcase{
		{name: "declare tx", tx: buildDeclareTx(tb)},
		{name: "invoke tx", tx: loadBroadcastedTxn(tb, invokeTxnPath)},
		{name: "deploy account tx", tx: loadBroadcastedTxn(tb, deployAccountTxnPath)},
	}
}

func buildDeclareTx(tb testing.TB) *BroadcastedTransaction {
	tb.Helper()
	sierraClassBytes := readFile(tb, sierraClassPath)
	var contractClass ContractClass
	if err := json.Unmarshal(sierraClassBytes, &contractClass); err != nil {
		tb.Fatalf("error unmarshalling contract class: %v", err)
	}

	broadcastedTxn := loadBroadcastedTxn(tb, declareTxnPath)
	broadcastedTxn.ContractClass = &contractClass
	return broadcastedTxn
}

func loadBroadcastedTxn(tb testing.TB, path string) *BroadcastedTransaction {
	tb.Helper()

	var temp struct {
		Transaction starknet.Transaction `json:"transaction"`
	}
	if err := json.Unmarshal(readFile(tb, path), &temp); err != nil {
		tb.Fatalf("error unmarshalling broadcast txn: %v", err)
	}

	coreTx, err := sn2core.AdaptTransaction(&temp.Transaction)
	if err != nil {
		tb.Fatalf("error adapting transaction: %v", err)
	}

	tx := AdaptCoreTransaction(coreTx)
	broadcastedTxn := BroadcastedTransaction{Transaction: *tx}
	return &broadcastedTxn
}

func readFile(tb testing.TB, path string) []byte {
	abs, err := filepath.Abs(path)
	if err != nil {
		tb.Fatalf("error resolving path: %v", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		tb.Fatalf("error reading file: %v", err)
	}
	return data
}

// ------------------------------------------------------------------
// Handler builders for the two AddTransaction paths.
// ------------------------------------------------------------------

// newMempoolHandler wires a Handler that takes the addToMempool branch.
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

func BenchmarkAddTxn_Mempool(b *testing.B) {
	benchcases := getBenchmarkCases(b)
	ctx := b.Context()

	for _, benchcase := range benchcases {
		b.Run(benchcase.name, func(b *testing.B) {
			h := newMempoolHandler(b, stubCompiler{})
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				_, err := h.AddTransaction(ctx, benchcase.tx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ------------------------------------------------------------------
// AddTransaction benchmarks — full method call, feeder gateway path.
// ------------------------------------------------------------------

func BenchmarkAddTxn_Gateway(b *testing.B) {
	benchcases := getBenchmarkCases(b)
	ctx := b.Context()

	for _, benchcase := range benchcases {
		b.Run(benchcase.name, func(b *testing.B) {
			h := newGatewayHandler(b, stubCompiler{})
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				_, err := h.AddTransaction(ctx, benchcase.tx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ------------------------------------------------------------------
// REAL COMPILER benchmarks — Declare via each AddTransaction path with
// the in-process FFI Sierra->CASM compiler. mempool + feed both enabled
// (default node config): v9 compiles Sierra TWICE per submission, v10
// compiles ONCE.
// ------------------------------------------------------------------

func BenchmarkAddTxn_Declare_RealCompiler_Mempool(b *testing.B) {
	tx := buildDeclareTx(b)
	h := newMempoolHandler(b, compiler.NewUnsafe())
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
	tx := buildDeclareTx(b)
	h := newGatewayHandler(b, compiler.NewUnsafe())
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
