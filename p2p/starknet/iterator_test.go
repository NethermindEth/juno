package starknet

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestIterator(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	t.Run("wrong args", func(t *testing.T) {
		// zero limit
		_, err := newIteratorByNumber(nil, 1, 0, 1, false)
		assert.Error(t, err)

		// zero step
		_, err = newIteratorByNumber(nil, 1, 1, 0, false)
		assert.Error(t, err)
	})
	t.Run("forward", func(t *testing.T) {
		reader := mocks.NewMockReader(mockCtrl)
		it, err := newIteratorByNumber(reader, 1, 10, 2, true)
		require.NoError(t, err)

		blocks := []*core.Block{
			newBlock(1),
			newBlock(3),
			newBlock(5),
		}

		for _, block := range blocks {
			reader.EXPECT().BlockByNumber(block.Number).Return(block, nil)
		}
		reader.EXPECT().BlockByNumber(uint64(7)).Return(nil, db.ErrKeyNotFound)

		var i int
		for it.Valid() {
			block, err := it.Block()
			if err != nil {
				assert.Equal(t, err, db.ErrKeyNotFound)
				continue
			}
			assert.Equal(t, blocks[i], block)

			it.Next()
			i++
		}
	})
	t.Run("backward", func(t *testing.T) {
		reader := mocks.NewMockReader(mockCtrl)
		it, err := newIteratorByNumber(reader, 10, 3, 2, false)
		require.NoError(t, err)

		blocks := []*core.Block{
			newBlock(10),
			newBlock(8),
			newBlock(6),
		}

		for _, block := range blocks {
			reader.EXPECT().BlockByNumber(block.Number).Return(block, nil)
		}

		var i int
		for it.Valid() {
			block, err := it.Block()
			if err != nil {
				assert.Equal(t, err, db.ErrKeyNotFound)
				continue
			}
			assert.Equal(t, blocks[i], block)

			it.Next()
			i++
		}
	})
}

func newBlock(number uint64) *core.Block {
	return &core.Block{
		Header: &core.Header{
			Number: number,
		},
	}
}
