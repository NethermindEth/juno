package types_test

import (
	"testing"

	"github.com/NethermindEth/juno/l1/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEthereumAddressRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{
			name: "mainnet core contract",
			addr: "0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4",
		},
		{
			name: "zero address",
			addr: "0x0000000000000000000000000000000000000000",
		},
		{
			name: "all ones",
			addr: "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
		},
		{
			name: "mixed",
			addr: "0x1234567890123456789012345678901234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := common.HexToAddress(tt.addr)
			l1Addr := types.L1AddressFromEth(original)
			converted := l1Addr.ToEthAddress()
			require.Equal(t, original, converted, "Round-trip conversion failed")
		})
	}
}

func TestEthereumHashRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{
			name: "typical transaction hash",
			hash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		},
		{
			name: "zero hash",
			hash: "0x0000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name: "all ones",
			hash: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		},
		{
			name: "mixed",
			hash: "0x123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := common.HexToHash(tt.hash)

			l1Hash := types.L1HashFromEth(original)
			converted := l1Hash.ToEthHash()
			require.Equal(t, original, converted, "Round-trip conversion failed")
		})
	}
}

// TestEthereumAddressPadding verifies that Ethereum addresses (20 bytes)
// are correctly left-padded with 12 zero bytes when stored as L1Address (32 bytes).
func TestEthereumAddressPadding(t *testing.T) {
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	l1Addr := types.L1AddressFromEth(addr)

	bytes := l1Addr.Bytes()

	for i := range 12 {
		assert.Zero(t, bytes[i], "Expected zero padding at byte %d", i)
	}

	assert.Equal(t, addr[:], bytes[12:], "Address bytes don't match")
}

// TestCBORBackwardCompatibility ensures that L1Address CBOR encoding
// maintains backward compatibility by:
// 1. Encoding Ethereum addresses as compact 20-byte format (new behavior)
// 2. Still being able to decode old 32-byte padded format
func TestCBORBackwardCompatibility(t *testing.T) {
	ethAddr := common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")

	t.Run("New compact encoding", func(t *testing.T) {
		l1Addr := types.L1AddressFromEth(ethAddr)
		newCBOR, err := l1Addr.MarshalCBOR()
		require.NoError(t, err)

		expectedCompact, err := cbor.Marshal(ethAddr[:])
		require.NoError(t, err)

		assert.Equal(
			t,
			expectedCompact,
			newCBOR,
			"Should encode Ethereum address as compact 20-byte format",
		)
	})

	t.Run("Can decode old 32-byte format", func(t *testing.T) {
		var oldPadded [32]byte
		copy(oldPadded[12:], ethAddr[:])
		oldCBOR, err := cbor.Marshal(oldPadded[:])
		require.NoError(t, err)

		var decoded types.L1Address
		err = decoded.UnmarshalCBOR(oldCBOR)
		require.NoError(t, err)

		l1Addr := types.L1AddressFromEth(ethAddr)
		assert.True(t, types.Equal(&l1Addr, &decoded), "Failed to decode old 32-byte CBOR format")
	})

	t.Run("Can decode new 20-byte format", func(t *testing.T) {
		// New compact encoding
		l1Addr := types.L1AddressFromEth(ethAddr)
		compactCBOR, err := l1Addr.MarshalCBOR()
		require.NoError(t, err)

		var decoded types.L1Address
		err = decoded.UnmarshalCBOR(compactCBOR)
		require.NoError(t, err)

		assert.True(t, types.Equal(&l1Addr, &decoded), "Failed to decode new 20-byte CBOR format")
		assert.Equal(t, ethAddr, decoded.ToEthAddress(), "Round-trip failed")
	})
}

// TestCBORHashBackwardCompatibility ensures that L1Hash CBOR encoding
// is compatible with the previous go-ethereum-based implementation.
func TestCBORHashBackwardCompatibility(t *testing.T) {
	ethHash := common.HexToHash("0x123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0")

	oldCBOR, err := cbor.Marshal(ethHash[:])
	require.NoError(t, err)

	l1Hash := types.L1HashFromEth(ethHash)
	newCBOR, err := l1Hash.MarshalCBOR()
	require.NoError(t, err)

	assert.Equal(t, oldCBOR, newCBOR, "CBOR encoding changed - backward compatibility broken!")

	var decoded types.L1Hash
	err = decoded.UnmarshalCBOR(oldCBOR)
	require.NoError(t, err)
	assert.True(t, types.Equal(&l1Hash, &decoded), "Failed to decode old CBOR format")
}

// TestJSONCompatibility verifies JSON marshaling/unmarshaling works correctly.
func TestJSONCompatibility(t *testing.T) {
	t.Run("L1Address", func(t *testing.T) {
		ethAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		l1Addr := types.L1AddressFromEth(ethAddr)

		jsonData, err := l1Addr.MarshalJSON()
		require.NoError(t, err)

		var decoded types.L1Address
		err = decoded.UnmarshalJSON(jsonData)
		require.NoError(t, err)

		assert.True(t, types.Equal(&l1Addr, &decoded), "JSON round-trip failed")
	})

	t.Run("L1Hash", func(t *testing.T) {
		ethHash := common.HexToHash("0x123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0")
		l1Hash := types.L1HashFromEth(ethHash)

		jsonData, err := l1Hash.MarshalJSON()
		require.NoError(t, err)

		var decoded types.L1Hash
		err = decoded.UnmarshalJSON(jsonData)
		require.NoError(t, err)

		assert.True(t, types.Equal(&l1Hash, &decoded), "JSON round-trip failed")
	})
}

// TestStringRepresentation verifies string conversion is correct.
func TestStringRepresentation(t *testing.T) {
	t.Run("L1Address", func(t *testing.T) {
		ethAddr := common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")
		l1Addr := types.L1AddressFromEth(ethAddr)

		str := l1Addr.String()
		assert.NotEmpty(t, str)
		assert.Contains(t, str, "0x")

		parsed, err := types.NewFromString[types.L1Address](str)
		require.NoError(t, err)
		assert.True(t, types.Equal(&l1Addr, parsed))
	})

	t.Run("L1Hash", func(t *testing.T) {
		ethHash := common.HexToHash("0x123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0")
		l1Hash := types.L1HashFromEth(ethHash)

		str := l1Hash.String()
		assert.NotEmpty(t, str)
		assert.Contains(t, str, "0x")

		parsed, err := types.NewFromString[types.L1Hash](str)
		require.NoError(t, err)
		assert.True(t, types.Equal(&l1Hash, parsed))
	})
}
