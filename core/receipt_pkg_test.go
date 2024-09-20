package core_test

import (
	"context"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func getReceipts(b *testing.B, blockNum uint64) []*core.TransactionReceipt {
	b.Helper()
	client := feeder.NewClient(utils.Sepolia.FeederURL)
	gw := adaptfeeder.New(client)
	block, err := gw.BlockByNumber(context.Background(), blockNum)
	require.NoError(b, err)

	return slices.Repeat(block.Receipts, 100)
}

func BenchmarkReceiptCommitment(b *testing.B) {
	receipts := getReceipts(b, 35748)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := core.ReceiptCommitment(receipts)
		require.NoError(b, err)
	}
}
