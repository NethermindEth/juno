package contract_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/eth/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

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
	data := bytes.Repeat([]byte{0x11}, 32) // globalRoot
	data = append(data, leftPad32Uint64(1337)...)
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

	var wantRoot, wantHash felt.Felt
	wantRoot.SetBytes(bytes.Repeat([]byte{0x11}, 32))
	wantHash.SetBytes(bytes.Repeat([]byte{0x22}, 32))
	assert.Equal(t, wantRoot, ev.GlobalRoot)
	assert.Equal(t, uint64(1337), ev.BlockNumber)
	assert.Equal(t, wantHash, ev.BlockHash)
	assert.Equal(t, uint64(1_000), uint64(ev.Raw.BlockNumber))
	assert.False(t, ev.Raw.Removed)
}

func TestDecode_BlockNumberMaxUint64(t *testing.T) {
	data := make([]byte, 96)
	// globalRoot - anything decodable.
	data[31] = 0x01
	// blockNumber slot [32:64]: upper 24 bytes zero, low 8 bytes math.MaxUint64.
	for i := 56; i < 64; i++ {
		data[i] = 0xff
	}
	// blockHash - anything decodable.
	data[95] = 0x02

	ev, err := contract.Decode(&eth.Log{
		Topics: []eth.Hash{contract.LogStateUpdateSigHash},
		Data:   eth.DataBytes(data),
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(math.MaxUint64), ev.BlockNumber)
}

func TestDecode_BlockNumberUpperBytesMustBeZero(t *testing.T) {
	data := make([]byte, 96)
	data[31] = 0x01 // globalRoot - anything decodable.
	data[40] = 0xaa // noise inside the blockNumber slot's upper 24 bytes
	data[63] = 0x05 // low bytes: blockNumber 5
	data[95] = 0x02 // blockHash - anything decodable.

	_, err := contract.Decode(&eth.Log{
		Topics: []eth.Hash{contract.LogStateUpdateSigHash},
		Data:   eth.DataBytes(data),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blockNumber")
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
	assert.Contains(t, err.Error(), "bad LogStateUpdate data length")
}

type fakeLogClient struct {
	filterReturn []eth.Log
	filterErr    error
}

func (f *fakeLogClient) FilterLogs(_ context.Context, _ client.FilterQuery) ([]eth.Log, error) {
	return f.filterReturn, f.filterErr
}

func TestFilterLogStateUpdate_DecodesAll(t *testing.T) {
	fc := &fakeLogClient{
		filterReturn: []eth.Log{
			validStateUpdateLog(1),
			validStateUpdateLog(2),
		},
	}
	contractAddr := eth.AddressFromString("0x000000000000000000000000000000000000beef")

	got, err := contract.FilterLogStateUpdate(t.Context(), fc, contractAddr, 100, 200)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, uint64(1), got[0].BlockNumber)
	assert.Equal(t, uint64(2), got[1].BlockNumber)
}

func TestFilterLogStateUpdate_DecodeFailureSurfaces(t *testing.T) {
	bad := validStateUpdateLog(1)
	bad.Data = bad.Data[:50] // truncated
	fc := &fakeLogClient{filterReturn: []eth.Log{bad}}

	_, err := contract.FilterLogStateUpdate(t.Context(), fc,
		eth.Address{}, 0, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad LogStateUpdate data length")
}

func TestFilterLogStateUpdate_FilterErr(t *testing.T) {
	fc := &fakeLogClient{filterErr: errors.New("rate limited")}
	_, err := contract.FilterLogStateUpdate(t.Context(), fc, eth.Address{}, 0, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}

// --- helpers ---

// validStateUpdateLog varies globalRoot and blockHash by blockNumber so
// different blocks yield different logs.
func validStateUpdateLog(blockNumber uint64) eth.Log {
	data := make([]byte, 0, 96)
	// globalRoot — use blockNumber as a placeholder.
	data = append(data, leftPad32Uint64(blockNumber)...)
	// blockNumber as int256 (positive values match unsigned encoding).
	data = append(data, leftPad32Uint64(blockNumber)...)
	// blockHash — also placeholder.
	data = append(data, leftPad32Uint64(blockNumber)...)
	return eth.Log{
		Topics:      []eth.Hash{contract.LogStateUpdateSigHash},
		Data:        eth.DataBytes(data),
		BlockNumber: eth.HexU64(blockNumber + 1_000_000),
	}
}

// leftPad32Uint64 encodes n as a 32-byte big-endian uint256 (uint64
// value zero-extended into the high 24 bytes).
func leftPad32Uint64(n uint64) []byte {
	out := make([]byte, 32)
	binary.BigEndian.PutUint64(out[24:], n)
	return out
}
