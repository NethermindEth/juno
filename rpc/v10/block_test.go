package rpcv10_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type blockTestCase struct {
	description   string
	block         *core.Block
	commitments   *core.BlockCommitments
	stateUpdate   *core.StateUpdate
	blockID       *rpc.BlockID
	l1Head        *core.L1Head
	blockStatus   rpc.BlockStatus
	responseFlags rpc.ResponseFlags
}

func createBlockTestCases(
	block *core.Block,
	commitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
) []blockTestCase {
	blockIDLatest := rpc.BlockIDLatest()
	blockIDHash := rpc.BlockIDFromHash(block.Hash)
	blockIDNumber := rpc.BlockIDFromNumber(block.Number)
	blockIDL1Accepted := rpc.BlockIDL1Accepted()
	blockIDPreConfirmed := rpc.BlockIDPreConfirmed()
	return []blockTestCase{
		{
			description: "blockID - latest",
			block:       block,
			commitments: commitments,
			stateUpdate: stateUpdate,
			blockID:     &blockIDLatest,
			l1Head:      nil,
			blockStatus: rpc.BlockAcceptedL2,
		},
		{
			description: "blockID - hash",
			block:       block,
			commitments: commitments,
			stateUpdate: stateUpdate,
			blockID:     &blockIDHash,
			l1Head:      nil,
			blockStatus: rpc.BlockAcceptedL2,
		},
		{
			description: "blockID - number",
			block:       block,
			commitments: commitments,
			stateUpdate: stateUpdate,
			blockID:     &blockIDNumber,
			l1Head:      nil,
			blockStatus: rpc.BlockAcceptedL2,
		},
		{
			description: "blockID - number accepted on l1",
			block:       block,
			commitments: commitments,
			stateUpdate: stateUpdate,
			blockID:     &blockIDNumber,
			l1Head: &core.L1Head{
				BlockNumber: block.Number,
				BlockHash:   block.Hash,
				StateRoot:   block.GlobalStateRoot,
			},
			blockStatus: rpc.BlockAcceptedL1,
		},
		{
			description: "blockID - l1_accepted",
			block:       block,
			commitments: commitments,
			stateUpdate: stateUpdate,
			blockID:     &blockIDL1Accepted,
			l1Head: &core.L1Head{
				BlockNumber: block.Number,
				BlockHash:   block.Hash,
				StateRoot:   block.GlobalStateRoot,
			},
			blockStatus: rpc.BlockAcceptedL1,
		},
		{
			description: "blockID - pre_confirmed",
			block:       block,
			commitments: nil,
			stateUpdate: nil,
			blockID:     &blockIDPreConfirmed,
			l1Head:      nil,
			blockStatus: rpc.BlockPreConfirmed,
		},
	}
}

func createBlockResponseFlagsTestCases(
	block *core.Block,
	commitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
) []blockTestCase {
	blockIDNumber := rpc.BlockIDFromNumber(block.Number)
	return []blockTestCase{
		{
			description:   "response flags - without IncludeProofFacts",
			block:         block,
			commitments:   commitments,
			stateUpdate:   stateUpdate,
			blockID:       &blockIDNumber,
			l1Head:        nil,
			blockStatus:   rpc.BlockAcceptedL2,
			responseFlags: rpc.ResponseFlags{},
		},
		{
			description:   "response flags - with IncludeProofFacts",
			block:         block,
			commitments:   commitments,
			stateUpdate:   stateUpdate,
			blockID:       &blockIDNumber,
			l1Head:        nil,
			blockStatus:   rpc.BlockAcceptedL2,
			responseFlags: rpc.ResponseFlags{IncludeProofFacts: true},
		},
	}
}

// assertCommittedBlockHeader asserts that the RPC block header matches the core block header
func assertCommittedBlockHeader(
	t *testing.T,
	expectedBlock *core.Block,
	expectedCommitments *core.BlockCommitments,
	stateDiffLen uint64,
	actual *rpc.BlockHeader,
) {
	t.Helper()

	assert.Equal(t, expectedBlock.Hash, actual.Hash)
	assert.Equal(t, expectedBlock.ParentHash, actual.ParentHash)
	assert.Equal(t, expectedBlock.Number, *actual.Number)
	assert.Equal(t, expectedBlock.GlobalStateRoot, actual.NewRoot)
	assert.Equal(t, expectedBlock.Timestamp, actual.Timestamp)
	assert.Equal(t, expectedBlock.SequencerAddress, actual.SequencerAddress)
	assert.Equal(t, nilToOne(expectedBlock.L1GasPriceETH), actual.L1GasPrice.InWei)
	assert.Equal(t, nilToOne(expectedBlock.L1GasPriceSTRK), actual.L1GasPrice.InFri)
	assert.Equal(t, nilToOne(expectedBlock.L1DataGasPrice.PriceInWei), actual.L1DataGasPrice.InWei)
	assert.Equal(t, nilToOne(expectedBlock.L1DataGasPrice.PriceInFri), actual.L1DataGasPrice.InFri)
	assert.Equal(t, nilToOne(expectedBlock.L2GasPrice.PriceInWei), actual.L2GasPrice.InWei)
	assert.Equal(t, nilToOne(expectedBlock.L2GasPrice.PriceInFri), actual.L2GasPrice.InFri)
	var expectedl1DAMode rpc.L1DAMode
	switch expectedBlock.L1DAMode {
	case core.Blob:
		expectedl1DAMode = rpc.Blob
	case core.Calldata:
		expectedl1DAMode = rpc.Calldata
	}
	assert.Equal(t, expectedl1DAMode, *actual.L1DAMode)
	assert.Equal(t, expectedBlock.ProtocolVersion, actual.StarknetVersion)
	assert.Equal(
		t,
		(*felt.Hash)(nilToZero(expectedCommitments.TransactionCommitment)),
		actual.TransactionCommitment,
	)
	assert.Equal(
		t,
		(*felt.Hash)(nilToZero(expectedCommitments.EventCommitment)),
		actual.EventCommitment,
	)
	assert.Equal(
		t,
		(*felt.Hash)(nilToZero(expectedCommitments.ReceiptCommitment)),
		actual.ReceiptCommitment,
	)
	assert.Equal(
		t,
		(*felt.Hash)(nilToZero(expectedCommitments.StateDiffCommitment)),
		actual.StateDiffCommitment,
	)
	assert.Equal(t, stateDiffLen, *actual.StateDiffLength)
	assert.Equal(t, expectedBlock.TransactionCount, *actual.TransactionCount)
	assert.Equal(t, expectedBlock.EventCount, *actual.EventCount)
}

// assertPreConfirmedBlockHeader asserts that the RPC block header matches
// the core block header for pre-confirmed blocks
func assertPreConfirmedBlockHeader(
	t *testing.T,
	expectedBlock *core.Block,
	actual *rpc.BlockHeader,
) {
	t.Helper()

	assert.Equal(t, expectedBlock.Number, *actual.Number)
	assert.Equal(t, expectedBlock.Timestamp, actual.Timestamp)
	assert.Equal(t, expectedBlock.SequencerAddress, actual.SequencerAddress)
	assert.Equal(t, nilToOne(expectedBlock.L1GasPriceETH), actual.L1GasPrice.InWei)
	assert.Equal(t, nilToOne(expectedBlock.L1GasPriceSTRK), actual.L1GasPrice.InFri)
	assert.Equal(t, nilToOne(expectedBlock.L1DataGasPrice.PriceInWei), actual.L1DataGasPrice.InWei)
	assert.Equal(t, nilToOne(expectedBlock.L1DataGasPrice.PriceInFri), actual.L1DataGasPrice.InFri)
	assert.Equal(t, nilToOne(expectedBlock.L2GasPrice.PriceInWei), actual.L2GasPrice.InWei)
	assert.Equal(t, nilToOne(expectedBlock.L2GasPrice.PriceInFri), actual.L2GasPrice.InFri)
	assert.Equal(t, expectedBlock.ProtocolVersion, actual.StarknetVersion)
	var expectedl1DAMode rpc.L1DAMode
	switch expectedBlock.L1DAMode {
	case core.Blob:
		expectedl1DAMode = rpc.Blob
	case core.Calldata:
		expectedl1DAMode = rpc.Calldata
	}
	assert.Equal(t, expectedl1DAMode, *actual.L1DAMode)

	assert.Nil(t, actual.Hash)
	assert.Nil(t, actual.ParentHash)
	assert.Nil(t, actual.NewRoot)

	assert.Nil(t, actual.TransactionCommitment)
	assert.Nil(t, actual.EventCommitment)
	assert.Nil(t, actual.ReceiptCommitment)
	assert.Nil(t, actual.StateDiffCommitment)
	assert.Nil(t, actual.StateDiffLength)
	assert.Nil(t, actual.TransactionCount)
	assert.Nil(t, actual.EventCount)
}

// assertBlockWithTxHashes asserts that BlockWithTxHashes matches the expected core block
func assertBlockWithTxHashes(
	t *testing.T,
	expectedBlock *core.Block,
	expectedStatus rpc.BlockStatus,
	expectedCommitments *core.BlockCommitments,
	expectedStateUpdate *core.StateUpdate,
	actual *rpc.BlockWithTxHashes,
) {
	t.Helper()
	assert.Equal(t, expectedStatus, actual.Status)
	if expectedStatus == rpc.BlockPreConfirmed {
		assertPreConfirmedBlockHeader(t, expectedBlock, &actual.BlockHeader)
	} else {
		assertCommittedBlockHeader(
			t,
			expectedBlock,
			expectedCommitments,
			expectedStateUpdate.StateDiff.Length(),
			&actual.BlockHeader,
		)
	}

	assertTransactionHashesEq(t, expectedBlock.Transactions, actual.TxnHashes)
}

func assertBlockWithTxs(
	t *testing.T,
	expectedBlock *core.Block,
	expectedCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
	expectedStatus rpc.BlockStatus,
	actual *rpc.BlockWithTxs,
	responseFlags rpc.ResponseFlags,
) {
	t.Helper()
	assert.Equal(t, expectedStatus, actual.Status)
	if expectedStatus == rpc.BlockPreConfirmed {
		assertPreConfirmedBlockHeader(t, expectedBlock, &actual.BlockHeader)
	} else {
		assertCommittedBlockHeader(
			t,
			expectedBlock,
			expectedCommitments,
			stateUpdate.StateDiff.Length(),
			&actual.BlockHeader,
		)
	}
	includeProofFacts := responseFlags.IncludeProofFacts
	assertTransactionsEq(t, expectedBlock.Transactions, actual.Transactions, includeProofFacts)
}

func assertTransactionsEq(
	t *testing.T,
	expectedTransactions []core.Transaction,
	actualTransactions []*rpc.Transaction,
	includeProofFacts bool,
) {
	t.Helper()
	require.Equal(t, len(expectedTransactions), len(actualTransactions))
	for i, expectedTransaction := range expectedTransactions {
		require.Equal(t, expectedTransaction.Hash(), actualTransactions[i].Hash)
		adaptedTransaction := rpc.AdaptTransaction(expectedTransaction, includeProofFacts)
		require.Equal(t, adaptedTransaction, *actualTransactions[i])
	}
}

func assertTransactionHashesEq(
	t *testing.T,
	expectedTransactions []core.Transaction,
	actualTransactionHashes []*felt.Felt,
) {
	t.Helper()
	require.Equal(t, len(expectedTransactions), len(actualTransactionHashes))
	for i, expectedTransaction := range expectedTransactions {
		require.Equal(t, expectedTransaction.Hash(), actualTransactionHashes[i])
	}
}

func assertTransactionsWithReceiptsEq(
	t *testing.T,
	expectedBlock *core.Block,
	expectedTxnFinalityStatus rpc.TxnFinalityStatus,
	actual []rpc.TransactionWithReceipt,
	responseFlags rpc.ResponseFlags,
) {
	t.Helper()
	require.Equal(t, len(expectedBlock.Receipts), len(actual))
	for i, expectedReceipt := range expectedBlock.Receipts {
		require.Equal(t, expectedReceipt.TransactionHash, actual[i].Receipt.Hash)
		adaptedTransaction := rpc.AdaptTransaction(
			expectedBlock.Transactions[i],
			responseFlags.IncludeProofFacts,
		)
		adaptedTransaction.Hash = nil
		adaptedReceipt := rpc.AdaptReceipt(
			expectedReceipt,
			expectedBlock.Transactions[i],
			expectedTxnFinalityStatus,
		)

		actualTx := *actual[i].Transaction
		require.Equal(t, adaptedTransaction, actualTx)
		require.Equal(t, adaptedReceipt, actual[i].Receipt)
	}
}

func blockStatusToTxnFinalityStatus(blockStatus rpc.BlockStatus) rpc.TxnFinalityStatus {
	switch blockStatus {
	case rpc.BlockAcceptedL1:
		return rpc.TxnAcceptedOnL1
	case rpc.BlockAcceptedL2:
		return rpc.TxnAcceptedOnL2
	}
	return rpc.TxnPreConfirmed
}

func assertBlockWithReceipts(
	t *testing.T,
	expectedBlock *core.Block,
	expectedStatus rpc.BlockStatus,
	expectedCommitments *core.BlockCommitments,
	expectedStateUpdate *core.StateUpdate,
	actual *rpc.BlockWithReceipts,
	responseFlags rpc.ResponseFlags,
) {
	t.Helper()
	assert.Equal(t, expectedStatus, actual.Status)
	if expectedStatus == rpc.BlockPreConfirmed {
		assertPreConfirmedBlockHeader(t, expectedBlock, &actual.BlockHeader)
	} else {
		assertCommittedBlockHeader(
			t,
			expectedBlock,
			expectedCommitments,
			expectedStateUpdate.StateDiff.Length(),
			&actual.BlockHeader,
		)
	}
	assertTransactionsWithReceiptsEq(
		t,
		expectedBlock,
		blockStatusToTxnFinalityStatus(expectedStatus),
		actual.Transactions,
		responseFlags,
	)
}

func setupMockBlockTest(
	t *testing.T,
	mockChain *mocks.MockReader,
	mockSyncReader *mocks.MockSyncReader,
	block *core.Block,
	commitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
	blockID *rpc.BlockID,
	l1Head *core.L1Head,
) {
	// mock L1 head
	if l1Head != nil {
		mockChain.EXPECT().L1Head().Return(*l1Head, nil).AnyTimes()
	} else {
		mockChain.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound)
	}

	// if pre_confirmed do not mock commitments
	if !blockID.IsPreConfirmed() {
		mockChain.EXPECT().BlockCommitmentsByNumber(block.Number).Return(commitments, nil)
		mockChain.EXPECT().StateUpdateByNumber(block.Number).Return(stateUpdate, nil)
	}

	switch {
	case blockID.IsPreConfirmed():
		blockAsPreConfirmed := rpc.CreateTestPreConfirmed(
			t,
			block,
			int(block.TransactionCount),
		)
		mockSyncReader.EXPECT().PreConfirmed().Return(
			&blockAsPreConfirmed,
			nil,
		).AnyTimes()
	case blockID.IsLatest():
		mockChain.EXPECT().Head().Return(block, nil).AnyTimes()
		mockChain.EXPECT().HeadsHeader().Return(block.Header, nil).AnyTimes()
		mockChain.EXPECT().TransactionsByBlockNumber(block.Number).Return(
			block.Transactions, nil).AnyTimes()
	case blockID.IsHash():
		mockChain.EXPECT().BlockByHash(block.Hash).Return(block, nil).AnyTimes()
		mockChain.EXPECT().BlockHeaderByHash(block.Hash).Return(block.Header, nil).AnyTimes()
		mockChain.EXPECT().TransactionsByBlockNumber(block.Number).Return(
			block.Transactions, nil).AnyTimes()
	case blockID.IsL1Accepted():
		mockChain.EXPECT().BlockByNumber(block.Number).Return(block, nil).AnyTimes()
		mockChain.EXPECT().BlockHeaderByNumber(block.Number).Return(block.Header, nil).AnyTimes()
		mockChain.EXPECT().TransactionsByBlockNumber(block.Number).Return(
			block.Transactions, nil).AnyTimes()
	default:
		mockChain.EXPECT().BlockByNumber(block.Number).Return(block, nil).AnyTimes()
		mockChain.EXPECT().BlockHeaderByNumber(block.Number).Return(block.Header, nil).AnyTimes()
		mockChain.EXPECT().TransactionsByBlockNumber(block.Number).Return(
			block.Transactions, nil).AnyTimes()
	}
}

func TestBlockNumber(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, nil)

	t.Run("empty blockchain", func(t *testing.T) {
		expectedHeight := uint64(0)
		mockReader.EXPECT().Height().Return(expectedHeight, errors.New("empty blockchain"))

		num, err := handler.BlockNumber()
		assert.Equal(t, expectedHeight, num)
		assert.Equal(t, rpccore.ErrNoBlock, err)
	})

	t.Run("blockchain height is 21", func(t *testing.T) {
		expectedHeight := uint64(21)
		mockReader.EXPECT().Height().Return(expectedHeight, nil)

		num, err := handler.BlockNumber()
		require.Nil(t, err)
		assert.Equal(t, expectedHeight, num)
	})
}

func TestBlockHashAndNumber(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, nil)

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().Head().Return(nil, errors.New("empty blockchain"))

		block, err := handler.BlockHashAndNumber()
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrNoBlock, err)
	})

	t.Run("blockchain height is 147", func(t *testing.T) {
		client := feeder.NewTestClient(t, n)
		gw := adaptfeeder.New(client)

		expectedBlock, err := gw.BlockByNumber(t.Context(), 147)
		require.NoError(t, err)

		expectedBlockHashAndNumber := &rpc.BlockHashAndNumber{
			Hash:   expectedBlock.Hash,
			Number: expectedBlock.Number,
		}

		mockReader.EXPECT().Head().Return(expectedBlock, nil)

		hashAndNum, rpcErr := handler.BlockHashAndNumber()
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedBlockHashAndNumber, hashAndNum)
	})
}

func TestBlockTransactionCount(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	n := utils.HeapPtr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	log := utils.NewNopZapLogger()
	handler := rpc.New(mockReader, mockSyncReader, nil, log)

	client := feeder.NewTestClient(t, n)
	gw := adaptfeeder.New(client)

	latestBlockNumber := uint64(56377)
	latestBlock, err := gw.BlockByNumber(t.Context(), latestBlockNumber)

	require.NoError(t, err)
	latestBlockHash := latestBlock.Hash
	expectedCount := latestBlock.TransactionCount

	t.Run("empty blockchain", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)
		latest := rpc.BlockIDLatest()
		count, rpcErr := handler.BlockTransactionCount(&latest)
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent block hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(gomock.Any()).Return(nil, db.ErrKeyNotFound)
		hash := rpc.BlockIDFromHash(felt.NewFromBytes[felt.Felt]([]byte("random")))
		count, rpcErr := handler.BlockTransactionCount(&hash)
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("non-existent pre_confirmed block", func(t *testing.T) {
		mockSyncReader.EXPECT().PreConfirmed().Return(nil, db.ErrKeyNotFound)
		preConfirmed := rpc.BlockIDPreConfirmed()
		count, rpcErr := handler.BlockTransactionCount(&preConfirmed)
		require.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		assert.Equal(t, uint64(0), count)
	})

	t.Run("non-existent block number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).Return(nil, db.ErrKeyNotFound)
		number := rpc.BlockIDFromNumber(uint64(328476))
		count, rpcErr := handler.BlockTransactionCount(&number)
		assert.Equal(t, uint64(0), count)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("blockID - latest", func(t *testing.T) {
		mockReader.EXPECT().HeadsHeader().Return(latestBlock.Header, nil)
		latest := rpc.BlockIDLatest()
		count, rpcErr := handler.BlockTransactionCount(&latest)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - hash", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByHash(latestBlockHash).Return(latestBlock.Header, nil)
		hash := rpc.BlockIDFromHash(latestBlockHash)
		count, rpcErr := handler.BlockTransactionCount(&hash)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - number", func(t *testing.T) {
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)
		number := rpc.BlockIDFromNumber(latestBlockNumber)
		count, rpcErr := handler.BlockTransactionCount(&number)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - l1_accepted", func(t *testing.T) {
		mockReader.EXPECT().L1Head().Return(
			core.L1Head{
				BlockNumber: latestBlock.Number,
				BlockHash:   latestBlock.Hash,
				StateRoot:   latestBlock.GlobalStateRoot,
			},
			nil,
		)
		mockReader.EXPECT().BlockHeaderByNumber(latestBlockNumber).Return(latestBlock.Header, nil)
		l1AcceptedID := rpc.BlockIDL1Accepted()
		count, rpcErr := handler.BlockTransactionCount(&l1AcceptedID)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})

	t.Run("blockID - pre_confirmed", func(t *testing.T) {
		latestBlock.Hash = nil
		latestBlock.GlobalStateRoot = nil
		preConfirmed := core.NewPreConfirmed(latestBlock, nil, nil, nil)
		mockSyncReader.EXPECT().PreConfirmed().Return(
			&preConfirmed,
			nil,
		)

		preConfirmedID := rpc.BlockIDPreConfirmed()
		count, rpcErr := handler.BlockTransactionCount(&preConfirmedID)
		require.Nil(t, rpcErr)
		assert.Equal(t, expectedCount, count)
	})
}

//nolint:dupl // Shares similar structure with other tests but tests different method
func TestBlockWithTxHashes_ErrorCases(t *testing.T) {
	errTests := map[string]rpc.BlockID{
		"latest":        rpc.BlockIDLatest(),
		"pre_confirmed": rpc.BlockIDPreConfirmed(),
		"hash":          rpc.BlockIDFromHash(&felt.One),
		"number":        rpc.BlockIDFromNumber(2),
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpc.New(chain, mockSyncReader, nil, log)

			if description == "pre_confirmed" {
				mockSyncReader.EXPECT().PreConfirmed().Return(nil, db.ErrKeyNotFound)
			}

			block, rpcErr := handler.BlockWithTxHashes(&id)
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	t.Run("l1head failure", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, nil)

		blockID := rpc.BlockIDFromNumber(777)
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, err)

		resp, rpcErr := handler.BlockWithReceipts(&blockID, rpc.ResponseFlags{})
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})
}

func TestBlockWithTxHashes(t *testing.T) {
	network := &utils.Sepolia
	client := feeder.NewTestClient(t, network)

	block, commitments, stateUpdate := rpc.GetTestBlockWithCommitments(t, client, 56377)

	testCases := createBlockTestCases(block, commitments, stateUpdate)

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)
			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpc.New(mockReader, mockSyncReader, nil, nil)

			setupMockBlockTest(
				t,
				mockReader,
				mockSyncReader,
				tc.block,
				tc.commitments,
				tc.stateUpdate,
				tc.blockID,
				tc.l1Head,
			)

			block, rpcErr := handler.BlockWithTxHashes(tc.blockID)
			require.Nil(t, rpcErr)
			assertBlockWithTxHashes(
				t,
				tc.block,
				tc.blockStatus,
				tc.commitments,
				tc.stateUpdate,
				block,
			)
		})
	}
}

func TestBlockWithTxHashes_TxnsFetchError(t *testing.T) {
	blockNumber := uint64(123)
	header := &core.Header{Number: blockNumber}

	t.Run("TransactionsByBlockNumber returns ErrKeyNotFound", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, nil)

		id := rpc.BlockIDFromNumber(blockNumber)
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxHashes(&id)
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("TransactionsByBlockNumber returns internal error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, nil)

		id := rpc.BlockIDFromNumber(blockNumber)
		internalErr := errors.New("some internal error")
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(nil, internalErr)

		block, rpcErr := handler.BlockWithTxHashes(&id)
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(internalErr), rpcErr)
	})
}

//nolint:dupl // Shares similar structure with other tests but tests different method
func TestBlockWithTxs_ErrorCases(t *testing.T) {
	errTests := map[string]rpc.BlockID{
		"latest":        rpc.BlockIDLatest(),
		"pre_confirmed": rpc.BlockIDPreConfirmed(),
		"hash":          rpc.BlockIDFromHash(&felt.One),
		"number":        rpc.BlockIDFromNumber(1),
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

			handler := rpc.New(chain, mockSyncReader, nil, log)

			if description == "pre_confirmed" {
				mockSyncReader.EXPECT().PreConfirmed().Return(nil, db.ErrKeyNotFound)
			}

			block, rpcErr := handler.BlockWithTxs(&id, rpc.ResponseFlags{})
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	//nolint:dupl // Similar l1head failure test structure across all error test functions
	t.Run("l1head failure", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, nil)

		blockID := rpc.BlockIDFromNumber(777)
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, err)

		resp, rpcErr := handler.BlockWithReceipts(&blockID, rpc.ResponseFlags{})
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})
}

func TestBlockWithTxs(t *testing.T) {
	network := &utils.Mainnet
	client := feeder.NewTestClient(t, network)

	block, commitments, stateUpdate := rpc.GetTestBlockWithCommitments(t, client, 16697)

	testCases := createBlockTestCases(block, commitments, stateUpdate)

	clientSepolia := feeder.NewTestClient(t, &utils.Sepolia)
	blockWithProofFacts, commitmentsPF, stateUpdatePF := rpc.GetTestBlockWithCommitments(
		t,
		clientSepolia,
		4072139,
	)
	testCases = append(
		testCases,
		createBlockResponseFlagsTestCases(
			blockWithProofFacts,
			commitmentsPF,
			stateUpdatePF,
		)...,
	)

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)
			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpc.New(mockReader, mockSyncReader, nil, nil)

			setupMockBlockTest(
				t,
				mockReader,
				mockSyncReader,
				tc.block,
				tc.commitments,
				tc.stateUpdate,
				tc.blockID,
				tc.l1Head,
			)

			block, rpcErr := handler.BlockWithTxs(tc.blockID, tc.responseFlags)
			require.Nil(t, rpcErr)
			assertBlockWithTxs(
				t,
				tc.block,
				tc.commitments,
				tc.stateUpdate,
				tc.blockStatus,
				block,
				tc.responseFlags,
			)
		})
	}
}

func TestBlockWithTxs_TxnsFetchError(t *testing.T) {
	blockNumber := uint64(123)
	header := &core.Header{Number: blockNumber}

	t.Run("TransactionsByBlockNumber returns ErrKeyNotFound", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, nil)

		id := rpc.BlockIDFromNumber(blockNumber)
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(nil, db.ErrKeyNotFound)

		block, rpcErr := handler.BlockWithTxs(&id, rpc.ResponseFlags{})
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
	})

	t.Run("TransactionsByBlockNumber returns internal error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)
		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, nil)

		id := rpc.BlockIDFromNumber(blockNumber)
		internalErr := errors.New("some internal error")
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(nil, internalErr)

		block, rpcErr := handler.BlockWithTxs(&id, rpc.ResponseFlags{})
		assert.Nil(t, block)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(internalErr), rpcErr)
	})
}

//nolint:dupl // Shares similar structure with other tests but tests different method
func TestBlockWithReceipts_ErrorCases(t *testing.T) {
	errTests := map[string]rpc.BlockID{
		"latest":        rpc.BlockIDLatest(),
		"pre_confirmed": rpc.BlockIDPreConfirmed(),
		"hash":          rpc.BlockIDFromHash(&felt.One),
		"number":        rpc.BlockIDFromNumber(2),
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpc.New(chain, mockSyncReader, nil, log)

			if description == "pre_confirmed" {
				mockSyncReader.EXPECT().PreConfirmed().Return(nil, db.ErrKeyNotFound)
			}

			block, rpcErr := handler.BlockWithReceipts(&id, rpc.ResponseFlags{})
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	//nolint:dupl // Similar l1head failure test structure across all error test functions
	t.Run("l1head failure", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpc.New(mockReader, nil, nil, nil)

		blockID := rpc.BlockIDFromNumber(777)
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, err)

		resp, rpcErr := handler.BlockWithReceipts(&blockID, rpc.ResponseFlags{})
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})
}

func TestBlockWithReceipts(t *testing.T) {
	network := &utils.Mainnet
	client := feeder.NewTestClient(t, network)

	block, commitments, stateUpdate := rpc.GetTestBlockWithCommitments(t, client, 16697)

	testCases := createBlockTestCases(block, commitments, stateUpdate)

	clientSepolia := feeder.NewTestClient(t, &utils.Sepolia)
	blockWithProofFacts, commitmentsPF, stateUpdatePF := rpc.GetTestBlockWithCommitments(
		t,
		clientSepolia,
		4072139,
	)
	testCases = append(
		testCases,
		createBlockResponseFlagsTestCases(
			blockWithProofFacts,
			commitmentsPF,
			stateUpdatePF,
		)...,
	)

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)
			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpc.New(mockReader, mockSyncReader, nil, nil)

			setupMockBlockTest(
				t,
				mockReader,
				mockSyncReader,
				tc.block,
				tc.commitments,
				tc.stateUpdate,
				tc.blockID,
				tc.l1Head,
			)

			blockWithReceipts, rpcErr := handler.BlockWithReceipts(tc.blockID, tc.responseFlags)
			require.Nil(t, rpcErr)
			assertBlockWithReceipts(
				t,
				tc.block,
				tc.blockStatus,
				tc.commitments,
				tc.stateUpdate,
				blockWithReceipts,
				tc.responseFlags,
			)
		})
	}
}

func TestRpcBlockAdaptation(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := utils.HeapPtr(utils.Sepolia)
	mockReader := mocks.NewMockReader(mockCtrl)
	handler := rpc.New(mockReader, nil, nil, nil)

	client := feeder.NewTestClient(t, n)
	latestBlockNumber := uint64(56378)

	t.Run("default sequencer address", func(t *testing.T) {
		block, commitments, stateUpdate := rpc.GetTestBlockWithCommitments(
			t,
			client,
			latestBlockNumber,
		)

		block.Header.SequencerAddress = nil
		mockReader.EXPECT().HeadsHeader().Return(block.Header, nil).Times(2)
		mockReader.EXPECT().TransactionsByBlockNumber(block.Number).Return(
			block.Transactions, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).Times(2)
		mockReader.EXPECT().BlockCommitmentsByNumber(block.Number).Return(commitments, nil).Times(2)
		mockReader.EXPECT().StateUpdateByNumber(block.Number).Return(stateUpdate, nil).Times(2)

		blockID := rpc.BlockIDLatest()
		actual, rpcErr := handler.BlockWithTxs(&blockID, rpc.ResponseFlags{})
		require.Nil(t, rpcErr)
		require.Equal(t, &felt.Zero, actual.BlockHeader.SequencerAddress)

		blockWithTxHashes, rpcErr := handler.BlockWithTxHashes(&blockID)
		require.Nil(t, rpcErr)
		require.Equal(t, &felt.Zero, blockWithTxHashes.BlockHeader.SequencerAddress)
	})
}

func TestBlockWithTxHashesV013(t *testing.T) {
	network := utils.HeapPtr(utils.SepoliaIntegration)
	blockNumber := uint64(16350)
	client := feeder.NewTestClient(t, network)
	block, commitments, stateUpdate := rpc.GetTestBlockWithCommitments(t, client, blockNumber)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)

	mockReader.EXPECT().BlockHeaderByNumber(blockNumber).Return(block.Header, nil)
	mockReader.EXPECT().TransactionsByBlockNumber(blockNumber).Return(block.Transactions, nil)
	mockReader.EXPECT().L1Head().Return(core.L1Head{}, nil)
	mockReader.EXPECT().BlockCommitmentsByNumber(blockNumber).Return(commitments, nil)
	mockReader.EXPECT().StateUpdateByNumber(blockNumber).Return(stateUpdate, nil)

	handler := rpc.New(mockReader, nil, nil, nil)

	tx, ok := block.Transactions[0].(*core.InvokeTransaction)
	require.True(t, ok)

	blockID := rpc.BlockIDFromNumber(blockNumber)
	got, rpcErr := handler.BlockWithTxs(&blockID, rpc.ResponseFlags{})
	require.Nil(t, rpcErr)
	got.Transactions = got.Transactions[:1]

	stateDiffLength := stateUpdate.StateDiff.Length()
	require.Equal(t, &rpc.BlockWithTxs{
		BlockHeader: rpc.BlockHeader{
			Hash:            block.Hash,
			StarknetVersion: block.ProtocolVersion,
			NewRoot:         block.GlobalStateRoot,
			Number:          &block.Number,
			ParentHash:      block.ParentHash,
			L1DAMode:        utils.HeapPtr(rpc.Blob),
			L1GasPrice: &rpc.ResourcePrice{
				InFri: felt.NewUnsafeFromString[felt.Felt]("0x17882b6aa74"),
				InWei: felt.NewUnsafeFromString[felt.Felt]("0x3b9aca10"),
			},
			L1DataGasPrice: &rpc.ResourcePrice{
				InFri: felt.NewUnsafeFromString[felt.Felt]("0x2cc6d7f596e1"),
				InWei: felt.NewUnsafeFromString[felt.Felt]("0x716a8f6dd"),
			},
			SequencerAddress: block.SequencerAddress,
			Timestamp:        block.Timestamp,
			L2GasPrice: &rpc.ResourcePrice{
				InFri: &felt.One,
				InWei: &felt.One,
			},
			TransactionCommitment: (*felt.Hash)(nilToZero(commitments.TransactionCommitment)),
			EventCommitment:       (*felt.Hash)(nilToZero(commitments.EventCommitment)),
			ReceiptCommitment:     (*felt.Hash)(nilToZero(commitments.ReceiptCommitment)),
			StateDiffCommitment:   (*felt.Hash)(nilToZero(commitments.StateDiffCommitment)),
			StateDiffLength:       &stateDiffLength,
			TransactionCount:      &block.TransactionCount,
			EventCount:            &block.EventCount,
		},
		Status: rpc.BlockAcceptedL2,
		Transactions: []*rpc.Transaction{
			{
				Hash:               tx.Hash(),
				Type:               rpc.TxnInvoke,
				Version:            tx.Version.AsFelt(),
				Nonce:              tx.Nonce,
				MaxFee:             tx.MaxFee,
				ContractAddress:    tx.ContractAddress,
				SenderAddress:      tx.SenderAddress,
				Signature:          &tx.TransactionSignature,
				CallData:           &tx.CallData,
				EntryPointSelector: tx.EntryPointSelector,
				ResourceBounds: &rpc.ResourceBoundsMap{
					L1Gas: &rpc.ResourceBounds{
						MaxAmount: felt.NewFromUint64[felt.Felt](
							tx.ResourceBounds[core.ResourceL1Gas].MaxAmount,
						),
						MaxPricePerUnit: tx.ResourceBounds[core.ResourceL1Gas].MaxPricePerUnit,
					},
					L2Gas: &rpc.ResourceBounds{
						MaxAmount: felt.NewFromUint64[felt.Felt](
							tx.ResourceBounds[core.ResourceL2Gas].MaxAmount,
						),
						MaxPricePerUnit: tx.ResourceBounds[core.ResourceL2Gas].MaxPricePerUnit,
					},
					L1DataGas: &rpc.ResourceBounds{
						MaxAmount:       &felt.Zero,
						MaxPricePerUnit: &felt.Zero,
					},
				},
				Tip:                   felt.NewFromUint64[felt.Felt](tx.Tip),
				PaymasterData:         &tx.PaymasterData,
				AccountDeploymentData: &tx.AccountDeploymentData,
				NonceDAMode:           utils.HeapPtr(rpc.DataAvailabilityMode(tx.NonceDAMode)),
				FeeDAMode:             utils.HeapPtr(rpc.DataAvailabilityMode(tx.FeeDAMode)),
			},
		},
	}, got)
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}

func nilToOne(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.One
	}
	return f
}

func TestBlockWithTxsWithResponseFlags(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	network := &utils.Sepolia
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)
	require.NotNil(t, block)
	require.Greater(t, len(block.Transactions), 0)

	// Count invoke v3 transactions with proof_facts and total invoke v3 transactions
	var invokeV3WithProofFactsCount int
	var invokeV3Count int
	for _, tx := range block.Transactions {
		if invokeTx, ok := tx.(*core.InvokeTransaction); ok {
			invokeV3Count++
			if invokeTx.ProofFacts != nil {
				invokeV3WithProofFactsCount++
			}
		}
	}
	require.Greater(
		t,
		invokeV3Count,
		0,
		"Block should contain at least one invoke v3 transaction",
	)
	require.Equal(
		t,
		invokeV3Count,
		invokeV3WithProofFactsCount,
		"All invoke v3 transactions should have proof_facts set when flag is included",
	)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	blockID := rpc.BlockIDFromNumber(block.Header.Number)
	mockReader.EXPECT().BlockHeaderByNumber(block.Header.Number).Return(block.Header, nil).AnyTimes()
	mockReader.EXPECT().TransactionsByBlockNumber(
		block.Header.Number,
	).Return(block.Transactions, nil).AnyTimes()
	mockReader.EXPECT().Network().Return(network).AnyTimes()
	mockReader.EXPECT().L1Head().Return(core.L1Head{}, nil).AnyTimes()

	mockReader.EXPECT().BlockCommitmentsByNumber(block.Header.Number).Return(&core.BlockCommitments{
		TransactionCommitment: &felt.Zero,
		EventCommitment:       &felt.Zero,
		ReceiptCommitment:     &felt.Zero,
		StateDiffCommitment:   &felt.Zero,
	}, nil).AnyTimes()
	mockReader.EXPECT().StateUpdateByNumber(block.Header.Number).Return(&core.StateUpdate{
		StateDiff: &core.StateDiff{},
	}, nil).AnyTimes()

	handler := rpc.New(mockReader, mockSyncReader, nil, utils.NewNopZapLogger())

	t.Run("WithResponseFlag", func(t *testing.T) {
		responseFlags := rpc.ResponseFlags{IncludeProofFacts: true}
		blockWithTxs, rpcErr := handler.BlockWithTxs(&blockID, responseFlags)
		require.Nil(t, rpcErr)
		require.NotNil(t, blockWithTxs)

		// Verify total number of transactions is the same
		require.Equal(t, len(block.Transactions), len(blockWithTxs.Transactions))

		// Count transactions with proof_facts in response
		var txsWithProofFactsCount int
		for _, tx := range blockWithTxs.Transactions {
			if tx.ProofFacts != nil {
				txsWithProofFactsCount++
			}
		}

		// Verify number of transactions with proof_facts matches expected
		require.Equal(
			t,
			invokeV3Count,
			txsWithProofFactsCount,
			"All invoke v3 transactions should have proof_facts set when flag is included",
		)
	})

	t.Run("WithoutResponseFlag", func(t *testing.T) {
		blockWithTxs, rpcErr := handler.BlockWithTxs(&blockID, rpc.ResponseFlags{})
		require.Nil(t, rpcErr)
		require.NotNil(t, blockWithTxs)

		// Verify total number of transactions is the same
		require.Equal(t, len(block.Transactions), len(blockWithTxs.Transactions))

		// Verify no transactions have proof_facts when flag is not set
		for _, tx := range blockWithTxs.Transactions {
			require.Nil(t, tx.ProofFacts, "proof_facts should not be included when flag is not set")
		}
	})
}

func TestBlockWithReceiptsWithResponseFlags(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	network := &utils.Sepolia
	client := feeder.NewTestClient(t, network)
	gw := adaptfeeder.New(client)

	block, err := gw.BlockByNumber(t.Context(), 4072139)
	require.NoError(t, err)
	require.NotNil(t, block)
	require.Greater(t, len(block.Transactions), 0)
	require.Equal(
		t,
		len(block.Transactions),
		len(block.Receipts),
		"Block should have receipts for all transactions",
	)

	// Count invoke v3 transactions
	var invokeV3Count int
	for _, tx := range block.Transactions {
		if _, ok := tx.(*core.InvokeTransaction); ok {
			invokeV3Count++
		}
	}
	require.Greater(
		t,
		invokeV3Count,
		0,
		"Block should contain at least one invoke v3 transaction")

	// Count all transactions
	totalTxCount := len(block.Transactions)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	blockID := rpc.BlockIDFromNumber(block.Header.Number)
	mockReader.EXPECT().BlockByNumber(block.Header.Number).Return(block, nil).AnyTimes()
	mockReader.EXPECT().Network().Return(network).AnyTimes()
	mockReader.EXPECT().L1Head().Return(core.L1Head{}, nil).AnyTimes()

	mockReader.EXPECT().BlockCommitmentsByNumber(block.Header.Number).Return(&core.BlockCommitments{
		TransactionCommitment: &felt.Zero,
		EventCommitment:       &felt.Zero,
		ReceiptCommitment:     &felt.Zero,
		StateDiffCommitment:   &felt.Zero,
	}, nil).AnyTimes()
	mockReader.EXPECT().StateUpdateByNumber(block.Header.Number).Return(&core.StateUpdate{
		StateDiff: &core.StateDiff{},
	}, nil).AnyTimes()

	handler := rpc.New(mockReader, mockSyncReader, nil, utils.NewNopZapLogger())

	t.Run("WithResponseFlag", func(t *testing.T) {
		responseFlags := rpc.ResponseFlags{IncludeProofFacts: true}
		blockWithReceipts, rpcErr := handler.BlockWithReceipts(&blockID, responseFlags)
		require.Nil(t, rpcErr)
		require.NotNil(t, blockWithReceipts)

		// Verify total number of transactions is the same
		require.Equal(t, totalTxCount, len(blockWithReceipts.Transactions))

		// Count transactions with proof_facts in response
		var txsWithProofFactsCount int
		for _, txWithReceipt := range blockWithReceipts.Transactions {
			if txWithReceipt.Transaction.ProofFacts != nil {
				txsWithProofFactsCount++
			}
		}

		// Verify number of transactions with proof_facts matches expected
		require.Equal(
			t,
			invokeV3Count,
			txsWithProofFactsCount,
			"All invoke v3 transactions should have proof_facts set when flag is included",
		)
	})

	t.Run("WithoutResponseFlag", func(t *testing.T) {
		t.Run("WithoutResponseFlag", func(t *testing.T) {
			blockWithReceipts, rpcErr := handler.BlockWithReceipts(&blockID, rpc.ResponseFlags{})
			require.Nil(t, rpcErr)
			require.NotNil(t, blockWithReceipts)

			// Verify total number of transactions is the same
			require.Equal(t, len(block.Transactions), len(blockWithReceipts.Transactions))

			// Verify no transactions have proof_facts when flag is not set
			for _, tx := range blockWithReceipts.Transactions {
				require.Nil(
					t,
					tx.Transaction.ProofFacts,
					"proof_facts should not be included when flag is not set",
				)
			}
		})
	})
}
