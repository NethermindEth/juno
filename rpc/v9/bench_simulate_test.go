package rpcv9

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/ethereum/go-ethereum/common"
)

// ------------------------------------------------------------------
// simulateTransactions / estimateFee prep: per-tx adapt loop work.
// Same per-tx call path that prepareTransactions runs for each entry.
// ------------------------------------------------------------------

func BenchmarkSimulatePrep_3Mixed(b *testing.B) {
	txs := []*BroadcastedTransaction{
		loadBroadcastedTxn(b, invokeTxnPath),
		loadBroadcastedTxn(b, deployAccountTxnPath),
		buildDeclareTx(b),
	}
	ctx := b.Context()
	stub := stubCompiler{}
	network := &networks.Sepolia
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		coreTxs := make([]core.Transaction, 0, len(txs))
		var classes []core.ClassDefinition
		for _, tx := range txs {
			coreTx, sierra, err := AdaptBroadcastedTransaction(
				ctx, stub, tx, network,
			)
			if err != nil {
				b.Fatal(err)
			}
			coreTxs = append(coreTxs, coreTx)
			if sierra != nil {
				classes = append(classes, sierra)
			}
		}
		_ = coreTxs
		_ = classes
	}
}

// ------------------------------------------------------------------
// EstimateMessageFee prep: v9 builds a BroadcastedTransaction wrapper
// of type L1_HANDLER and runs it through the full broadcasted-to-core
// adapt pipeline (incl. core.TransactionHash).
// ------------------------------------------------------------------

func BenchmarkMessageFeePrep(b *testing.B) {
	from := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	to := felt.NewUnsafeFromString[felt.Felt](
		"0x4a3140b5a24d4010e8e7bd02142ebd6a3e6daf8d8aa5568f0db45961938d1ad",
	)
	selector := felt.NewUnsafeFromString[felt.Felt](
		"0x3dbc508ba4afd040c8dc4ff8a61113a7bcaf5eae88a6ba27b3c50578b3587e3",
	)
	payload := []felt.Felt{
		felt.FromUint64[felt.Felt](1),
		felt.FromUint64[felt.Felt](2),
		felt.FromUint64[felt.Felt](3),
		felt.FromUint64[felt.Felt](4),
	}
	msg := &MsgFromL1{
		From: from, To: *to, Selector: *selector, Payload: payload,
	}
	ctx := b.Context()
	stub := stubCompiler{}
	network := &networks.Sepolia
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// Mirror v9's EstimateMessageFee prep: build a BroadcastedTransaction
		// of type L1_HANDLER and run AdaptBroadcastedTransaction on it.
		calldata := make([]*felt.Felt, len(msg.Payload)+1)
		calldata[0] = felt.NewFromBytes[felt.Felt](msg.From.Bytes())
		for j := range msg.Payload {
			calldata[j+1] = &msg.Payload[j]
		}
		tx := BroadcastedTransaction{
			Transaction: Transaction{
				Type:               TxnL1Handler,
				ContractAddress:    &msg.To,
				EntryPointSelector: &msg.Selector,
				CallData:           &calldata,
				Version:            &felt.Zero,
				Nonce:              &felt.Zero,
			},
		}
		_, _, err := AdaptBroadcastedTransaction(ctx, stub, &tx, network)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// silence unused-import errors when isolated
var _ = rpccore.ErrInternal
