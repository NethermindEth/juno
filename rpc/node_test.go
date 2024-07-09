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
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockTrie := mocks.NewMockClassesTrie(mockCtrl)

	handler := rpc.New(mockReader, nil, nil, "", nil)
	zero := trie.NewKey(0, []byte{})
	errUnmarshal := errors.New("size of input data is less than felt size")
	nodes := []trie.StorageNode{*trie.NewStorageNode(&zero, &trie.Node{})}

	type mockBehavior func(*mocks.MockReader, *mocks.MockStateHistoryReader, *mocks.MockClassesTrie)
	type testCase struct {
		name         string
		input        *felt.Felt
		mockBehavior mockBehavior
		isErr        bool
	}

	testTable := []testCase{
		{
			name:  "Empty blockchain",
			input: &felt.Zero,
			mockBehavior: func(mr *mocks.MockReader, _ *mocks.MockStateHistoryReader, _ *mocks.MockClassesTrie) {
				mr.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)
			},
			isErr: true,
		},
		{
			name:  "Key not found",
			input: &felt.Zero,
			mockBehavior: func(mr *mocks.MockReader, mshr *mocks.MockStateHistoryReader, _ *mocks.MockClassesTrie) {
				mr.EXPECT().HeadState().Return(mshr, nil, nil)
				mshr.EXPECT().ClassesTrie().Return(nil, nil, db.ErrKeyNotFound)
			},
			isErr: true,
		},
		{
			name:  "trieInstance.NodesFromRoot error",
			input: &felt.Zero,
			mockBehavior: func(mr *mocks.MockReader, mshr *mocks.MockStateHistoryReader, mct *mocks.MockClassesTrie) {
				mr.EXPECT().HeadState().Return(mshr, nil, nil)
				mshr.EXPECT().ClassesTrie().Return(mockTrie, nil, nil)
				mct.EXPECT().FeltToKey(&felt.Zero).Return(zero)
				mct.EXPECT().NodesFromRoot(&zero).Return(nil, errUnmarshal)
			},
			isErr: true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			test.mockBehavior(mockReader, mockState, mockTrie)
			resp, err := handler.NodesFromRoot(test.input)
			if test.isErr {
				require.Nil(t, resp)
				assert.Equal(t, jsonrpc.InternalError, err.Code)
			} else {
				require.Nil(t, err)
				assert.Equal(t, nodes, resp)
			}
		})
	}
}
