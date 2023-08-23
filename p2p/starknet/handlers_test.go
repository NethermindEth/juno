package starknet_test

import (
	"errors"
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

func TestHandleGetEvents(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	log := utils.NewNopZapLogger()
	mockReader := mocks.NewMockReader(mockCtrl)

	h := starknet.NewHandler(mockReader, log)

	t.Run("nil block id", func(t *testing.T) {
		_, err := h.HandleGetEvents(&spec.GetEvents{})
		assert.Error(t, err)
	})
	t.Run("block by hash", func(t *testing.T) {
		hash := new(felt.Felt).SetUint64(77)
		mockReader.EXPECT().BlockByHash(hash).Return(&core.Block{}, nil)

		expected := &spec.Events{}

		events, err := h.HandleGetEvents(&spec.GetEvents{
			Id: &spec.BlockID{
				Hash: starknet.AdaptFeltToHash(hash),
			},
		})
		require.NoError(t, err)
		assert.True(t, proto.Equal(expected, events))
	})
	t.Run("block by height", func(t *testing.T) {
		t.Run("failure", func(t *testing.T) {
			height := uint64(777)
			someErr := errors.New("some error")
			mockReader.EXPECT().BlockByNumber(height).Return(nil, someErr)

			_, err := h.HandleGetEvents(&spec.GetEvents{
				Id: &spec.BlockID{
					Height: height,
				},
			})
			assert.True(t, errors.Is(err, someErr))
		})
		t.Run("ok", func(t *testing.T) {
			height := uint64(888)
			block := &core.Block{
				Receipts: []*core.TransactionReceipt{
					{
						Events: []*core.Event{
							{
								From: randFelt(),
								Keys: randFeltSlice(1),
								Data: randFeltSlice(1),
							},
						},
					},
					{
						Events: []*core.Event{
							{
								From: randFelt(),
								Keys: randFeltSlice(2),
								Data: randFeltSlice(2),
							},
						},
					},
				},
			}
			mockReader.EXPECT().BlockByNumber(height).Return(block, nil)

			event1 := block.Receipts[0].Events[0]
			event2 := block.Receipts[1].Events[0]
			expected := &spec.Events{
				Events: []*spec.Event{
					{
						FromAddress: starknet.AdaptFelt(event1.From),
						Keys:        utils.Map(event1.Keys, starknet.AdaptFelt),
						Data:        utils.Map(event1.Data, starknet.AdaptFelt),
					},
					{
						FromAddress: starknet.AdaptFelt(event2.From),
						Keys:        utils.Map(event2.Keys, starknet.AdaptFelt),
						Data:        utils.Map(event2.Data, starknet.AdaptFelt),
					},
				},
			}

			result, err := h.HandleGetEvents(&spec.GetEvents{
				Id: &spec.BlockID{
					Height: height,
				},
			})
			require.NoError(t, err)
			assert.True(t, proto.Equal(expected, result))
		})
	})
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
