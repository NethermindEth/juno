package blockchain

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBlockDbKey(t *testing.T) {
	bytes := [32]byte{}
	for i := 0; i < 32; i++ {
		bytes[i] = byte(i + 1)
	}
	key := &blockDbKey{
		Number: 44,
		Hash:   new(felt.Felt).SetBytes(bytes[:]),
	}

	keyB, err := key.MarshalBinary()
	expectedKeyB := []byte{byte(db.Blocks), 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 44, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9,
		0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c,
		0x1d, 0x1e, 0x1f, 0x20}
	assert.Equal(t, expectedKeyB, keyB)
	assert.NoError(t, err)
	keyUnmarshaled := new(blockDbKey)
	keyUnmarshaled.UnmarshalBinary(keyB)
	assert.Equal(t, key, keyUnmarshaled)

	keyB[0] = byte(db.State)
	assert.EqualError(t, keyUnmarshaled.UnmarshalBinary(keyB), "wrong prefix")
	keyB = append(keyB, 0)
	assert.EqualError(t, keyUnmarshaled.UnmarshalBinary(keyB), "key should be 41 bytes long")
}
