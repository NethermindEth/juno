package contract_test

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/eth/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

// TestLogStateUpdateSigHash_DerivedFromSignature verifies the
// hard-coded constant is keccak256("LogStateUpdate(uint256,int256,uint256)").
// Catches a typo in the constant if anyone ever edits it.
func TestLogStateUpdateSigHash_DerivedFromSignature(t *testing.T) {
	h := sha3.NewLegacyKeccak256()
	h.Write([]byte("LogStateUpdate(uint256,int256,uint256)"))
	var sum eth.Hash
	sum.SetBytes(h.Sum(nil))
	assert.Equal(t, sum, contract.LogStateUpdateSigHash)
}

func TestDecode_Success(t *testing.T) {
	// globalRoot = 0x11..11; blockNumber = 0x539 (1337);
	// blockHash = 0x22..22.
	data := bytes.Repeat([]byte{0x11}, 32)                 // globalRoot
	data = append(data, leftPad32(big.NewInt(1337).Bytes())...)
	data = append(data, bytes.Repeat([]byte{0x22}, 32)...) // blockHash
	require.Len(t, data, 96)

	log := &eth.Log{
		Topics:      []eth.Hash{contract.LogStateUpdateSigHash},
		Data:        eth.DataBytes(data),
		BlockNumber: eth.HexU64(1_000),
		Removed:     false,
	}

	ev, err := contract.Decode(log)
	require.NoError(t, err)
	assert.Equal(t, new(big.Int).SetBytes(bytes.Repeat([]byte{0x11}, 32)), ev.GlobalRoot)
	assert.Equal(t, big.NewInt(1337), ev.BlockNumber)
	assert.Equal(t, new(big.Int).SetBytes(bytes.Repeat([]byte{0x22}, 32)), ev.BlockHash)
	assert.Equal(t, uint64(1_000), uint64(ev.Raw.BlockNumber))
	assert.False(t, ev.Raw.Removed)
}

func TestDecode_NegativeInt256(t *testing.T) {
	// All-0xFF data is "-1" as int256.
	data := make([]byte, 96)
	for i := range data {
		data[i] = 0xff
	}
	log := &eth.Log{
		Topics: []eth.Hash{contract.LogStateUpdateSigHash},
		Data:   eth.DataBytes(data),
	}
	ev, err := contract.Decode(log)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(-1), ev.BlockNumber,
		"int256 0xff..ff should decode as -1")

	// GlobalRoot/BlockHash are uint256 — unsigned — and stay positive.
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	assert.Equal(t, max, ev.GlobalRoot)
	assert.Equal(t, max, ev.BlockHash)
}

func TestDecode_WrongTopic(t *testing.T) {
	log := &eth.Log{
		Topics: []eth.Hash{eth.HashFromString("0x" + strings.Repeat("00", 32))},
		Data:   eth.DataBytes(make([]byte, 96)),
	}
	_, err := contract.Decode(log)
	require.ErrorIs(t, err, contract.ErrWrongTopic)
}

func TestDecode_NoTopics(t *testing.T) {
	log := &eth.Log{Data: eth.DataBytes(make([]byte, 96))}
	_, err := contract.Decode(log)
	require.ErrorIs(t, err, contract.ErrWrongTopic)
}

func TestDecode_BadDataLength(t *testing.T) {
	log := &eth.Log{
		Topics: []eth.Hash{contract.LogStateUpdateSigHash},
		Data:   eth.DataBytes(make([]byte, 95)),
	}
	_, err := contract.Decode(log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad data length")
}

// fakeLogClient is a hand-written LogClient for the Filter/Watch tests.
type fakeLogClient struct {
	mu           sync.Mutex
	filterReturn []eth.Log
	filterErr    error
	subSink      chan<- *eth.Log
	subErr       chan error
	closed       chan struct{}
	closeOnce    sync.Once
}

func (f *fakeLogClient) FilterLogs(_ context.Context, _ client.FilterQuery) ([]eth.Log, error) {
	return f.filterReturn, f.filterErr
}

func (f *fakeLogClient) SubscribeLogs(_ context.Context, _ client.FilterQuery, sink chan<- *eth.Log) (eth.Subscription, error) {
	f.mu.Lock()
	f.subSink = sink
	f.subErr = make(chan error, 1)
	f.closed = make(chan struct{})
	f.mu.Unlock()
	return f, nil
}

func (f *fakeLogClient) Err() <-chan error { return f.subErr }
func (f *fakeLogClient) Unsubscribe() {
	f.closeOnce.Do(func() {
		close(f.closed)
	})
}

func TestFilterLogStateUpdate_DecodesAll(t *testing.T) {
	fc := &fakeLogClient{
		filterReturn: []eth.Log{
			validStateUpdateLog(big.NewInt(1)),
			validStateUpdateLog(big.NewInt(2)),
		},
	}
	contractAddr := eth.AddressFromString("0x000000000000000000000000000000000000beef")

	got, err := contract.FilterLogStateUpdate(t.Context(), fc, contractAddr, 100, 200)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, big.NewInt(1), got[0].BlockNumber)
	assert.Equal(t, big.NewInt(2), got[1].BlockNumber)
}

func TestFilterLogStateUpdate_DecodeFailureSurfaces(t *testing.T) {
	bad := validStateUpdateLog(big.NewInt(1))
	bad.Data = bad.Data[:50] // truncated
	fc := &fakeLogClient{filterReturn: []eth.Log{bad}}

	_, err := contract.FilterLogStateUpdate(t.Context(), fc,
		eth.Address{}, 0, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad data length")
}

func TestFilterLogStateUpdate_FilterErr(t *testing.T) {
	fc := &fakeLogClient{filterErr: errors.New("rate limited")}
	_, err := contract.FilterLogStateUpdate(t.Context(), fc, eth.Address{}, 0, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestWatchLogStateUpdate_DeliversDecoded(t *testing.T) {
	fc := &fakeLogClient{}
	sink := make(chan *contract.LogStateUpdate, 4)
	sub, err := contract.WatchLogStateUpdate(t.Context(), fc, eth.Address{}, sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Push two log payloads through the fake's sub sink.
	for _, n := range []int64{42, 43} {
		fc.mu.Lock()
		sender := fc.subSink
		fc.mu.Unlock()
		raw := validStateUpdateLog(big.NewInt(n))
		sender <- &raw
	}
	got := drainStateUpdates(t, sink, 2, time.Second)
	require.Len(t, got, 2)
	assert.Equal(t, big.NewInt(42), got[0].BlockNumber)
	assert.Equal(t, big.NewInt(43), got[1].BlockNumber)
}

func TestWatchLogStateUpdate_DecodeFailureClosesErr(t *testing.T) {
	fc := &fakeLogClient{}
	sink := make(chan *contract.LogStateUpdate, 1)
	sub, err := contract.WatchLogStateUpdate(t.Context(), fc, eth.Address{}, sink)
	require.NoError(t, err)

	// Wait until SubscribeLogs has installed the sink, then push a
	// malformed log.
	require.Eventually(t, func() bool {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		return fc.subSink != nil
	}, time.Second, time.Millisecond)
	bad := validStateUpdateLog(big.NewInt(1))
	bad.Data = bad.Data[:1]
	fc.subSink <- &bad

	select {
	case errOut := <-sub.Err():
		require.Error(t, errOut)
		assert.Contains(t, errOut.Error(), "decode")
	case <-time.After(time.Second):
		t.Fatal("Err() did not fire on decode failure")
	}
}

func TestWatchLogStateUpdate_InnerErrPropagates(t *testing.T) {
	fc := &fakeLogClient{}
	sink := make(chan *contract.LogStateUpdate, 1)
	sub, err := contract.WatchLogStateUpdate(t.Context(), fc, eth.Address{}, sink)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Signal a transport failure from the fake.
	want := errors.New("ws closed")
	fc.mu.Lock()
	fc.subErr <- want
	fc.mu.Unlock()

	select {
	case got := <-sub.Err():
		assert.ErrorIs(t, got, want)
	case <-time.After(time.Second):
		t.Fatal("Err() did not propagate inner error")
	}
}

// --- helpers ---

// validStateUpdateLog builds an eth.Log with the LogStateUpdate sig
// hash and a well-formed 96-byte data section. blockNumber is the
// int256; globalRoot and blockHash are placeholders that vary by
// blockNumber so different blocks produce different logs (a sanity
// hook for ordering tests).
func validStateUpdateLog(blockNumber *big.Int) eth.Log {
	data := make([]byte, 0, 96)
	// globalRoot — use blockNumber as a placeholder.
	data = append(data, leftPad32(blockNumber.Bytes())...)
	// blockNumber as int256 (positive values match unsigned encoding).
	data = append(data, leftPad32(blockNumber.Bytes())...)
	// blockHash — also placeholder.
	data = append(data, leftPad32(blockNumber.Bytes())...)
	return eth.Log{
		Topics:      []eth.Hash{contract.LogStateUpdateSigHash},
		Data:        eth.DataBytes(data),
		BlockNumber: eth.HexU64(blockNumber.Uint64() + 1_000_000),
	}
}

func leftPad32(b []byte) []byte {
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

func drainStateUpdates(t *testing.T, sink <-chan *contract.LogStateUpdate, n int, timeout time.Duration) []*contract.LogStateUpdate {
	t.Helper()
	deadline := time.After(timeout)
	out := make([]*contract.LogStateUpdate, 0, n)
	for len(out) < n {
		select {
		case ev := <-sink:
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
	return out
}
