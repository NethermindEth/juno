package core

import (
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

// note(rdr): based on git blame, it seems this global var is here to avoid certain compiler optimizations.
//			  it would be nice to have extra clarity

var benchReceiptR felt.Felt

func BenchmarkReceiptCommitment(b *testing.B) {
	// receipts were taken from sepolia block 35748
	// we don't use adaptfeeder here because it causes cyclic import
	baseReceipts := []*TransactionReceipt{
		{
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x5ac644bbd6ae98d3be2d988439854e33f0961e24f349a63b43e16d172bfe747"),
			Fee:             felt.NewUnsafeFromString[felt.Felt]("0xd07af45c84550"),
			Events: []*Event{
				{
					From: felt.NewUnsafeFromString[felt.Felt]("0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
					Data: []*felt.Felt{
						felt.NewUnsafeFromString[felt.Felt]("0x472aa8128e01eb0df145810c9511a92852d62a68ba8198ce5fa414e6337a365"),
						felt.NewUnsafeFromString[felt.Felt]("0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
						felt.NewUnsafeFromString[felt.Felt]("0xd07af45c84550"),
						felt.NewUnsafeFromString[felt.Felt]("0x0"),
					},
					Keys: []*felt.Felt{
						felt.NewUnsafeFromString[felt.Felt]("0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"),
					},
				},
			},
			ExecutionResources: &ExecutionResources{
				BuiltinInstanceCounter: BuiltinInstanceCounter{
					Pedersen:   16,
					RangeCheck: 157,
					Ecsda:      1,
					Poseidon:   4,
				},
				MemoryHoles: 0,
				Steps:       3950,
				DataAvailability: &DataAvailability{
					L1Gas:     0,
					L1DataGas: 192,
				},
				TotalGasConsumed: &GasConsumed{
					L1Gas:     117620,
					L1DataGas: 192,
				},
			},
		},
		{
			Fee: felt.NewUnsafeFromString[felt.Felt]("0x471426f16c4330"),
			Events: []*Event{
				{
					From: felt.NewUnsafeFromString[felt.Felt]("0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
					Data: []*felt.Felt{
						felt.NewUnsafeFromString[felt.Felt]("0x472aa8128e01eb0df145810c9511a92852d62a68ba8198ce5fa414e6337a365"),
						felt.NewUnsafeFromString[felt.Felt]("0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
						felt.NewUnsafeFromString[felt.Felt]("0x471426f16c4330"),
						felt.NewUnsafeFromString[felt.Felt]("0x0"),
					},
					Keys: []*felt.Felt{
						felt.NewUnsafeFromString[felt.Felt]("0x99cd8bde557814842a3121e8ddfd433a539b8c9f14bf31ebf108d12e6196e9"),
					},
				},
			},
			ExecutionResources: &ExecutionResources{
				BuiltinInstanceCounter: BuiltinInstanceCounter{
					Pedersen:   16,
					RangeCheck: 157,
					Ecsda:      1,
					Poseidon:   4,
				},
				Steps: 3950,
				DataAvailability: &DataAvailability{
					L1Gas:     0,
					L1DataGas: 192,
				},
				TotalGasConsumed: &GasConsumed{
					L1Gas:     641644,
					L1DataGas: 192,
				},
			},
			TransactionHash: felt.NewUnsafeFromString[felt.Felt]("0x21bc0afe54123b946855e1bf9389d943313df5c5c396fbf0630234a44f6f592"),
		},
	}
	receipts := slices.Repeat(baseReceipts, 100)
	var f felt.Felt
	var err error
	b.ResetTimer()
	for range b.N {
		f, err = receiptCommitment(receipts)
		require.NoError(b, err)
	}
	benchReceiptR = f
}
