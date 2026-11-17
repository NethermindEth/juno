package cursor_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
)

// BenchmarkReadCompactFelt reads one felt of each compact form, with the Cursor and with core/felt.
func BenchmarkReadCompactFelt(b *testing.B) {
	forms := []struct {
		name string
		data []byte
	}{
		{"uint", []byte{0x17}},
		{"1 byte", []byte{0x41, 0xff}},
		{"8 bytes", byteStringOf(trimmed(8))},
		{"30 bytes", byteStringOf(trimmed(30))},
		{"limbs", byteStringOf(append([]byte{0x01}, trimmed(31)...))},
	}
	for _, form := range forms {
		b.Run(form.name+"/cursor", func(b *testing.B) {
			var dst [cursor.Limbs]uint64
			for range b.N {
				c := cursor.NewCursor(form.data)
				if err := c.Limbs(&dst); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(form.name+"/codec", func(b *testing.B) {
			var dst felt.Felt
			for range b.N {
				if err := dst.UnmarshalCBOR(form.data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
