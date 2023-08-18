package starknet

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByteReader(t *testing.T) {
	buffer := bytes.NewBuffer([]byte{1, 2, 3, 4})
	bReader := byteReader{buffer}

	read, err := bReader.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(1), read)

	read, err = bReader.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(2), read)

	readAll, err := io.ReadAll(bReader)
	require.NoError(t, err)
	assert.Equal(t, []byte{3, 4}, readAll)
}
