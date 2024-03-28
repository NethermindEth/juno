package rpc_test

import (
	"testing"

	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestChainId(t *testing.T) {
	for _, n := range []utils.Network{utils.Mainnet, utils.Goerli, utils.Goerli2, utils.Integration} {
		t.Run(n.String(), func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			mockReader := mocks.NewMockReader(mockCtrl)
			mockReader.EXPECT().Network().Return(&n)
			handler := rpc.New(mockReader, nil, nil, "", nil)

			cID, err := handler.ChainID()
			require.Nil(t, err)
			assert.Equal(t, n.L2ChainIDFelt(), cID)
		})
	}
}
