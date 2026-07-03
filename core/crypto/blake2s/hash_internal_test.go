package blake2s

import (
	"testing"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

func TestUnsafeConversion(t *testing.T) {
	val := []*[4]uint64{
		{10, 12, 14, 16},
		{15, 20, 21, 23},
		{17, 19, 23, 29},
	}

	felts := unsafe.Slice((**felt.Felt)(unsafe.Pointer(&val[0])), len(val))
	for i, f := range felts {
		assert.Equal(t, *val[i], [4]uint64(*f))
	}
}
