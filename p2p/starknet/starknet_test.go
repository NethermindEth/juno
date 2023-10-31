package starknet_test

import (
	"context"
	"reflect"
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

	log, err := utils.NewZapLogger(utils.ERROR, false)
	require.NoError(t, err)
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
				header: fillFelts(t, &core.Header{
					Number:           i,
					Timestamp:        i,
					TransactionCount: i,
					EventCount:       i,
				}),
				commitments: fillFelts(t, &core.BlockCommitments{}),
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
									Height: 251,
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
		txH := randFelt(t)
		// There are common receipt fields shared by all of different transactions.
		commonReceipt := &core.TransactionReceipt{
			TransactionHash: txH,
			Fee:             randFelt(t),
			L2ToL1Message:   []*core.L2ToL1Message{fillFelts(t, &core.L2ToL1Message{}), fillFelts(t, &core.L2ToL1Message{})},
			ExecutionResources: &core.ExecutionResources{
				BuiltinInstanceCounter: core.BuiltinInstanceCounter{
					Pedersen:   1,
					RangeCheck: 2,
					Bitwise:    3,
					Output:     4,
					Ecsda:      5,
					EcOp:       6,
					Keccak:     7,
					Poseidon:   8,
				},
				MemoryHoles: 9,
				Steps:       10,
			},
			RevertReason:  "some revert reason",
			Events:        []*core.Event{fillFelts(t, &core.Event{}), fillFelts(t, &core.Event{})},
			L1ToL2Message: fillFelts(t, &core.L1ToL2Message{}),
		}

		specReceiptCommon := &spec.Receipt_Common{
			TransactionHash:    core2p2p.AdaptHash(commonReceipt.TransactionHash),
			ActualFee:          core2p2p.AdaptFelt(commonReceipt.Fee),
			MessagesSent:       utils.Map(commonReceipt.L2ToL1Message, core2p2p.AdaptMessageToL1),
			ExecutionResources: core2p2p.AdaptExecutionResources(commonReceipt.ExecutionResources),
			RevertReason:       commonReceipt.RevertReason,
		}

		invokeTx := &core.InvokeTransaction{TransactionHash: txH}
		expectedInvoke := &spec.Receipt{
			Receipt: &spec.Receipt_Invoke_{
				Invoke: &spec.Receipt_Invoke{
					Common: specReceiptCommon,
				},
			},
		}

		declareTx := &core.DeclareTransaction{TransactionHash: txH}
		expectedDeclare := &spec.Receipt{
			Receipt: &spec.Receipt_Declare_{
				Declare: &spec.Receipt_Declare{
					Common: specReceiptCommon,
				},
			},
		}

		l1Txn := &core.L1HandlerTransaction{
			TransactionHash:    txH,
			CallData:           []*felt.Felt{new(felt.Felt).SetBytes([]byte("calldata 1")), new(felt.Felt).SetBytes([]byte("calldata 2"))},
			ContractAddress:    new(felt.Felt).SetBytes([]byte("contract address")),
			EntryPointSelector: new(felt.Felt).SetBytes([]byte("entry point selector")),
			Nonce:              new(felt.Felt).SetBytes([]byte("nonce")),
		}
		expectedL1Handler := &spec.Receipt{
			Receipt: &spec.Receipt_L1Handler_{
				L1Handler: &spec.Receipt_L1Handler{
					Common:  specReceiptCommon,
					MsgHash: &spec.Hash{Elements: l1Txn.MessageHash()},
				},
			},
		}

		deployAccTxn := &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash: txH,
				ContractAddress: new(felt.Felt).SetBytes([]byte("contract address")),
			},
		}
		expectedDeployAccount := &spec.Receipt{
			Receipt: &spec.Receipt_DeployAccount_{
				DeployAccount: &spec.Receipt_DeployAccount{
					Common:          specReceiptCommon,
					ContractAddress: core2p2p.AdaptFelt(deployAccTxn.ContractAddress),
				},
			},
		}

		deployTxn := &core.DeployTransaction{
			TransactionHash: txH,
			ContractAddress: new(felt.Felt).SetBytes([]byte("contract address")),
		}
		expectedDeploy := &spec.Receipt{
			Receipt: &spec.Receipt_DeprecatedDeploy{
				DeprecatedDeploy: &spec.Receipt_Deploy{
					Common:          specReceiptCommon,
					ContractAddress: core2p2p.AdaptFelt(deployTxn.ContractAddress),
				},
			},
		}

		tests := []struct {
			b          *core.Block
			expectedRs *spec.Receipts
		}{
			{
				b: &core.Block{
					Header:       &core.Header{Number: 0, Hash: randFelt(t)},
					Transactions: []core.Transaction{invokeTx},
					Receipts:     []*core.TransactionReceipt{commonReceipt},
				},
				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedInvoke}},
			},
			{
				b: &core.Block{
					Header:       &core.Header{Number: 1, Hash: randFelt(t)},
					Transactions: []core.Transaction{declareTx},
					Receipts:     []*core.TransactionReceipt{commonReceipt},
				},
				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedDeclare}},
			},
			{
				b: &core.Block{
					Header:       &core.Header{Number: 2, Hash: randFelt(t)},
					Transactions: []core.Transaction{l1Txn},
					Receipts:     []*core.TransactionReceipt{commonReceipt},
				},
				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedL1Handler}},
			},
			{
				b: &core.Block{
					Header:       &core.Header{Number: 3, Hash: randFelt(t)},
					Transactions: []core.Transaction{deployAccTxn},
					Receipts:     []*core.TransactionReceipt{commonReceipt},
				},
				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedDeployAccount}},
			},
			{
				b: &core.Block{
					Header:       &core.Header{Number: 4, Hash: randFelt(t)},
					Transactions: []core.Transaction{deployTxn},
					Receipts:     []*core.TransactionReceipt{commonReceipt},
				},
				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedDeploy}},
			},
			{
				// block with multiple txs receipts
				b: &core.Block{
					Header:       &core.Header{Number: 5, Hash: randFelt(t)},
					Transactions: []core.Transaction{invokeTx, declareTx},
					Receipts:     []*core.TransactionReceipt{commonReceipt, commonReceipt},
				},
				expectedRs: &spec.Receipts{Items: []*spec.Receipt{expectedInvoke, expectedDeclare}},
			},
		}

		numOfBs := uint64(len(tests))
		for _, test := range tests {
			mockReader.EXPECT().BlockByNumber(test.b.Number).Return(test.b, nil)
		}

		res, cErr := client.RequestReceipts(testCtx, &spec.ReceiptsRequest{Iteration: &spec.Iteration{
			Start:     &spec.Iteration_BlockNumber{BlockNumber: tests[0].b.Number},
			Direction: spec.Iteration_Forward,
			Limit:     numOfBs,
			Step:      1,
		}})
		require.NoError(t, cErr)

		var count uint64
		for receipts, valid := res(); valid; receipts, valid = res() {
			if count == numOfBs {
				assert.NotNil(t, receipts.GetFin())
				continue
			}

			assert.Equal(t, count, receipts.Id.Number)

			expectedRs := tests[count].expectedRs
			assert.True(t, proto.Equal(expectedRs, receipts.GetReceipts()))
			count++
		}
		require.Equal(t, numOfBs, count)
	})

	t.Run("get txns", func(t *testing.T) {
		blocks := []*core.Block{
			{
				Header: &core.Header{
					Number: 0,
				},
				Transactions: []core.Transaction{
					fillFelts(t, &core.DeployTransaction{
						ConstructorCallData: feltSlice(3),
					}),
					fillFelts(t, &core.L1HandlerTransaction{
						CallData: feltSlice(2),
						Version:  txVersion(1),
					}),
				},
			},
			{
				Header: &core.Header{
					Number: 1,
				},
				Transactions: []core.Transaction{
					fillFelts(t, &core.DeployAccountTransaction{
						DeployTransaction: core.DeployTransaction{
							ConstructorCallData: feltSlice(3),
							Version:             txVersion(1),
						},
						TransactionSignature: feltSlice(2),
					}),
				},
			},
			{
				Header: &core.Header{
					Number: 2,
				},
				Transactions: []core.Transaction{
					fillFelts(t, &core.DeclareTransaction{
						TransactionSignature: feltSlice(2),
						Version:              txVersion(0),
					}),
					fillFelts(t, &core.DeclareTransaction{
						TransactionSignature: feltSlice(2),
						Version:              txVersion(1),
					}),
				},
			},
			{
				Header: &core.Header{
					Number: 3,
				},
				Transactions: []core.Transaction{
					fillFelts(t, &core.InvokeTransaction{
						CallData:             feltSlice(3),
						TransactionSignature: feltSlice(2),
						Version:              txVersion(0),
					}),
					fillFelts(t, &core.InvokeTransaction{
						CallData:             feltSlice(4),
						TransactionSignature: feltSlice(2),
						Version:              txVersion(1),
					}),
				},
			},
		}
		numOfBlocks := uint64(len(blocks))

		for _, block := range blocks {
			mockReader.EXPECT().BlockByNumber(block.Number).Return(block, nil)
		}

		res, cErr := client.RequestTransactions(testCtx, &spec.TransactionsRequest{
			Iteration: &spec.Iteration{
				Start: &spec.Iteration_BlockNumber{
					BlockNumber: blocks[0].Number,
				},
				Direction: spec.Iteration_Forward,
				Limit:     numOfBlocks,
				Step:      1,
			},
		})
		require.NoError(t, cErr)

		var count uint64
		for txn, valid := res(); valid; txn, valid = res() {
			if count == numOfBlocks {
				assert.NotNil(t, txn.GetFin())
				break
			}

			assert.Equal(t, count, txn.Id.Number)

			expectedTx := mapToExpectedTransactions(blocks[count])
			assert.True(t, proto.Equal(expectedTx, txn.GetTransactions()))
			count++
		}
		require.Equal(t, numOfBlocks, count)
	})

	t.Run("get events", func(t *testing.T) {
		eventsPerBlock := [][]*core.Event{
			{}, // block with no events
			{
				{
					From: randFelt(t),
					Data: feltSlice(1),
					Keys: feltSlice(1),
				},
			},
			{
				{
					From: randFelt(t),
					Data: feltSlice(2),
					Keys: feltSlice(2),
				},
				{
					From: randFelt(t),
					Data: feltSlice(3),
					Keys: feltSlice(3),
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

func mapToExpectedTransactions(block *core.Block) *spec.Transactions {
	return &spec.Transactions{
		Items: utils.Map(block.Transactions, mapToExpectedTransaction),
	}
}

func mapToExpectedTransaction(tx core.Transaction) *spec.Transaction {
	adaptHash := core2p2p.AdaptHash
	adaptFelt := core2p2p.AdaptFelt
	adaptAddress := core2p2p.AdaptAddress

	var resp spec.Transaction
	switch v := tx.(type) {
	case *core.DeployTransaction:
		resp.Txn = &spec.Transaction_Deploy_{
			Deploy: &spec.Transaction_Deploy{
				ClassHash:   adaptHash(v.ClassHash),
				AddressSalt: adaptFelt(v.ContractAddressSalt),
				Calldata:    utils.Map(v.ConstructorCallData, adaptFelt),
			},
		}
	case *core.L1HandlerTransaction:
		resp.Txn = &spec.Transaction_L1Handler{
			L1Handler: &spec.Transaction_L1HandlerV1{
				Nonce:              adaptFelt(v.Nonce),
				Address:            adaptAddress(v.ContractAddress),
				EntryPointSelector: adaptFelt(v.EntryPointSelector),
				Calldata:           utils.Map(v.CallData, adaptFelt),
			},
		}
	case *core.DeployAccountTransaction:
		resp.Txn = &spec.Transaction_DeployAccountV1_{
			DeployAccountV1: &spec.Transaction_DeployAccountV1{
				MaxFee:      adaptFelt(v.MaxFee),
				Signature:   core2p2p.AdaptAccountSignature(v.TransactionSignature),
				ClassHash:   adaptHash(v.ClassHash),
				Nonce:       adaptFelt(v.Nonce),
				AddressSalt: adaptFelt(v.ContractAddressSalt),
				Calldata:    utils.Map(v.ConstructorCallData, adaptFelt),
			},
		}
	case *core.DeclareTransaction:
		if v.Version.Is(0) {
			resp.Txn = &spec.Transaction_DeclareV0_{
				DeclareV0: &spec.Transaction_DeclareV0{
					Sender:    adaptAddress(v.SenderAddress),
					MaxFee:    adaptFelt(v.MaxFee),
					Signature: core2p2p.AdaptAccountSignature(v.Signature()),
					ClassHash: adaptHash(v.ClassHash),
				},
			}
		} else if v.Version.Is(1) {
			resp.Txn = &spec.Transaction_DeclareV1_{
				DeclareV1: &spec.Transaction_DeclareV1{
					Sender:    adaptAddress(v.SenderAddress),
					MaxFee:    adaptFelt(v.MaxFee),
					Signature: core2p2p.AdaptAccountSignature(v.Signature()),
					ClassHash: adaptHash(v.ClassHash),
					Nonce:     adaptFelt(v.Nonce),
				},
			}
		}
	case *core.InvokeTransaction:
		if v.Version.Is(0) {
			resp.Txn = &spec.Transaction_InvokeV0_{
				InvokeV0: &spec.Transaction_InvokeV0{
					MaxFee:             adaptFelt(v.MaxFee),
					Signature:          core2p2p.AdaptAccountSignature(v.Signature()),
					Address:            adaptAddress(v.ContractAddress),
					EntryPointSelector: adaptFelt(v.EntryPointSelector),
					Calldata:           utils.Map(v.CallData, adaptFelt),
				},
			}
		} else if v.Version.Is(1) {
			resp.Txn = &spec.Transaction_InvokeV1_{
				InvokeV1: &spec.Transaction_InvokeV1{
					Sender:    adaptAddress(v.SenderAddress),
					MaxFee:    adaptFelt(v.MaxFee),
					Signature: core2p2p.AdaptAccountSignature(v.Signature()),
					Calldata:  utils.Map(v.CallData, adaptFelt),
				},
			}
		}
	}
	return &resp
}

func txVersion(v uint64) *core.TransactionVersion {
	var f felt.Felt
	f.SetUint64(v)

	txV := core.TransactionVersion(f)
	return &txV
}

func feltSlice(n int) []*felt.Felt {
	return make([]*felt.Felt, n)
}

func randFelt(t *testing.T) *felt.Felt {
	t.Helper()

	f, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	return f
}

func fillFelts[T any](t *testing.T, i T) T {
	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	typ := v.Type()

	const feltTypeStr = "*felt.Felt"

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		ftyp := typ.Field(i).Type // Get the type of the current field

		// Skip unexported fields
		if !f.CanSet() {
			continue
		}

		switch f.Kind() {
		case reflect.Ptr:
			// Check if the type is Felt
			if ftyp.String() == feltTypeStr {
				f.Set(reflect.ValueOf(randFelt(t)))
			} else if f.IsNil() {
				// Initialise the pointer if it's nil
				f.Set(reflect.New(ftyp.Elem()))
			}

			if f.Elem().Kind() == reflect.Struct {
				// Recursive call for nested structs
				fillFelts(t, f.Interface())
			}
		case reflect.Slice:
			// For slices, loop and populate
			for j := 0; j < f.Len(); j++ {
				elem := f.Index(j)
				if elem.Type().String() == feltTypeStr {
					elem.Set(reflect.ValueOf(randFelt(t)))
				}
			}
		case reflect.Struct:
			// Recursive call for nested structs
			fillFelts(t, f.Addr().Interface())
		}
	}

	return i
}
