package starknet_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/adapters/core2p2p"
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
				Hash: core2p2p.AdaptHash(hash),
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
			assert.ErrorIs(t, err, someErr)
		})
		t.Run("ok", func(t *testing.T) {
			height := uint64(888)
			block := &core.Block{
				Receipts: []*core.TransactionReceipt{
					{
						Events: []*core.Event{
							{
								From: randFelt(t),
								Keys: randFeltSlice(t, 1),
								Data: randFeltSlice(t, 1),
							},
						},
					},
					{
						Events: []*core.Event{
							{
								From: randFelt(t),
								Keys: randFeltSlice(t, 2),
								Data: randFeltSlice(t, 2),
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
						FromAddress: core2p2p.AdaptFelt(event1.From),
						Keys:        utils.Map(event1.Keys, core2p2p.AdaptFelt),
						Data:        utils.Map(event1.Data, core2p2p.AdaptFelt),
					},
					{
						FromAddress: core2p2p.AdaptFelt(event2.From),
						Keys:        utils.Map(event2.Keys, core2p2p.AdaptFelt),
						Data:        utils.Map(event2.Data, core2p2p.AdaptFelt),
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

func randFeltSlice(t *testing.T, n int) []*felt.Felt {
	t.Helper()

	sl := make([]*felt.Felt, n)
	for i := range sl {
		sl[i] = randFelt(t)
	}

	return sl
}

func randFelt(t *testing.T) *felt.Felt {
	t.Helper()

	f, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	return f
}
