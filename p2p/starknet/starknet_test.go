package starknet_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestClientHandler(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	testNetwork := utils.INTEGRATION
	testCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockNet, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)

	peers := mockNet.Peers()
	require.Len(t, peers, 2)
	handlerID := peers[0]
	clientID := peers[1]

	log := utils.NewNopZapLogger()
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := starknet.NewHandler(mockReader, log)

	handlerHost := mockNet.Host(handlerID)
	handlerHost.SetStreamHandler(starknet.BlockHeadersPID(testNetwork), handler.BlockHeadersHandler)
	handlerHost.SetStreamHandler(starknet.BlockBodiesPID(testNetwork), handler.BlockBodiesHandler)
	handlerHost.SetStreamHandler(starknet.EventsPID(testNetwork), handler.EventsHandler)
	handlerHost.SetStreamHandler(starknet.ReceiptsPID(testNetwork), handler.ReceiptsHandler)
	handlerHost.SetStreamHandler(starknet.TransactionsPID(testNetwork), handler.TransactionsHandler)

	clientHost := mockNet.Host(clientID)
	client := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return clientHost.NewStream(ctx, handlerID, pids...)
	}, testNetwork, log)

	t.Run("get block headers", func(t *testing.T) {
		type pair struct {
			header      *core.Header
			commitments *core.BlockCommitments
		}
		pairsPerBlock := []pair{}
		for i := uint64(0); i < 2; i++ {
			pairsPerBlock = append(pairsPerBlock, pair{
				header: &core.Header{
					Number:           i,
					ParentHash:       randFelt(t),
					Timestamp:        i,
					SequencerAddress: randFelt(t),
					GlobalStateRoot:  randFelt(t),
					TransactionCount: i,
					EventCount:       i,
					Hash:             randFelt(t),
				},
				commitments: &core.BlockCommitments{
					TransactionCommitment: randFelt(t),
					EventCommitment:       randFelt(t),
				},
			})
		}

		for blockNumber, pair := range pairsPerBlock {
			blockNumber := uint64(blockNumber)
			mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(pair.header, nil)
			mockReader.EXPECT().BlockCommitmentsByNumber(blockNumber).Return(pair.commitments, nil)
		}

		numOfBlocks := uint64(len(pairsPerBlock))
		res, cErr := client.RequestBlockHeaders(testCtx, &spec.BlockHeadersRequest{
			Iteration: &spec.Iteration{
				Start: &spec.Iteration_BlockNumber{
					BlockNumber: 0,
				},
				Direction: spec.Iteration_Forward,
				Limit:     numOfBlocks,
				Step:      1,
			},
		})
		require.NoError(t, cErr)

		var count uint64
		for response, valid := res(); valid; response, valid = res() {
			if count == numOfBlocks {
				assert.True(t, proto.Equal(&spec.Fin{}, response.Part[0].GetFin()))
				count++
				break
			}

			adaptHash := core2p2p.AdaptHash
			expectedPair := pairsPerBlock[count]
			header := expectedPair.header

			expectedResponse := &spec.BlockHeadersResponse{
				Part: []*spec.BlockHeadersResponsePart{
					{
						HeaderMessage: &spec.BlockHeadersResponsePart_Header{
							Header: &spec.BlockHeader{
								ParentHeader:     adaptHash(header.ParentHash),
								Number:           header.Number,
								Time:             timestamppb.New(time.Unix(int64(header.Timestamp), 0)),
								SequencerAddress: core2p2p.AdaptAddress(header.SequencerAddress),
								State: &spec.Patricia{
									Height: uint32(header.Number),
									Root:   adaptHash(header.GlobalStateRoot),
								},
								Transactions: &spec.Merkle{
									NLeaves: uint32(header.TransactionCount),
									Root:    adaptHash(expectedPair.commitments.TransactionCommitment),
								},
								Events: &spec.Merkle{
									NLeaves: uint32(header.EventCount),
									Root:    adaptHash(expectedPair.commitments.EventCommitment),
								},
							},
						},
					},
					{
						HeaderMessage: &spec.BlockHeadersResponsePart_Signatures{
							Signatures: &spec.Signatures{
								Block:      core2p2p.AdaptBlockID(expectedPair.header),
								Signatures: utils.Map(expectedPair.header.Signatures, core2p2p.AdaptSignature),
							},
						},
					},
				},
			}
			assert.True(t, proto.Equal(expectedResponse, response))

			assert.Equal(t, count, response.Part[0].GetHeader().Number)
			count++
		}

		expectedCount := numOfBlocks + 1 // plus fin
		require.Equal(t, expectedCount, count)
	})

	t.Run("get block bodies", func(t *testing.T) {
		res, cErr := client.RequestBlockBodies(testCtx, &spec.BlockBodiesRequest{})
		require.NoError(t, cErr)

		count := uint64(0)
		for body, valid := res(); valid; body, valid = res() {
			assert.Equal(t, count, body.Id.Number)
			count++
		}
		require.Equal(t, uint64(4), count)
	})

	t.Run("get receipts", func(t *testing.T) {
		res, cErr := client.RequestReceipts(testCtx, &spec.ReceiptsRequest{})
		require.NoError(t, cErr)

		count := uint64(0)
		for receipt, valid := res(); valid; receipt, valid = res() {
			assert.Equal(t, count, receipt.Id.Number)
			count++
		}
		require.Equal(t, uint64(4), count)
	})

	t.Run("get txns", func(t *testing.T) {
		res, cErr := client.RequestTransactions(testCtx, &spec.TransactionsRequest{})
		require.NoError(t, cErr)

		count := uint64(0)
		for txn, valid := res(); valid; txn, valid = res() {
			assert.Equal(t, count, txn.Id.Number)
			count++
		}
		require.Equal(t, uint64(4), count)
	})

	t.Run("get events", func(t *testing.T) {
		eventsPerBlock := [][]*core.Event{
			{}, // block with no events
			{
				{
					From: randFelt(t),
					Data: feltSlice(t, 1, randFelt),
					Keys: feltSlice(t, 1, randFelt),
				},
			},
			{
				{
					From: randFelt(t),
					Data: feltSlice(t, 2, randFelt),
					Keys: feltSlice(t, 2, randFelt),
				},
				{
					From: randFelt(t),
					Data: feltSlice(t, 3, randFelt),
					Keys: feltSlice(t, 3, randFelt),
				},
			},
		}
		for blockNumber, events := range eventsPerBlock {
			blockNumber := uint64(blockNumber)
			mockReader.EXPECT().BlockByNumber(blockNumber).Return(&core.Block{
				Header: &core.Header{
					Number: blockNumber,
				},
				Receipts: []*core.TransactionReceipt{
					{Events: events},
				},
			}, nil)
		}

		numOfBlocks := uint64(len(eventsPerBlock))
		res, cErr := client.RequestEvents(testCtx, &spec.EventsRequest{
			Iteration: &spec.Iteration{
				Start: &spec.Iteration_BlockNumber{
					BlockNumber: 0,
				},
				Direction: spec.Iteration_Forward,
				Limit:     numOfBlocks,
				Step:      1,
			},
		})
		require.NoError(t, cErr)

		var count uint64
		for evnt, valid := res(); valid; evnt, valid = res() {
			if count == numOfBlocks {
				assert.True(t, proto.Equal(&spec.Fin{}, evnt.GetFin()))
				count++
				break
			}

			assert.Equal(t, count, evnt.Id.Number)

			passedEvents := eventsPerBlock[int(count)]
			expectedEventsResponse := &spec.EventsResponse_Events{
				Events: &spec.Events{
					Items: utils.Map(passedEvents, func(e *core.Event) *spec.Event {
						adaptFelt := core2p2p.AdaptFelt
						return &spec.Event{
							FromAddress: adaptFelt(e.From),
							Keys:        utils.Map(e.Keys, adaptFelt),
							Data:        utils.Map(e.Data, adaptFelt),
						}
					}),
				},
			}

			assert.True(t, proto.Equal(expectedEventsResponse.Events, evnt.GetEvents()))
			count++
		}
		expectedCount := numOfBlocks + 1 // numOfBlocks messages with blocks + 1 fin message
		require.Equal(t, expectedCount, count)
	})
}

func feltSlice(t *testing.T, n int, generator func(*testing.T) *felt.Felt) []*felt.Felt {
	sl := make([]*felt.Felt, n)
	for i := range sl {
		sl[i] = generator(t)
	}

	return sl
}

func randFelt(t *testing.T) *felt.Felt {
	t.Helper()

	f, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	return f
}
