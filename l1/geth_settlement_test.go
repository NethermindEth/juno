package l1_test

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logStateUpdateTopicHex is keccak256("LogStateUpdate(uint256,int256,uint256)").
// Pinning the topic directly here (instead of importing either backend's
// constant) lets these tests catch a regression in either decoder.
const logStateUpdateTopicHex = "0xd342ddf7a308dec111745b00315c14b7efb2bdae570a6856e088ed0c65a3576c"

// gethLogStateUpdateJSON builds the JSON-RPC log envelope for a
// LogStateUpdate event. Mirrors stateUpdateLogJSON used by the hand-rolled
// tests so both backends are exercised against the same canonical payload.
func gethLogStateUpdateJSON(blockNumber, l1RefHeight uint64, removed bool) map[string]any {
	data := make([]byte, 96)
	data[31] = byte(blockNumber & 0xff)
	binary.BigEndian.PutUint64(data[56:64], blockNumber)
	data[95] = byte((blockNumber >> 8) & 0xff)
	return map[string]any{
		"address":          "0x0000000000000000000000000000000000000000",
		"topics":           []string{logStateUpdateTopicHex},
		"data":             "0x" + hex.EncodeToString(data),
		"blockNumber":      fmt.Sprintf("0x%x", l1RefHeight),
		"transactionHash":  "0x" + hex.EncodeToString(make([]byte, 32)),
		"transactionIndex": "0x0",
		"blockHash":        "0x" + hex.EncodeToString(make([]byte, 32)),
		"logIndex":         "0x0",
		"removed":          removed,
	}
}

// gethFullHeaderJSON returns a header payload with every field
// go-ethereum's types.Header JSON decoder treats as required (difficulty,
// gasLimit, etc.). Only Number is read by FinalisedHeight.
func gethFullHeaderJSON(number uint64) map[string]any {
	zeroHash := "0x" + hex.EncodeToString(make([]byte, 32))
	zeroAddr := "0x" + hex.EncodeToString(make([]byte, 20))
	zeroBloom := "0x" + hex.EncodeToString(make([]byte, 256))
	zeroNonce := "0x0000000000000000"
	return map[string]any{
		"parentHash":       zeroHash,
		"sha3Uncles":       zeroHash,
		"miner":            zeroAddr,
		"stateRoot":        zeroHash,
		"transactionsRoot": zeroHash,
		"receiptsRoot":     zeroHash,
		"logsBloom":        zeroBloom,
		"difficulty":       "0x0",
		"number":           fmt.Sprintf("0x%x", number),
		"gasLimit":         "0x0",
		"gasUsed":          "0x0",
		"timestamp":        "0x0",
		"extraData":        "0x",
		"mixHash":          zeroHash,
		"nonce":            zeroNonce,
		"hash":             zeroHash,
	}
}

// gethReceiptJSON returns a minimal eth_getTransactionReceipt response
// carrying the supplied logs. Only Logs is read in juno today.
func gethReceiptJSON(logs []map[string]any) map[string]any {
	zeroHash := "0x" + hex.EncodeToString(make([]byte, 32))
	zeroAddr := "0x" + hex.EncodeToString(make([]byte, 20))
	zeroBloom := "0x" + hex.EncodeToString(make([]byte, 256))
	return map[string]any{
		"transactionHash":   zeroHash,
		"transactionIndex":  "0x0",
		"blockHash":         zeroHash,
		"blockNumber":       "0x0",
		"from":              zeroAddr,
		"to":                zeroAddr,
		"cumulativeGasUsed": "0x0",
		"gasUsed":           "0x0",
		"contractAddress":   nil,
		"logs":              logs,
		"logsBloom":         zeroBloom,
		"status":            "0x1",
		"type":              "0x0",
		"effectiveGasPrice": "0x0",
	}
}

// TestGethSettlement_RejectsNonWSScheme verifies the URL guard fires
// before any dial attempt: http/https/wsx all fail with a clear scheme
// error so operators don't get a confusing wire-level failure later.
func TestGethSettlement_RejectsNonWSScheme(t *testing.T) {
	_, err := l1.NewGethSettlement(t.Context(), "http://example.invalid", eth.Address{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported url scheme")
}

// TestGethSettlement_NewGethSettlement_DialError verifies the constructor
// fails cleanly when the underlying dial fails.
func TestGethSettlement_NewGethSettlement_DialError(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err := l1.NewGethSettlement(ctx, "ws://127.0.0.1:1", eth.Address{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial L1")
}

// TestGethSettlement_ChainID exercises the happy path.
func TestGethSettlement_ChainID(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x539", nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	id, err := s.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1337", id.String())
}

// TestGethSettlement_LatestHeight exercises eth_blockNumber.
func TestGethSettlement_LatestHeight(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_blockNumber" {
			return "0xc8", nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	got, err := s.LatestHeight(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(200), got)
}

// TestGethSettlement_FinalisedHeight returns the header height via
// eth_getBlockByNumber("finalized", ...).
func TestGethSettlement_FinalisedHeight(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_getBlockByNumber" {
			return gethFullHeaderJSON(100), nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	got, err := s.FinalisedHeight(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(100), got)
}

// TestGethSettlement_FinalisedHeight_NotFound verifies the sentinel
// translation: a null result becomes ethereum.NotFound at the ethclient
// layer; GethSettlement must rewrap as eth.ErrNotFound.
func TestGethSettlement_FinalisedHeight_NotFound(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_getBlockByNumber" {
			return nil, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	_, err = s.FinalisedHeight(t.Context())
	require.Error(t, err)
	assert.True(t, errors.Is(err, eth.ErrNotFound),
		"FinalisedHeight must wrap eth.ErrNotFound; got %v", err)
}

// TestGethSettlement_FilterStateUpdate_DecodesAndTranslates exercises the
// abigen filterer + the StateUpdate translation against the same canonical
// payload as the hand-rolled equivalent.
func TestGethSettlement_FilterStateUpdate_DecodesAndTranslates(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_getLogs" {
			return []any{
				gethLogStateUpdateJSON(1, 1_000, false),
				gethLogStateUpdateJSON(2, 1_001, true),
			}, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	got, err := s.FilterStateUpdate(t.Context(), 100, 200)
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, uint64(1), got[0].L2BlockNumber)
	assert.Equal(t, uint64(1_000), got[0].L1RefHeight)
	assert.False(t, got[0].Removed)
	require.NotNil(t, got[0].L2BlockHash)
	require.NotNil(t, got[0].StateRoot)

	assert.Equal(t, uint64(2), got[1].L2BlockNumber)
	assert.Equal(t, uint64(1_001), got[1].L1RefHeight)
	assert.True(t, got[1].Removed)
}

// TestGethSettlement_FilterStateUpdate_ErrorWrapsRange surfaces the
// [from, to] range so operators can correlate a failure against the sync
// loop's logs.
func TestGethSettlement_FilterStateUpdate_ErrorWrapsRange(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(_ client.TestRequest) (any, *client.TestRPCError) {
		return nil, &client.TestRPCError{Code: -32000, Message: "query timeout"}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	_, err = s.FilterStateUpdate(t.Context(), 42, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "[42,99]")
	assert.Contains(t, err.Error(), "query timeout")
}

// TestGethSettlement_TransactionReceipt verifies the geth-to-eth receipt
// conversion: Topics, Data, BlockNumber, and Removed must survive the
// translation untouched.
func TestGethSettlement_TransactionReceipt(t *testing.T) {
	wantTopic := "0x" + hex.EncodeToString(append(make([]byte, 31), 0xaa))
	wantData := []byte{0xde, 0xad, 0xbe, 0xef}

	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_getTransactionReceipt" {
			return gethReceiptJSON([]map[string]any{{
				"address":          "0x0000000000000000000000000000000000000000",
				"topics":           []string{wantTopic},
				"data":             "0x" + hex.EncodeToString(wantData),
				"blockNumber":      "0x1092",
				"transactionHash":  "0x" + hex.EncodeToString(make([]byte, 32)),
				"transactionIndex": "0x0",
				"blockHash":        "0x" + hex.EncodeToString(make([]byte, 32)),
				"logIndex":         "0x0",
				"removed":          true,
			}}), nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	r, err := s.TransactionReceipt(t.Context(), eth.Hash{})
	require.NoError(t, err)
	require.Len(t, r.Logs, 1)

	got := r.Logs[0]
	require.Len(t, got.Topics, 1)
	assert.Equal(t, wantTopic, got.Topics[0].Hex())
	assert.Equal(t, eth.DataBytes(wantData), got.Data)
	assert.Equal(t, eth.HexU64(0x1092), got.BlockNumber)
	assert.True(t, got.Removed)
}

// TestGethSettlement_TransactionReceipt_NotFound verifies the sentinel
// translation: a null JSON-RPC result becomes ethereum.NotFound at the
// ethclient layer; GethSettlement must wrap as eth.ErrNotFound.
func TestGethSettlement_TransactionReceipt_NotFound(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_getTransactionReceipt" {
			return nil, nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	_, err = s.TransactionReceipt(t.Context(), eth.Hash{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, eth.ErrNotFound),
		"TransactionReceipt must wrap eth.ErrNotFound on missing tx; got %v", err)
}

// TestGethSettlement_ListenerFiresOnErrorPath verifies the deferred-observe
// contract: OnL1Call fires on every method, success or failure, so error
// rate is observable.
func TestGethSettlement_ListenerFiresOnErrorPath(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(_ client.TestRequest) (any, *client.TestRPCError) {
		return nil, &client.TestRPCError{Code: -32000, Message: "boom"}
	})

	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	t.Cleanup(s.Close)

	rec := &recordingListener{}
	s.SetListener(rec)

	_, _ = s.ChainID(t.Context())
	_, _ = s.LatestHeight(t.Context())
	_, _ = s.FinalisedHeight(t.Context())
	_, _ = s.TransactionReceipt(t.Context(), eth.Hash{})

	got := rec.Methods()
	want := []string{
		"eth_chainId",
		"eth_blockNumber",
		"eth_getBlockByNumber",
		"eth_getTransactionReceipt",
	}
	assert.Equal(t, want, got)
}

// TestGethSettlement_CloseIsIdempotent — double-Close must not panic.
func TestGethSettlement_CloseIsIdempotent(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x1", nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})
	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{})
	require.NoError(t, err)
	s.Close()
	s.Close()
}

// TestGethSettlement_WithSettlementLogger smoke-tests option wiring.
func TestGethSettlement_WithSettlementLogger(t *testing.T) {
	srv := client.NewTestServer(t)
	srv.SetHandler(func(req client.TestRequest) (any, *client.TestRPCError) {
		if req.Method == "eth_chainId" {
			return "0x1", nil
		}
		return nil, &client.TestRPCError{Code: -32601, Message: req.Method}
	})
	s, err := l1.NewGethSettlement(t.Context(), srv.WSURL(), eth.Address{},
		l1.WithSettlementLogger(log.NewNopZapLogger()),
	)
	require.NoError(t, err)
	t.Cleanup(s.Close)

	id, err := s.ChainID(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1", id.String())
}
