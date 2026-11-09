package core_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/stretchr/testify/require"
)

func TestTypedHeader(t *testing.T) {
	headers := []core.Header{
		{},
		{Number: 24, Timestamp: 1 << 40, ProtocolVersion: "0.13.α", Hash: felt.NewFromUint64[felt.Felt](17), Signatures: [][]*felt.Felt{}},
		{Number: 7, Signatures: [][]*felt.Felt{nil, {}, {nil, felt.NewFromUint64[felt.Felt](19)}}, L1DataGasPrice: &core.GasPrice{PriceInWei: felt.NewFromUint64[felt.Felt](3)}, L2GasPrice: &core.GasPrice{PriceInFri: felt.NewFromUint64[felt.Felt](5)}},
	}
	for _, want := range headers {
		data, err := cbor.Marshal(want)
		require.NoError(t, err)
		for _, decode := range []func([]byte, *core.Header) error{core.DecodeHeader, fastcbor.Unmarshal[core.Header]} {
			var got core.Header
			require.NoError(t, decode(data, &got))
			require.Equal(t, want, got)
		}
		input := append(bytes.Clone(data), 0x07)
		before := bytes.Clone(input)
		var got core.Header
		rest, err := core.DecodeHeaderFirst(input, &got)
		require.NoError(t, err)
		require.Equal(t, []byte{7}, rest)
		require.Equal(t, want, got)
		require.Equal(t, before, input)
		rest[0] = 8
		require.Equal(t, byte(8), input[len(data)])
		for i := range data {
			input[i] ^= 0xff
		}
		require.Equal(t, want, got, "decoded values must not borrow the input")
		for i := 0; i < len(data); i++ {
			var truncated core.Header
			require.Error(t, core.DecodeHeader(data[:i], &truncated))
			require.Equal(t, core.Header{}, truncated)
		}
		var trailing core.Header
		require.ErrorIs(t, core.DecodeHeader(append(bytes.Clone(data), 7), &trailing), fastcbor.ErrExtraneousData)
		require.Equal(t, core.Header{}, trailing)
	}
}

// Exercise non-preferred widths as well as each width's maximum, without using
// another CBOR implementation to construct the expected result.
func TestTypedHeaderUnsignedWidths(t *testing.T) {
	for _, width := range []int{0, 1, 2, 4, 8} {
		maxValue := uint64(23)
		if width != 0 {
			maxValue = ^uint64(0) >> (64 - width*8)
		}
		values := []uint64{0, 7, 23}
		if maxValue != 23 {
			values = append(values, maxValue)
		}
		for _, value := range values {
			t.Run(strconv.Itoa(width)+"/"+strconv.FormatUint(value, 10), func(t *testing.T) {
				var encoded []byte
				switch width {
				case 0:
					encoded = []byte{byte(value)}
				case 1:
					encoded = []byte{0x18, byte(value)}
				case 2:
					encoded = binary.BigEndian.AppendUint16([]byte{0x19}, uint16(value))
				case 4:
					encoded = binary.BigEndian.AppendUint32([]byte{0x1a}, uint32(value))
				case 8:
					encoded = binary.BigEndian.AppendUint64([]byte{0x1b}, value)
				}
				record := append([]byte{0xa1, 0x66, 'N', 'u', 'm', 'b', 'e', 'r'}, encoded...)
				input := append(bytes.Clone(record), 0xff, 0x00)
				before := bytes.Clone(input)
				var got core.Header
				rest, err := core.DecodeHeaderFirst(input, &got)
				require.NoError(t, err)
				require.Equal(t, core.Header{Number: value}, got)
				require.Equal(t, []byte{0xff, 0x00}, rest)
				require.Equal(t, before, input)
				for cut := 1; cut < len(encoded); cut++ {
					got = core.Header{}
					err := core.DecodeHeader(record[:8+cut], &got)
					require.ErrorIs(t, err, cursor.ErrTruncated)
					var detail *cursor.DecodeError
					require.ErrorAs(t, err, &detail)
					require.Equal(t, 8, detail.Offset)
					require.Equal(t, core.Header{}, got)
				}
			})
		}
	}
}

func TestTypedHeaderRejectsNonzeroWithoutMutation(t *testing.T) {
	for _, want := range []core.Header{{Number: 1}, {Signatures: [][]*felt.Felt{}}, {L1DataGasPrice: &core.GasPrice{}}} {
		got := want
		rest, err := core.DecodeHeaderFirst([]byte{0xa0}, &got)
		require.Nil(t, rest)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Equal(t, want, got)
	}
}

func TestTypedHeaderKeysAndNull(t *testing.T) {
	for _, data := range [][]byte{{0xf6}, {0xf7}, {0xa0}, {0xa1, 0x66, 'n', 'u', 'm', 'b', 'e', 'r', 7}, {0xa1, 0x63, 'o', 'l', 'd', 0x81, 1}} {
		var got, want core.Header
		require.NoError(t, cbor.Unmarshal(data, &want))
		require.NoError(t, core.DecodeHeader(data, &got))
		require.Equal(t, want, got)
	}
	repeated := []byte{0xa2, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', 1, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', 2}
	var got core.Header
	require.ErrorIs(t, core.DecodeHeader(repeated, &got), cursor.ErrUnsupported)
	require.Equal(t, core.Header{}, got)
}

func TestTypedHeaderFeltNullAndEmptyArray(t *testing.T) {
	for _, tc := range []struct {
		name string
		item byte
		want *felt.Felt
	}{
		{"null", 0xf6, nil},
		{"empty array", 0x80, new(felt.Felt)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte{0xa1, 0x64, 'H', 'a', 's', 'h', tc.item}
			before := bytes.Clone(data)
			var got, want core.Header
			var planned plannerHeader
			require.NoError(t, cbor.Unmarshal(data, &want))
			require.NoError(t, fastcbor.Unmarshal(data, &planned))
			require.NoError(t, core.DecodeHeader(data, &got))
			require.Equal(t, tc.want, got.Hash)
			require.Equal(t, want, got)
			require.Equal(t, core.Header(planned), got)
			require.Equal(t, before, data)
		})
	}
}

func TestTypedHeaderErrorsKeepDiagnostics(t *testing.T) {
	var got core.Header
	err := core.DecodeHeader([]byte{}, &got)
	require.ErrorIs(t, err, cursor.ErrTruncated)
	require.EqualError(t, err, "cbor: unexpected end of input at byte 0: input ends before the item")
	require.ErrorIs(t, core.DecodeHeader([]byte{0xa1, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', 0xf5}, &got), cursor.ErrType)
	require.Equal(t, core.Header{}, got)
}

// A valid first field must not escape when the next declared pair is missing.
func TestTypedHeaderClearsPartialResult(t *testing.T) {
	for number := range byte(24) {
		t.Run(strconv.Itoa(int(number)), func(t *testing.T) {
			data := []byte{0xa1, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', number}
			var got core.Header
			require.NoError(t, core.DecodeHeader(data, &got))
			require.Equal(t, core.Header{Number: uint64(number)}, got)

			data[0] = 0xa2 // Two pairs declared; only the first is present.
			before := bytes.Clone(data)
			checkError := func(err error) {
				t.Helper()
				require.ErrorIs(t, err, cursor.ErrTruncated)
				var detail *cursor.DecodeError
				require.ErrorAs(t, err, &detail)
				require.Equal(t, len(data), detail.Offset)
				require.EqualError(t, err, "cbor: unexpected end of input at byte 9: input ends before the item")
				require.Equal(t, core.Header{}, got)
				require.Equal(t, before, data)
			}
			got = core.Header{}
			rest, err := core.DecodeHeaderFirst(data, &got)
			checkError(err)
			require.Nil(t, rest)
			checkError(core.DecodeHeader(data, &got))
		})
	}
}

// A distinct named type selects the original planner through the public API.
// It has the exact Header field types and tags, without a generated registration.
type plannerHeader core.Header

func FuzzTypedHeaderMatchesPlanner(f *testing.F) {
	for _, data := range [][]byte{{0xa0}, {0xf6}, {0xf7}, {0xa2, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', 7}, {0xa1, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', 7}, {0xa2, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', 1, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', 2}} {
		f.Add(data)
	}
	data, err := cbor.Marshal(core.Header{Number: 128, ProtocolVersion: "0.13", L1DataGasPrice: &core.GasPrice{PriceInWei: felt.NewFromUint64[felt.Felt](7)}})
	require.NoError(f, err)
	f.Add(data)
	f.Fuzz(func(t *testing.T, data []byte) {
		var got core.Header
		var want plannerHeader
		gr, ge := core.DecodeHeaderFirst(data, &got)
		wr, we := fastcbor.UnmarshalFirst(data, &want)
		require.Equal(t, we == nil, ge == nil)
		require.Equal(t, core.Header(want), got)
		require.Equal(t, wr, gr)
		if we != nil {
			require.Equal(t, we.Error(), ge.Error())
		}
	})
}

// Generate valid values before encoding so limb, string and nested-slice paths
// receive coverage independently of the byte-mutation fuzzer's seed corpus.
func FuzzTypedHeaderRoundTrip(f *testing.F) {
	for _, seed := range []string{"nil", "empty", "populated", "legacy gas keys"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, seed []byte) {
		rng := rand.New(rand.NewChaCha8(sha256.Sum256(seed)))
		nextFelt := func() *felt.Felt {
			if rng.IntN(4) == 0 {
				return nil
			}
			var b [32]byte
			for i := range b {
				b[i] = byte(rng.Uint32())
			}
			return new(felt.Felt).SetBytes(b[:])
		}
		nextGas := func() *core.GasPrice {
			if rng.IntN(3) == 0 {
				return nil
			}
			return &core.GasPrice{PriceInWei: nextFelt(), PriceInFri: nextFelt()}
		}
		want := core.Header{
			Hash: nextFelt(), ParentHash: nextFelt(), Number: rng.Uint64(), GlobalStateRoot: nextFelt(),
			SequencerAddress: nextFelt(), TransactionCount: rng.Uint64(), EventCount: rng.Uint64(), Timestamp: rng.Uint64(),
			L1GasPriceETH: nextFelt(), L1GasPriceSTRK: nextFelt(), L1DAMode: core.L1DAMode(rng.Uint64()),
			L1DataGasPrice: nextGas(), L2GasPrice: nextGas(),
		}
		alphabet := []rune("0.13alphaα世界\x00")
		var version []rune
		for n := rng.IntN(40); n > 0; n-- {
			version = append(version, alphabet[rng.IntN(len(alphabet))])
		}
		want.ProtocolVersion = string(version)
		if rng.IntN(3) != 0 {
			want.Signatures = make([][]*felt.Felt, rng.IntN(6))
			for i := range want.Signatures {
				if rng.IntN(3) == 0 {
					continue
				}
				want.Signatures[i] = make([]*felt.Felt, rng.IntN(6))
				for j := range want.Signatures[i] {
					want.Signatures[i][j] = nextFelt()
				}
			}
		}
		data, err := cbor.Marshal(want)
		require.NoError(t, err)
		var got, fx core.Header
		require.NoError(t, core.DecodeHeader(data, &got))
		require.NoError(t, cbor.Unmarshal(data, &fx))
		require.Equal(t, want, got)
		require.Equal(t, fx, got)
		var planned plannerHeader
		require.NoError(t, fastcbor.Unmarshal(data, &planned))
		require.Equal(t, core.Header(planned), got)
	})
}
