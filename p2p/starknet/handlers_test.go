package starknet_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestHandleGetSignatures(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	log := utils.NewNopZapLogger()
	mockReader := mocks.NewMockReader(mockCtrl)

	h := starknet.NewHandler(mockReader, log)

	hash := new(felt.Felt).SetUint64(77)
	block := &core.Block{
		Transactions: []core.Transaction{
			&core.InvokeTransaction{
				TransactionSignature: randFeltSlice(3),
			},
			&core.DeclareTransaction{
				TransactionSignature: randFeltSlice(2),
			},
		},
	}
	mockReader.EXPECT().BlockByHash(hash).Return(block, nil)

	blockID := &spec.BlockID{
		Hash: starknet.AdaptFeltToHash(hash),
	}
	response, err := h.HandleGetSignatures(&spec.GetSignatures{
		Id: blockID,
	})
	require.NoError(t, err)

	expected := &spec.Signatures{
		Id: blockID,
		Signatures: utils.Map(block.Transactions, func(tx core.Transaction) *spec.Signature {
			return &spec.Signature{
				Parts: utils.Map(tx.Signature(), starknet.AdaptFelt),
			}
		}),
	}
	assert.True(t, proto.Equal(expected, response))
}

func randFeltSlice(n int) []*felt.Felt {
	sl := make([]*felt.Felt, n)
	for i := range sl {
		sl[i] = randFelt()
	}

	return sl
}

func randFelt() *felt.Felt {
	f, err := new(felt.Felt).SetRandom()
	if err != nil {
		panic(err)
	}

	return f
}
