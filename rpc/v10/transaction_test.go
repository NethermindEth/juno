package rpcv10_test

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// loadBlockFromFeederTestdata loads a block from feeder testdata and adapts it to core.Block.
// This allows us to use testdata blocks without fetching from a real gateway.
func loadBlockFromFeederTestdata(
	t *testing.T,
	network *utils.Network,
	blockNumber uint64,
) *core.Block {
	t.Helper()
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)
	block, err := gw.BlockByNumber(t.Context(), blockNumber)
	require.NoError(t, err)
	return block
}

func TestTransactionReceiptByHash(t *testing.T) {
	type testCase struct {
		description   string
		network       *utils.Network
		expected      *rpcv9.TransactionReceipt
		pendingDataFn func(t *testing.T, block *core.Block) core.PendingData
		l1Head        core.L1Head
	}

	emptyPendingDataFunc := func(t *testing.T, block *core.Block) core.PendingData {
		return &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: block.Number + 1,
				},
			},
		}
	}

	preConfirmedPendingDataFunc := func(t *testing.T, block *core.Block) core.PendingData {
		return &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number,
					TransactionCount: block.TransactionCount,
					EventCount:       block.EventCount,
				},
				Transactions: block.Transactions,
				Receipts:     block.Receipts,
			},
		}
	}

	withPreLatestPendingDataFunc := func(t *testing.T, block *core.Block) core.PendingData {
		preLatest := core.PreLatest{
			Block: &core.Block{
				Header: &core.Header{
					Number:           block.Number,
					ParentHash:       block.ParentHash,
					TransactionCount: block.TransactionCount,
					EventCount:       block.EventCount,
				},
				Transactions: block.Transactions,
				Receipts:     block.Receipts,
			},
		}
		preConfirmed := &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Number: preLatest.Block.Number + 1,
				},
			},
			PreLatest: &preLatest,
		}

		return preConfirmed
	}

	testCases := []testCase{
		{
			description: "receipt accepted on l2",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_accepted_on_l2.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt accepted on l1",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_accepted_on_l1.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: math.MaxUint64},
		},
		{
			description: "receipt pre confirmed",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_pre_confirmed.json",
			),
			pendingDataFn: preConfirmedPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt pre latest",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_pre_latest.json",
			),
			pendingDataFn: withPreLatestPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt reverted",
			network:     &utils.Integration,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_reverted.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt invoke v3",
			network:     &utils.Integration,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_invoke_v3.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt non empty da",
			network:     &utils.SepoliaIntegration,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_non_empty_da.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
		{
			description: "receipt deploy",
			network:     &utils.Mainnet,
			expected: readTestData[*rpcv9.TransactionReceipt](
				t,
				"transactions/receipt_deploy.json",
			),
			pendingDataFn: emptyPendingDataFunc,
			l1Head:        core.L1Head{BlockNumber: 0},
		},
	}

	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			expected := test.expected
			require.NotNil(t, expected.BlockNumber)

			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

			handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

			loadedBlock := loadBlockFromFeederTestdata(t, test.network, *expected.BlockNumber)
			var transaction core.Transaction
			var transactionIndex int
			// find the transaction in the block
			for i, tx := range loadedBlock.Transactions {
				if tx.Hash().Equal(expected.Hash) {
					transaction = tx
					transactionIndex = i
					break
				}
			}
			require.NotNil(t, transaction, "transaction not found on expected block")

			pendingData := test.pendingDataFn(t, loadedBlock)
			mockSyncReader.EXPECT().PendingData().Return(pendingData, nil)
			_, _, _, err := pendingData.ReceiptByHash(transaction.Hash())
			if err != nil {
				// receipt belong to canonical block mock expectations
				mockReader.EXPECT().BlockNumberAndIndexByTxHash(
					(*felt.TransactionHash)(expected.Hash),
				).Return(*expected.BlockNumber, uint64(transactionIndex), nil)
				mockReader.EXPECT().TransactionByBlockNumberAndIndex(
					*expected.BlockNumber, uint64(transactionIndex),
				).Return(transaction, nil)
				mockReader.EXPECT().ReceiptByBlockNumberAndIndex(
					*expected.BlockNumber, uint64(transactionIndex),
				).Return(*loadedBlock.Receipts[transactionIndex], expected.BlockHash, nil)
				mockReader.EXPECT().L1Head().Return(test.l1Head, nil)
			}

			receipt, rpcErr := handler.TransactionReceiptByHash(expected.Hash)
			require.Nil(t, rpcErr)
			require.Equal(t, expected, receipt)
		})
	}
}

func TestTransactionReceiptByHash_NotFound(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

	txHash := felt.NewFromBytes[felt.Felt]([]byte("random hash"))
	mockReader.EXPECT().BlockNumberAndIndexByTxHash(
		(*felt.TransactionHash)(txHash),
	).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
	mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
	mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)

	tx, rpcErr := handler.TransactionReceiptByHash(txHash)
	assert.Nil(t, tx)
	assert.Equal(t, rpccore.ErrTxnHashNotFound, rpcErr)
}

func TestTransactionStatus(t *testing.T) {
	mainnetClient := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(mainnetClient)

	block, err := gw.BlockLatest(t.Context())
	require.NoError(t, err)
	tx := block.Transactions[0]
	targetTxnHash := tx.Hash()
	type testCase struct {
		description    string
		network        *utils.Network
		txHash         *felt.Felt
		expectedStatus rpcv9.TransactionStatus
		expectedErr    *jsonrpc.Error
		setupMocks     func(
			mockReader *mocks.MockReader,
			mockSyncReader *mocks.MockSyncReader,
		)
	}
	preConfirmedPlaceHolder := core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: block.Number + 1,
			},
		},
	}
	mockFoundInDB := func(
		mockReader *mocks.MockReader,
		mockSyncReader *mocks.MockSyncReader,
	) {
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(tx.Hash()),
		).Return(block.Number, uint64(0), nil)
		mockReader.EXPECT().TransactionByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(tx, nil)
		mockReader.EXPECT().ReceiptByBlockNumberAndIndex(
			block.Number, uint64(0),
		).Return(*block.Receipts[0], block.Hash, nil)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmedPlaceHolder, nil)
	}

	mockNotFound := func(
		mockReader *mocks.MockReader,
		mockSyncReader *mocks.MockSyncReader,
	) {
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(
			gomock.Any(),
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmedPlaceHolder, nil).Times(2)
	}

	// TODO(Ege): Add test with failure reason REVERTED
	testCases := []testCase{
		{
			description: "status ACCEPTED_ON_L2",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL2,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				mockFoundInDB(mockReader, mockSyncReader)
				mockReader.EXPECT().L1Head().Return(core.L1Head{BlockNumber: 0}, nil)
			},
		},
		{
			description: "status ACCEPTED_ON_L1",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL1,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				mockFoundInDB(mockReader, mockSyncReader)
				mockReader.EXPECT().L1Head().Return(core.L1Head{BlockNumber: math.MaxUint64}, nil)
			},
		},
		{
			description: "status PRE_CONFIRMED",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusPreConfirmed,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				mockSyncReader.EXPECT().PendingData().Return(
					&core.PreConfirmed{
						Block: &core.Block{
							Header: &core.Header{
								Number: block.Number,
							},
							Transactions: block.Transactions,
							Receipts:     block.Receipts,
						},
					}, nil,
				)
			},
		},
		{
			description: "status CANDIDATE",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusCandidate,
				Execution: rpcv9.UnknownExecution,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				mockReader.EXPECT().BlockNumberAndIndexByTxHash(
					(*felt.TransactionHash)(targetTxnHash),
				).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
				mockSyncReader.EXPECT().PendingData().Return(
					&core.PreConfirmed{
						Block: &core.Block{
							Header: &core.Header{
								Number: block.Number,
							},
						},
						CandidateTxs: []core.Transaction{tx},
					}, nil,
				).Times(2)
			},
		},
		{
			description: "status ACCEPTED_ON_L2 from pre-latest",
			network:     &utils.Mainnet,
			txHash:      targetTxnHash,
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL2,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: func(mockReader *mocks.MockReader, mockSyncReader *mocks.MockSyncReader) {
				preLatest := core.PreLatest{
					Block: &core.Block{
						Header: &core.Header{
							ParentHash: block.ParentHash,
							Number:     block.Number,
						},
						Transactions: block.Transactions,
						Receipts:     block.Receipts,
					},
				}

				mockSyncReader.EXPECT().PendingData().Return(
					preConfirmedPlaceHolder.Copy().WithPreLatest(&preLatest),
					nil,
				)
			},
		},
		{
			description: "not found localy - ACCEPTED_ON_L1 from feeder",
			network:     &utils.Mainnet,
			txHash: felt.NewUnsafeFromString[felt.Felt](
				"0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e",
			),
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL1,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: mockNotFound,
		},
		{
			description: "not found localy - ACCEPTED_ON_L2 from feeder",
			network:     &utils.Mainnet,
			txHash: felt.NewUnsafeFromString[felt.Felt](
				"0x6c40890743aa220b10e5ee68cef694c5c23cc2defd0dbdf5546e687f9982ab1",
			),
			expectedStatus: rpcv9.TransactionStatus{
				Finality:  rpcv9.TxnStatusAcceptedOnL2,
				Execution: rpcv9.TxnSuccess,
			},
			setupMocks: mockNotFound,
		},
		{
			description: "transaction not found",
			network:     &utils.Mainnet,
			txHash:      felt.NewUnsafeFromString[felt.Felt]("0xFF00FF00"),
			expectedErr: rpccore.ErrTxnHashNotFound,
			setupMocks:  mockNotFound,
		},
		{
			// RPCv10 does not have REJECTED status.
			// For historical queries return not found error instead.
			description: "REJECTED historical txn found in feeder",
			network:     &utils.SepoliaIntegration,
			txHash:      felt.NewUnsafeFromString[felt.Felt]("0x1111"),
			expectedErr: rpccore.ErrTxnHashNotFound,
			setupMocks:  mockNotFound,
		},
	}
	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			client := feeder.NewTestClient(t, test.network)
			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			test.setupMocks(mockReader, mockSyncReader)

			handler := rpcv10.New(mockReader, mockSyncReader, nil, nil).WithFeeder(client)

			status, rpcErr := handler.TransactionStatus(t.Context(), test.txHash)
			require.Equal(t, test.expectedErr, rpcErr)
			require.Equal(t, test.expectedStatus, status)
		})
	}
}

func TestSubmittedTransactionsCache(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	log := utils.NewNopZapLogger()
	network := utils.Integration

	client := feeder.NewTestClient(t, &network)

	cacheSize := uint(5)
	cacheEntryTimeOut := time.Second

	txnToAdd := createBaseInvokeTransactionV3()

	broadcastedTxn := &rpcv9.BroadcastedTransaction{
		Transaction: *rpcv9.AdaptTransaction(txnToAdd),
	}

	var gatewayResponse struct {
		TransactionHash *felt.Felt `json:"transaction_hash"`
	}

	gatewayResponse.TransactionHash = txnToAdd.TransactionHash
	rawGatewayResponse, err := json.Marshal(gatewayResponse)
	require.NoError(t, err)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockGateway := mocks.NewMockGateway(mockCtrl)
	mockGateway.
		EXPECT().
		AddTransaction(gomock.Any(), gomock.Any()).
		Return(rawGatewayResponse, nil).
		Times(2)

	preConfirmedPlaceHolder := core.PreConfirmed{
		Block: &core.Block{
			Header: &core.Header{
				Number: 1,
			},
		},
	}
	t.Run("transaction not found in db and feeder but found in cache", func(t *testing.T) {
		submittedTransactionCache := rpccore.NewTransactionCache(cacheEntryTimeOut, cacheSize)
		fakeClock := make(chan time.Time, 1)
		defer close(fakeClock)
		submittedTransactionCache.WithTicker(fakeClock)
		ctx := t.Context()
		go func() {
			err := submittedTransactionCache.Run(ctx)
			require.NoError(t, err)
		}()

		handler := rpcv10.New(mockReader, mockSyncReader, nil, log).
			WithFeeder(client).
			WithGateway(mockGateway).
			WithSubmittedTransactionsCache(submittedTransactionCache)

		rpcv9Handler := rpcv9.New(mockReader, mockSyncReader, nil, log).
			WithFeeder(client).
			WithGateway(mockGateway).
			WithSubmittedTransactionsCache(submittedTransactionCache)

		res, err := rpcv9Handler.AddTransaction(ctx, broadcastedTxn)
		require.Nil(t, err)
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(res.TransactionHash),
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmedPlaceHolder, nil).Times(2)

		status, err := handler.TransactionStatus(ctx, res.TransactionHash)
		require.Nil(t, err)
		require.Equal(t, rpcv9.TxnStatusReceived, status.Finality)
		require.Equal(t, rpcv9.UnknownExecution, status.Execution)
	})

	t.Run("transaction not found in db and feeder, found in cache but expired", func(t *testing.T) {
		submittedTransactionCache := rpccore.NewTransactionCache(cacheEntryTimeOut, cacheSize)
		fakeClock := make(chan time.Time, 1)
		defer close(fakeClock)
		submittedTransactionCache.WithTicker(fakeClock)
		ctx := t.Context()
		go func() {
			err := submittedTransactionCache.Run(ctx)
			require.NoError(t, err)
		}()

		handler := rpcv10.New(mockReader, mockSyncReader, nil, log).
			WithFeeder(client).
			WithGateway(mockGateway).
			WithSubmittedTransactionsCache(submittedTransactionCache)

		rpcv9Handler := rpcv9.New(mockReader, mockSyncReader, nil, log).
			WithFeeder(client).
			WithGateway(mockGateway).
			WithSubmittedTransactionsCache(submittedTransactionCache)

		res, err := rpcv9Handler.AddTransaction(ctx, broadcastedTxn)
		require.Nil(t, err)
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(
			(*felt.TransactionHash)(res.TransactionHash),
		).Return(uint64(0), uint64(0), db.ErrKeyNotFound)
		mockSyncReader.EXPECT().PendingData().Return(&preConfirmedPlaceHolder, nil).Times(2)

		// Expire cache entry
		for range rpccore.NumTimeBuckets {
			fakeClock <- time.Now()
		}
		status, err := handler.TransactionStatus(ctx, res.TransactionHash)
		require.Equal(t, rpccore.ErrTxnHashNotFound, err)
		require.Empty(t, status)
	})
}

func TestAddTransactionWithProofAndProofFacts(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	baseTxnToAdd := createBaseInvokeTransactionV3()

	handler := rpcv10.New(nil, nil, nil, utils.NewNopZapLogger())

	t.Run("WithProofAndProofFacts", func(t *testing.T) {
		proofFacts := []*felt.Felt{
			felt.NewFromUint64[felt.Felt](100),
			felt.NewFromUint64[felt.Felt](200),
		}
		txnToAdd := *baseTxnToAdd
		txnToAdd.ProofFacts = proofFacts

		broadcastedTxn := &rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: *rpcv9.AdaptTransaction(&txnToAdd),
			},
			Proof:      []uint64{1, 2, 3},
			ProofFacts: proofFacts,
		}

		mockGateway := mocks.NewMockGateway(mockCtrl)
		mockGateway.
			EXPECT().
			AddTransaction(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, txnJSON json.RawMessage) {
				var txStruct struct {
					Proof      []uint64     `json:"proof,omitempty"`
					ProofFacts []*felt.Felt `json:"proof_facts,omitempty"`
				}
				require.NoError(t, json.Unmarshal(txnJSON, &txStruct))
				require.NotNil(t, txStruct.Proof)
				require.Equal(t, []uint64{1, 2, 3}, txStruct.Proof)
				require.NotNil(t, txStruct.ProofFacts)
				require.Equal(t, 2, len(txStruct.ProofFacts))
				require.Equal(t, proofFacts[0], txStruct.ProofFacts[0])
				require.Equal(t, proofFacts[1], txStruct.ProofFacts[1])
			}).
			Return(json.RawMessage(`{
				"transaction_hash": "0x1",
				"address": "0x2",
				"class_hash": "0x3"
			}`), nil).
			Times(1)

		h := handler.WithGateway(mockGateway)
		got, rpcErr := h.AddTransaction(t.Context(), broadcastedTxn)
		require.Nil(t, rpcErr)
		require.Equal(t, rpcv10.AddTxResponse{
			TransactionHash: (*felt.TransactionHash)(felt.NewFromUint64[felt.Felt](0x1)),
			ContractAddress: (*felt.Address)(felt.NewFromUint64[felt.Felt](0x2)),
			ClassHash:       (*felt.ClassHash)(felt.NewFromUint64[felt.Felt](0x3)),
		}, got)
	})

	t.Run("WithoutProofAndProofFacts", func(t *testing.T) {
		broadcastedTxn := &rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: *rpcv9.AdaptTransaction(baseTxnToAdd),
			},
		}

		mockGateway := mocks.NewMockGateway(mockCtrl)
		mockGateway.
			EXPECT().
			AddTransaction(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, txnJSON json.RawMessage) {
				var txStruct struct {
					Proof      []uint64     `json:"proof,omitempty"`
					ProofFacts []*felt.Felt `json:"proof_facts,omitempty"`
				}
				require.NoError(t, json.Unmarshal(txnJSON, &txStruct))
				require.Nil(t, txStruct.Proof)
				require.Nil(t, txStruct.ProofFacts)
			}).
			Return(json.RawMessage(`{
				"transaction_hash": "0x1",
				"address": "0x2",
				"class_hash": "0x3"
			}`), nil).
			Times(1)

		h := handler.WithGateway(mockGateway)
		got, rpcErr := h.AddTransaction(t.Context(), broadcastedTxn)
		require.Nil(t, rpcErr)
		require.NotNil(t, got.TransactionHash)
	})
}

func TestTransactionByHashWithResponseFlags(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	network := &utils.Sepolia
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)

	txnHash := felt.NewUnsafeFromString[felt.Felt](
		"0x435f87f1eecd5968ba8190744fee1f3ef69f17471f8902ce1e7d444c4e0c8cb",
	)

	invokeTx, err := gw.Transaction(t.Context(), txnHash)
	require.NoError(t, err)
	require.IsType(t, &core.InvokeTransaction{}, invokeTx)

	invokeTxCore := invokeTx.(*core.InvokeTransaction)
	require.NotNil(t, invokeTxCore.ProofFacts)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound).AnyTimes()
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 0}, nil).AnyTimes()
	mockReader.EXPECT().TransactionByHash(txnHash).Return(invokeTxCore, nil).AnyTimes()
	mockReader.EXPECT().Network().Return(network).AnyTimes()

	handler := rpcv10.New(mockReader, mockSyncReader, nil, utils.NewNopZapLogger())

	t.Run("WithResponseFlag", func(t *testing.T) {
		responseFlags := rpcv10.ResponseFlags{IncludeProofFacts: true}
		tx, rpcErr := handler.TransactionByHash(txnHash, responseFlags)
		require.Nil(t, rpcErr)
		require.NotNil(t, tx)
		require.NotNil(t, tx.ProofFacts)
		require.Equal(t, len(invokeTxCore.ProofFacts), len(*tx.ProofFacts))
	})

	t.Run("WithoutResponseFlag", func(t *testing.T) {
		tx, rpcErr := handler.TransactionByHash(txnHash, rpcv10.ResponseFlags{})
		require.Nil(t, rpcErr)
		require.NotNil(t, tx)
		require.Nil(t, tx.ProofFacts)
	})
}

func TestTransactionByBlockIDAndIndexWithResponseFlags(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	network := &utils.Sepolia
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)

	txnHash := felt.NewUnsafeFromString[felt.Felt](
		"0x435f87f1eecd5968ba8190744fee1f3ef69f17471f8902ce1e7d444c4e0c8cb",
	)

	invokeTx, err := gw.Transaction(t.Context(), txnHash)
	require.NoError(t, err)
	require.IsType(t, &core.InvokeTransaction{}, invokeTx)

	invokeTxCore := invokeTx.(*core.InvokeTransaction)
	require.NotNil(t, invokeTxCore.ProofFacts)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	blockID := rpcv9.BlockIDFromNumber(1)
	mockReader.EXPECT().TransactionByBlockNumberAndIndex(
		uint64(1),
		uint64(0),
	).Return(invokeTxCore, nil).AnyTimes()
	mockReader.EXPECT().Network().Return(network).AnyTimes()

	handler := rpcv10.New(mockReader, mockSyncReader, nil, utils.NewNopZapLogger())

	t.Run("WithResponseFlag", func(t *testing.T) {
		responseFlags := &rpcv10.ResponseFlags{IncludeProofFacts: true}
		tx, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, 0, responseFlags)
		require.Nil(t, rpcErr)
		require.NotNil(t, tx)
		require.NotNil(t, tx.ProofFacts)
		require.Equal(t, len(invokeTxCore.ProofFacts), len(*tx.ProofFacts))
	})

	t.Run("WithoutResponseFlag", func(t *testing.T) {
		tx, rpcErr := handler.TransactionByBlockIDAndIndex(&blockID, 0, nil)
		require.Nil(t, rpcErr)
		require.NotNil(t, tx)
		require.Nil(t, tx.ProofFacts)
	})
}

func TestAdaptBroadcastedTransactionValidation(t *testing.T) {
	network := &utils.Sepolia

	t.Run("RejectProofFactsForNonV3Invoke", func(t *testing.T) {
		proofFact := felt.NewFromUint64[felt.Felt](100)
		broadcastedTxn := &rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: rpcv9.Transaction{
					Type:    rpcv9.TxnInvoke,
					Version: felt.NewFromUint64[felt.Felt](1),
					Signature: &[]*felt.Felt{
						felt.NewFromUint64[felt.Felt](0x1),
					},
					Nonce:         felt.NewFromUint64[felt.Felt](0x1),
					SenderAddress: felt.NewFromUint64[felt.Felt](0x1),
					CallData:      &[]*felt.Felt{},
				},
			},
			ProofFacts: []*felt.Felt{proofFact},
		}

		_, _, _, err := rpcv10.AdaptBroadcastedTransaction(broadcastedTxn, network)
		require.Error(t, err)
		require.Contains(t, err.Error(), "proof_facts can only be included in invoke v3 transactions")
	})

	t.Run("RejectProofForNonV3Invoke", func(t *testing.T) {
		broadcastedTxn := &rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: rpcv9.Transaction{
					Type:    rpcv9.TxnInvoke,
					Version: felt.NewFromUint64[felt.Felt](1),
					Signature: &[]*felt.Felt{
						felt.NewFromUint64[felt.Felt](0x1),
					},
					Nonce:         felt.NewFromUint64[felt.Felt](0x1),
					SenderAddress: felt.NewFromUint64[felt.Felt](0x1),
					CallData:      &[]*felt.Felt{},
				},
			},
			Proof: []uint64{1, 2, 3},
		}

		_, _, _, err := rpcv10.AdaptBroadcastedTransaction(broadcastedTxn, network)
		require.Error(t, err)
		require.Contains(t, err.Error(), "proof can only be included in invoke v3 transactions")
	})

	t.Run("RejectProofFactsForNonInvoke", func(t *testing.T) {
		proofFact := felt.NewFromUint64[felt.Felt](100)
		broadcastedTxn := &rpcv10.BroadcastedTransaction{
			BroadcastedTransaction: rpcv9.BroadcastedTransaction{
				Transaction: rpcv9.Transaction{
					Type:    rpcv9.TxnDeclare,
					Version: felt.NewFromUint64[felt.Felt](3),
					Signature: &[]*felt.Felt{
						felt.NewFromUint64[felt.Felt](0x1),
					},
					Nonce:         felt.NewFromUint64[felt.Felt](0x1),
					SenderAddress: felt.NewFromUint64[felt.Felt](0x1),
				},
			},
			ProofFacts: []*felt.Felt{proofFact},
		}

		_, _, _, err := rpcv10.AdaptBroadcastedTransaction(broadcastedTxn, network)
		require.Error(t, err)
		require.Contains(t, err.Error(), "proof_facts can only be included in invoke v3 transactions")
	})
}

func createBaseInvokeTransactionV3() *core.InvokeTransaction {
	return &core.InvokeTransaction{
		TransactionHash: felt.NewFromUint64[felt.Felt](12345),
		Version:         new(core.TransactionVersion).SetUint64(3),
		TransactionSignature: []*felt.Felt{
			felt.NewFromUint64[felt.Felt](0x1),
			felt.NewFromUint64[felt.Felt](0x1),
		},
		Nonce:       felt.NewFromUint64[felt.Felt](0x1),
		NonceDAMode: core.DAModeL1,
		FeeDAMode:   core.DAModeL1,
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas: {
				MaxAmount:       0x1,
				MaxPricePerUnit: felt.NewFromUint64[felt.Felt](0x1),
			},
			core.ResourceL1DataGas: {
				MaxAmount:       0x1,
				MaxPricePerUnit: felt.NewFromUint64[felt.Felt](0x1),
			},
			core.ResourceL2Gas: {
				MaxAmount:       0,
				MaxPricePerUnit: &felt.Zero,
			},
		},
		Tip:           0,
		PaymasterData: []*felt.Felt{},
		SenderAddress: felt.NewFromUint64[felt.Felt](0x1),
		CallData:      []*felt.Felt{},
	}
}
