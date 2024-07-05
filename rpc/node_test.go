package rpc_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNodesFromRoot(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, "", nil)
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockTrie := mocks.NewMockClassesTrie(mockCtrl)
	zero := trie.NewKey(0, []byte{})

	t.Run("Empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)

		nodes, err := handler.NodesFromRoot(&felt.Zero)
		require.Nil(t, nodes)
		assert.Equal(t, jsonrpc.InternalError, err.Code)
	})

	t.Run("Key not found", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(mockState, nil, nil)
		mockState.EXPECT().ClassesTrie().Return(nil, nil, db.ErrKeyNotFound)

		nodes, err := handler.NodesFromRoot(&felt.Zero)
		require.Nil(t, nodes)
		assert.Equal(t, jsonrpc.InternalError, err.Code)
	})

	t.Run("trieInstance.NodesFromRoot error", func(t *testing.T) {
		errUnmarshal := errors.New("size of input data is less than felt size")
		mockReader.EXPECT().HeadState().Return(mockState, nil, nil)
		mockState.EXPECT().ClassesTrie().Return(mockTrie, nil, nil)
		mockTrie.EXPECT().FeltToKey(&felt.Zero).Return(zero)
		mockTrie.EXPECT().NodesFromRoot(&zero).Return(nil, errUnmarshal)

		nodes, err := handler.NodesFromRoot(&felt.Zero)
		require.Nil(t, nodes)
		assert.Equal(t, jsonrpc.InternalError, err.Code)
	})

	t.Run("OK zero", func(t *testing.T) {
		nodes := []trie.StorageNode{
			*trie.NewStorageNode(&zero, &trie.Node{}),
		}

		mockReader.EXPECT().HeadState().Return(mockState, nil, nil)
		mockState.EXPECT().ClassesTrie().Return(mockTrie, nil, nil)
		mockTrie.EXPECT().FeltToKey(&felt.Zero).Return(zero)
		mockTrie.EXPECT().NodesFromRoot(&zero).Return(nodes, nil)

		resp, err := handler.NodesFromRoot(&felt.Zero)
		require.Nil(t, err)
		assert.Equal(t, nodes, resp)
	})
}
