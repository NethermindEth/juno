package server

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestIterator(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	t.Run("iterator by number", func(t *testing.T) {
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
				newBlock(1, nil),
				newBlock(3, nil),
				newBlock(5, nil),
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
			assert.Equal(t, len(blocks), i)
		})
		t.Run("backward", func(t *testing.T) {
			reader := mocks.NewMockReader(mockCtrl)
			it, err := newIteratorByNumber(reader, 10, 3, 2, false)
			require.NoError(t, err)

			blocks := []*core.Block{
				newBlock(10, nil),
				newBlock(8, nil),
				newBlock(6, nil),
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
			assert.Equal(t, len(blocks), i)
		})
	})
	t.Run("iterator by hash", func(t *testing.T) {
		t.Run("wrong args", func(t *testing.T) {
			reader := mocks.NewMockReader(mockCtrl)
			// zero limit
			hash := felt.NewRandom[felt.Felt]()
			reader.EXPECT().BlockByHash(hash).Return(&core.Block{Header: &core.Header{Number: 1}}, nil)
			_, err := newIteratorByHash(reader, hash, 0, 1, false)
			assert.Error(t, err)

			// zero step
			hash = felt.NewRandom[felt.Felt]()
			reader.EXPECT().BlockByHash(hash).Return(&core.Block{Header: &core.Header{Number: 2}}, nil)
			_, err = newIteratorByHash(reader, hash, 1, 0, false)
			assert.Error(t, err)

			// nil hash
			_, err = newIteratorByHash(reader, nil, 1, 1, false)
			assert.Error(t, err)
		})
		t.Run("iteration", func(t *testing.T) {
			firstBlockHash := felt.NewRandom[felt.Felt]()
			blocks := []*core.Block{
				newBlock(1, firstBlockHash),
				newBlock(2, felt.NewRandom[felt.Felt]()),
				newBlock(3, felt.NewRandom[felt.Felt]()),
			}

			reader := mocks.NewMockReader(mockCtrl)
			reader.EXPECT().BlockByHash(firstBlockHash).Return(blocks[0], nil)

			it, err := newIteratorByHash(reader, firstBlockHash, 3, 1, true)
			require.NoError(t, err)

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
			assert.Equal(t, len(blocks), i)
		})
	})
}

func newBlock(number uint64, hash *felt.Felt) *core.Block {
	return &core.Block{
		Header: &core.Header{
			Number: number,
			Hash:   hash,
		},
	}
}
