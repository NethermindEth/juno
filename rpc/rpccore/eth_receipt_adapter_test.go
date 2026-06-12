package rpccore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/rpc/rpccore"
	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

// fakeGethFetcher is a hand-rolled implementation of gethTxReceiptFetcher
// for adapter tests. Using a real mock would require generating one for a
// private interface; this struct is simpler and clearer at the call site.
type fakeGethFetcher struct {
	wantHash gethCommon.Hash
	receipt  *gethTypes.Receipt
	err      error
}

func (f *fakeGethFetcher) TransactionReceipt(
	_ context.Context, txHash gethCommon.Hash,
) (*gethTypes.Receipt, error) {
	f.wantHash = txHash
	return f.receipt, f.err
}

func TestEthReceiptAdapter_TransactionReceipt_ConvertsAllFields(t *testing.T) {
	txHash := eth.HashFromString("0x1111111111111111111111111111111111111111111111111111111111111111")

	gethReceipt := &gethTypes.Receipt{Logs: []*gethTypes.Log{
		{
			Topics: []gethCommon.Hash{
				gethCommon.HexToHash("0xaaaa000000000000000000000000000000000000000000000000000000000001"),
				gethCommon.HexToHash("0xbbbb000000000000000000000000000000000000000000000000000000000002"),
			},
			Data:        []byte{0xde, 0xad, 0xbe, 0xef},
			BlockNumber: 0x12345,
			Removed:     false,
			// Fields not consumed by juno — must be ignored by the adapter:
			Address: gethCommon.HexToAddress("0xc0ffee0000000000000000000000000000000001"),
			TxHash:  gethCommon.HexToHash("0xfeedbeef"),
			Index:   7,
		},
		{
			Topics:      nil,
			Data:        nil,
			BlockNumber: 0x12346,
			Removed:     true,
		},
	}}

	fetcher := &fakeGethFetcher{receipt: gethReceipt}
	adapter := &rpccore.EthReceiptAdapter{Sub: fetcher}

	got, err := adapter.TransactionReceipt(t.Context(), txHash)
	require.NoError(t, err)

	// txHash crossed the boundary correctly.
	require.Equal(t, gethCommon.Hash(txHash), fetcher.wantHash)

	require.Len(t, got.Logs, 2)

	// Log 0: every consumed field round-trips.
	require.Equal(t, []eth.Hash{
		eth.HashFromString("0xaaaa000000000000000000000000000000000000000000000000000000000001"),
		eth.HashFromString("0xbbbb000000000000000000000000000000000000000000000000000000000002"),
	}, got.Logs[0].Topics)
	require.Equal(t, eth.DataBytes{0xde, 0xad, 0xbe, 0xef}, got.Logs[0].Data)
	require.Equal(t, eth.HexU64(0x12345), got.Logs[0].BlockNumber)
	require.False(t, got.Logs[0].Removed)

	// Log 1: nil Topics and nil Data survive; Removed=true propagates.
	require.Empty(t, got.Logs[1].Topics)
	require.Empty(t, got.Logs[1].Data)
	require.Equal(t, eth.HexU64(0x12346), got.Logs[1].BlockNumber)
	require.True(t, got.Logs[1].Removed)
}

func TestEthReceiptAdapter_TransactionReceipt_EmptyLogs(t *testing.T) {
	fetcher := &fakeGethFetcher{receipt: &gethTypes.Receipt{Logs: nil}}
	adapter := &rpccore.EthReceiptAdapter{Sub: fetcher}

	got, err := adapter.TransactionReceipt(t.Context(), eth.Hash{})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got.Logs)
}

func TestEthReceiptAdapter_TransactionReceipt_NilReceiptReturnsError(t *testing.T) {
	fetcher := &fakeGethFetcher{receipt: nil}
	adapter := &rpccore.EthReceiptAdapter{Sub: fetcher}

	got, err := adapter.TransactionReceipt(t.Context(), eth.Hash{})
	require.Nil(t, got)
	require.Error(t, err)
}

func TestEthReceiptAdapter_TransactionReceipt_PropagatesError(t *testing.T) {
	sentinel := errors.New("rpc upstream down")
	fetcher := &fakeGethFetcher{err: sentinel}
	adapter := &rpccore.EthReceiptAdapter{Sub: fetcher}

	got, err := adapter.TransactionReceipt(t.Context(), eth.Hash{})
	require.Nil(t, got)
	require.ErrorIs(t, err, sentinel)
}
