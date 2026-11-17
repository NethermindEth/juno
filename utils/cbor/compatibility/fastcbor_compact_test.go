package cbor_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// feltForms returns both stored forms independently of the linked encoder.
func feltForms(value felt.Felt) map[string][]byte {
	legacy := []byte{0x84}
	for _, limb := range value {
		legacy = binary.BigEndian.AppendUint64(append(legacy, 0x1b), limb)
	}
	canonical := value.MinimalBytes()
	var compact []byte
	switch {
	case len(canonical) == 1 && canonical[0] < 24:
		compact = canonical
	case len(canonical) <= 30:
		compact = append([]byte{0x40 | byte(len(canonical))}, canonical...)
		if len(canonical) >= 24 {
			compact = append([]byte{0x58, byte(len(canonical))}, canonical...)
		}
	default:
		compact = []byte{0x58, 32}
		for i := len(value) - 1; i >= 0; i-- {
			compact = binary.BigEndian.AppendUint64(compact, value[i])
		}
	}
	return map[string][]byte{"legacy": legacy, "compact": compact}
}

func TestFeltWidthsAcrossReaders(t *testing.T) {
	maximum := felt.One[felt.Felt]()
	maximum.Neg(&maximum)
	values := []felt.Felt{
		felt.Zero[felt.Felt](), felt.One[felt.Felt](),
		felt.FromUint64[felt.Felt](23), felt.FromUint64[felt.Felt](24),
		felt.FromUint64[felt.Felt](^uint64(0)), maximum,
	}
	for _, width := range []int{9, 17, 25, 30, 31, 32} {
		raw := make([]byte, width)
		raw[0], raw[width-1] = 1, 0xff
		values = append(values, felt.FromBytes[felt.Felt](raw))
	}
	for i, value := range values {
		for form, data := range feltForms(value) {
			t.Run(fmt.Sprintf("%d/%s", i, form), func(t *testing.T) {
				var fast, reference, got felt.Felt
				require.NoError(t, fastcbor.Unmarshal(data, &fast))
				require.NoError(t, fxamacker.Unmarshal(data, &reference))
				c := cursor.NewCursor(data)
				require.NoError(t, cursor.Felt(&c, &got))
				require.Zero(t, c.Len())
				require.Equal(t, value, fast)
				require.Equal(t, value, reference)
				require.Equal(t, value, got)

				var like alias
				require.NoError(t, fastcbor.Unmarshal(data, &like))
				require.Equal(t, alias(value), like)
				var address felt.Address
				require.NoError(t, fastcbor.Unmarshal(data, &address))
				require.Equal(t, felt.Address(value), address)

				slice := append([]byte{0x82}, data...)
				slice = append(slice, data...)
				var list []felt.Felt
				require.NoError(t, fastcbor.Unmarshal(slice, &list))
				require.Equal(t, []felt.Felt{value, value}, list)
				c = cursor.NewCursor(slice)
				limbs, err := cursor.LimbSlice[felt.Felt](&c)
				require.NoError(t, err)
				require.Equal(t, list, limbs)

				mapping := append([]byte{0xa1}, data...)
				mapping = append(mapping, data...)
				var pairs map[felt.Felt]felt.Felt
				require.NoError(t, fastcbor.Unmarshal(mapping, &pairs))
				require.Equal(t, map[felt.Felt]felt.Felt{value: value}, pairs)
			})
		}
	}
}

// BenchmarkFeltDecode compares readers on the form selected by the build tag.
func BenchmarkFeltDecode(b *testing.B) {
	value := felt.FromUint64[felt.Felt](1 << 40)
	values := make([]felt.Felt, 64)
	pairs := make(map[felt.Felt]felt.Felt, len(values))
	for i := range values {
		values[i] = felt.FromUint64[felt.Felt](uint64(i) + 1<<40)
		pairs[values[i]] = value
	}
	benchmarkFeltDecode(b, "single", value)
	benchmarkFeltDecode(b, "slice", values)
	benchmarkFeltDecode(b, "map", pairs)
}

func benchmarkFeltDecode[T any](b *testing.B, name string, value T) {
	data, err := fxamacker.Marshal(value)
	require.NoError(b, err)
	for _, reader := range []struct {
		name string
		read func([]byte, *T) error
	}{
		{"fastcbor", fastcbor.Unmarshal[T]},
		{"fxamacker", func(data []byte, dst *T) error { return fxamacker.Unmarshal(data, dst) }},
	} {
		b.Run(name+"/"+reader.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var dst T
				if err := reader.read(data, &dst); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
