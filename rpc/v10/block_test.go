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
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
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
	blockID       *rpcv9.BlockID
	l1Head        *core.L1Head
	blockStatus   rpcv9.BlockStatus
	responseFlags rpcv10.ResponseFlags
}

func createBlockTestCases(
	block *core.Block,
	commitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
) []blockTestCase {
	blockIDLatest := rpcv9.BlockIDLatest()
	blockIDHash := rpcv9.BlockIDFromHash(block.Hash)
	blockIDNumber := rpcv9.BlockIDFromNumber(block.Number)
	blockIDL1Accepted := rpcv9.BlockIDL1Accepted()
	blockIDPreConfirmed := rpcv9.BlockIDPreConfirmed()
	return []blockTestCase{
		{
			description: "blockID - latest",
			block:       block,
			commitments: commitments,
			stateUpdate: stateUpdate,
			blockID:     &blockIDLatest,
			l1Head:      nil,
			blockStatus: rpcv9.BlockAcceptedL2,
		},
		{
			description: "blockID - hash",
			block:       block,
			commitments: commitments,
			stateUpdate: stateUpdate,
			blockID:     &blockIDHash,
			l1Head:      nil,
			blockStatus: rpcv9.BlockAcceptedL2,
		},
		{
			description: "blockID - number",
			block:       block,
			commitments: commitments,
			stateUpdate: stateUpdate,
			blockID:     &blockIDNumber,
			l1Head:      nil,
			blockStatus: rpcv9.BlockAcceptedL2,
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
			blockStatus: rpcv9.BlockAcceptedL1,
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
			blockStatus: rpcv9.BlockAcceptedL1,
		},
		{
			description: "blockID - pre_confirmed",
			block:       block,
			commitments: nil,
			stateUpdate: nil,
			blockID:     &blockIDPreConfirmed,
			l1Head:      nil,
			blockStatus: rpcv9.BlockPreConfirmed,
		},
	}
}

func createBlockResponseFlagsTestCases(
	block *core.Block,
	commitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
) []blockTestCase {
	blockIDNumber := rpcv9.BlockIDFromNumber(block.Number)
	return []blockTestCase{
		{
			description:   "response flags - without IncludeProofFacts",
			block:         block,
			commitments:   commitments,
			stateUpdate:   stateUpdate,
			blockID:       &blockIDNumber,
			l1Head:        nil,
			blockStatus:   rpcv9.BlockAcceptedL2,
			responseFlags: rpcv10.ResponseFlags{},
		},
		{
			description:   "response flags - with IncludeProofFacts",
			block:         block,
			commitments:   commitments,
			stateUpdate:   stateUpdate,
			blockID:       &blockIDNumber,
			l1Head:        nil,
			blockStatus:   rpcv9.BlockAcceptedL2,
			responseFlags: rpcv10.ResponseFlags{IncludeProofFacts: true},
		},
	}
}

// assertCommittedBlockHeader asserts that the RPC block header matches the core block header
func assertCommittedBlockHeader(
	t *testing.T,
	expectedBlock *core.Block,
	expectedCommitments *core.BlockCommitments,
	stateDiffLen uint64,
	actual *rpcv10.BlockHeader,
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
	var expectedl1DAMode rpcv6.L1DAMode
	switch expectedBlock.L1DAMode {
	case core.Blob:
		expectedl1DAMode = rpcv6.Blob
	case core.Calldata:
		expectedl1DAMode = rpcv6.Calldata
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
	actual *rpcv10.BlockHeader,
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
	var expectedl1DAMode rpcv6.L1DAMode
	switch expectedBlock.L1DAMode {
	case core.Blob:
		expectedl1DAMode = rpcv6.Blob
	case core.Calldata:
		expectedl1DAMode = rpcv6.Calldata
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
	expectedStatus rpcv9.BlockStatus,
	expectedCommitments *core.BlockCommitments,
	expectedStateUpdate *core.StateUpdate,
	actual *rpcv10.BlockWithTxHashes,
) {
	t.Helper()
	assert.Equal(t, expectedStatus, actual.Status)
	if expectedStatus == rpcv9.BlockPreConfirmed {
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
	expectedStatus rpcv9.BlockStatus,
	actual *rpcv10.BlockWithTxs,
	responseFlags rpcv10.ResponseFlags,
) {
	t.Helper()
	assert.Equal(t, expectedStatus, actual.Status)
	if expectedStatus == rpcv9.BlockPreConfirmed {
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
	actualTransactions []*rpcv10.Transaction,
	includeProofFacts bool,
) {
	t.Helper()
	require.Equal(t, len(expectedTransactions), len(actualTransactions))
	for i, expectedTransaction := range expectedTransactions {
		require.Equal(t, expectedTransaction.Hash(), actualTransactions[i].Hash)
		adaptedTransaction := rpcv10.AdaptTransaction(expectedTransaction, includeProofFacts)
		require.Equal(t, adaptedTransaction.Transaction, actualTransactions[i].Transaction)
		require.Equal(t, adaptedTransaction.ProofFacts, actualTransactions[i].ProofFacts)
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
	expectedTxnFinalityStatus rpcv9.TxnFinalityStatus,
	actual []rpcv10.TransactionWithReceipt,
	responseFlags rpcv10.ResponseFlags,
) {
	t.Helper()
	require.Equal(t, len(expectedBlock.Receipts), len(actual))
	for i, expectedReceipt := range expectedBlock.Receipts {
		require.Equal(t, expectedReceipt.TransactionHash, actual[i].Receipt.Hash)
		adaptedTransaction := rpcv10.AdaptTransaction(
			expectedBlock.Transactions[i],
			responseFlags.IncludeProofFacts,
		)
		adaptedTransaction.Transaction.Hash = nil
		adaptedReceipt := rpcv9.AdaptReceipt(
			expectedReceipt,
			expectedBlock.Transactions[i],
			expectedTxnFinalityStatus,
		)

		require.Equal(t, adaptedTransaction.Transaction, actual[i].Transaction.Transaction)
		require.Equal(t, adaptedTransaction.ProofFacts, actual[i].Transaction.ProofFacts)
		require.Equal(t, adaptedReceipt, actual[i].Receipt)
	}
}

func blockStatusToTxnFinalityStatus(blockStatus rpcv9.BlockStatus) rpcv9.TxnFinalityStatus {
	switch blockStatus {
	case rpcv9.BlockAcceptedL1:
		return rpcv9.TxnAcceptedOnL1
	case rpcv9.BlockAcceptedL2:
		return rpcv9.TxnAcceptedOnL2
	}
	return rpcv9.TxnPreConfirmed
}

func assertBlockWithReceipts(
	t *testing.T,
	expectedBlock *core.Block,
	expectedStatus rpcv9.BlockStatus,
	expectedCommitments *core.BlockCommitments,
	expectedStateUpdate *core.StateUpdate,
	actual *rpcv10.BlockWithReceipts,
	responseFlags rpcv10.ResponseFlags,
) {
	t.Helper()
	assert.Equal(t, expectedStatus, actual.Status)
	if expectedStatus == rpcv9.BlockPreConfirmed {
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
	blockID *rpcv9.BlockID,
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
		blockAsPreConfirmed := rpcv10.CreateTestPreConfirmed(
			t,
			block,
			int(block.TransactionCount),
		)
		mockSyncReader.EXPECT().PendingData().Return(
			&blockAsPreConfirmed,
			nil,
		)
	case blockID.IsLatest():
		mockChain.EXPECT().Head().Return(block, nil)
	case blockID.IsHash():
		mockChain.EXPECT().BlockByHash(block.Hash).Return(block, nil)
	case blockID.IsL1Accepted():
		mockChain.EXPECT().BlockByNumber(block.Number).Return(block, nil)
	default:
		mockChain.EXPECT().BlockByNumber(block.Number).Return(block, nil)
	}
}

//nolint:dupl // Shares similar structure with other tests but tests different method
func TestBlockWithTxHashes_ErrorCases(t *testing.T) {
	errTests := map[string]rpcv9.BlockID{
		"latest":        rpcv9.BlockIDLatest(),
		"pre_confirmed": rpcv9.BlockIDPreConfirmed(),
		"hash":          rpcv9.BlockIDFromHash(&felt.One),
		"number":        rpcv9.BlockIDFromNumber(2),
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpcv10.New(chain, mockSyncReader, nil, log)

			if description == "pre_confirmed" {
				mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
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
		handler := rpcv10.New(mockReader, nil, nil, nil)

		blockID := rpcv9.BlockIDFromNumber(777)
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, err)

		resp, rpcErr := handler.BlockWithReceipts(&blockID, rpcv10.ResponseFlags{})
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})
}

func TestBlockWithTxHashes(t *testing.T) {
	network := &utils.Sepolia
	client := feeder.NewTestClient(t, network)

	block, commitments, stateUpdate := rpcv10.GetTestBlockWithCommitments(t, client, 56377)

	testCases := createBlockTestCases(block, commitments, stateUpdate)

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)
			mockReader := mocks.NewMockReader(mockCtrl)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

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

//nolint:dupl // Shares similar structure with other tests but tests different method
func TestBlockWithTxs_ErrorCases(t *testing.T) {
	errTests := map[string]rpcv9.BlockID{
		"latest":        rpcv9.BlockIDLatest(),
		"pre_confirmed": rpcv9.BlockIDPreConfirmed(),
		"hash":          rpcv9.BlockIDFromHash(&felt.One),
		"number":        rpcv9.BlockIDFromNumber(1),
	}

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

			handler := rpcv10.New(chain, mockSyncReader, nil, log)

			if description == "pre_confirmed" {
				mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
			}

			block, rpcErr := handler.BlockWithTxs(&id, rpcv10.ResponseFlags{})
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	//nolint:dupl // Similar l1head failure test structure across all error test functions
	t.Run("l1head failure", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpcv10.New(mockReader, nil, nil, nil)

		blockID := rpcv9.BlockIDFromNumber(777)
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, err)

		resp, rpcErr := handler.BlockWithReceipts(&blockID, rpcv10.ResponseFlags{})
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})
}

func TestBlockWithTxs(t *testing.T) {
	network := &utils.Mainnet
	client := feeder.NewTestClient(t, network)

	block, commitments, stateUpdate := rpcv10.GetTestBlockWithCommitments(t, client, 16697)

	testCases := createBlockTestCases(block, commitments, stateUpdate)

	clientSepolia := feeder.NewTestClient(t, &utils.Sepolia)
	blockWithProofFacts, commitmentsPF, stateUpdatePF := rpcv10.GetTestBlockWithCommitments(
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
			handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

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

//nolint:dupl // Shares similar structure with other tests but tests different method
func TestBlockWithReceipts_ErrorCases(t *testing.T) {
	errTests := map[string]rpcv9.BlockID{
		"latest":        rpcv9.BlockIDLatest(),
		"pre_confirmed": rpcv9.BlockIDPreConfirmed(),
		"hash":          rpcv9.BlockIDFromHash(&felt.One),
		"number":        rpcv9.BlockIDFromNumber(2),
	}

	for description, id := range errTests {
		t.Run(description, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			t.Cleanup(mockCtrl.Finish)

			log := utils.NewNopZapLogger()
			n := &utils.Mainnet
			chain := blockchain.New(memory.New(), n)
			mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
			handler := rpcv10.New(chain, mockSyncReader, nil, log)

			if description == "pre_confirmed" {
				mockSyncReader.EXPECT().PendingData().Return(nil, core.ErrPendingDataNotFound)
			}

			block, rpcErr := handler.BlockWithReceipts(&id, rpcv10.ResponseFlags{})
			assert.Nil(t, block)
			assert.Equal(t, rpccore.ErrBlockNotFound, rpcErr)
		})
	}

	//nolint:dupl // Similar l1head failure test structure across all error test functions
	t.Run("l1head failure", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockReader := mocks.NewMockReader(mockCtrl)
		handler := rpcv10.New(mockReader, nil, nil, nil)

		blockID := rpcv9.BlockIDFromNumber(777)
		block := &core.Block{
			Header: &core.Header{},
		}

		err := errors.New("l1 failure")
		mockReader.EXPECT().BlockByNumber(blockID.Number()).Return(block, nil)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, err)

		resp, rpcErr := handler.BlockWithReceipts(&blockID, rpcv10.ResponseFlags{})
		assert.Nil(t, resp)
		assert.Equal(t, rpccore.ErrInternal.CloneWithData(err.Error()), rpcErr)
	})
}

func TestBlockWithReceipts(t *testing.T) {
	network := &utils.Mainnet
	client := feeder.NewTestClient(t, network)

	block, commitments, stateUpdate := rpcv10.GetTestBlockWithCommitments(t, client, 16697)

	testCases := createBlockTestCases(block, commitments, stateUpdate)

	clientSepolia := feeder.NewTestClient(t, &utils.Sepolia)
	blockWithProofFacts, commitmentsPF, stateUpdatePF := rpcv10.GetTestBlockWithCommitments(
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
			handler := rpcv10.New(mockReader, mockSyncReader, nil, nil)

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
	handler := rpcv10.New(mockReader, nil, nil, nil)

	client := feeder.NewTestClient(t, n)
	latestBlockNumber := uint64(56378)

	t.Run("default sequencer address", func(t *testing.T) {
		block, commitments, stateUpdate := rpcv10.GetTestBlockWithCommitments(
			t,
			client,
			latestBlockNumber,
		)

		block.Header.SequencerAddress = nil
		mockReader.EXPECT().Head().Return(block, nil).Times(2)
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).Times(2)
		mockReader.EXPECT().BlockCommitmentsByNumber(block.Number).Return(commitments, nil).Times(2)
		mockReader.EXPECT().StateUpdateByNumber(block.Number).Return(stateUpdate, nil).Times(2)

		blockID := rpcv9.BlockIDLatest()
		actual, rpcErr := handler.BlockWithTxs(&blockID, rpcv10.ResponseFlags{})
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
	block, commitments, stateUpdate := rpcv10.GetTestBlockWithCommitments(t, client, blockNumber)

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	mockReader := mocks.NewMockReader(mockCtrl)

	mockReader.EXPECT().BlockByNumber(gomock.Any()).Return(block, nil)
	mockReader.EXPECT().L1Head().Return(core.L1Head{}, nil)
	mockReader.EXPECT().BlockCommitmentsByNumber(blockNumber).Return(commitments, nil)
	mockReader.EXPECT().StateUpdateByNumber(blockNumber).Return(stateUpdate, nil)

	handler := rpcv10.New(mockReader, nil, nil, nil)

	tx, ok := block.Transactions[0].(*core.InvokeTransaction)
	require.True(t, ok)

	blockID := rpcv9.BlockIDFromNumber(blockNumber)
	got, rpcErr := handler.BlockWithTxs(&blockID, rpcv10.ResponseFlags{})
	require.Nil(t, rpcErr)
	got.Transactions = got.Transactions[:1]

	stateDiffLength := stateUpdate.StateDiff.Length()
	require.Equal(t, &rpcv10.BlockWithTxs{
		BlockHeader: rpcv10.BlockHeader{
			Hash:            block.Hash,
			StarknetVersion: block.ProtocolVersion,
			NewRoot:         block.GlobalStateRoot,
			Number:          &block.Number,
			ParentHash:      block.ParentHash,
			L1DAMode:        utils.HeapPtr(rpcv6.Blob),
			L1GasPrice: &rpcv6.ResourcePrice{
				InFri: felt.NewUnsafeFromString[felt.Felt]("0x17882b6aa74"),
				InWei: felt.NewUnsafeFromString[felt.Felt]("0x3b9aca10"),
			},
			L1DataGasPrice: &rpcv6.ResourcePrice{
				InFri: felt.NewUnsafeFromString[felt.Felt]("0x2cc6d7f596e1"),
				InWei: felt.NewUnsafeFromString[felt.Felt]("0x716a8f6dd"),
			},
			SequencerAddress: block.SequencerAddress,
			Timestamp:        block.Timestamp,
			L2GasPrice: &rpcv6.ResourcePrice{
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
		Status: rpcv9.BlockAcceptedL2,
		Transactions: []*rpcv10.Transaction{
			{
				Transaction: rpcv9.Transaction{
					Hash:               tx.Hash(),
					Type:               rpcv9.TxnInvoke,
					Version:            tx.Version.AsFelt(),
					Nonce:              tx.Nonce,
					MaxFee:             tx.MaxFee,
					ContractAddress:    tx.ContractAddress,
					SenderAddress:      tx.SenderAddress,
					Signature:          &tx.TransactionSignature,
					CallData:           &tx.CallData,
					EntryPointSelector: tx.EntryPointSelector,
					ResourceBounds: &rpcv9.ResourceBoundsMap{
						L1Gas: &rpcv9.ResourceBounds{
							MaxAmount: felt.NewFromUint64[felt.Felt](
								tx.ResourceBounds[core.ResourceL1Gas].MaxAmount,
							),
							MaxPricePerUnit: tx.ResourceBounds[core.ResourceL1Gas].MaxPricePerUnit,
						},
						L2Gas: &rpcv9.ResourceBounds{
							MaxAmount: felt.NewFromUint64[felt.Felt](
								tx.ResourceBounds[core.ResourceL2Gas].MaxAmount,
							),
							MaxPricePerUnit: tx.ResourceBounds[core.ResourceL2Gas].MaxPricePerUnit,
						},
						L1DataGas: &rpcv9.ResourceBounds{
							MaxAmount:       &felt.Zero,
							MaxPricePerUnit: &felt.Zero,
						},
					},
					Tip:                   felt.NewFromUint64[felt.Felt](tx.Tip),
					PaymasterData:         &tx.PaymasterData,
					AccountDeploymentData: &tx.AccountDeploymentData,
					NonceDAMode:           utils.HeapPtr(rpcv9.DataAvailabilityMode(tx.NonceDAMode)),
					FeeDAMode:             utils.HeapPtr(rpcv9.DataAvailabilityMode(tx.FeeDAMode)),
				},
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
			if invokeTx.Version.Is(3) {
				invokeV3Count++
				if invokeTx.ProofFacts != nil {
					invokeV3WithProofFactsCount++
				}
			}
		}
	}
	require.Greater(
		t,
		invokeV3WithProofFactsCount,
		0,
		"Block should contain at least one invoke v3 transaction with proof_facts",
	)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	blockID := rpcv9.BlockIDFromNumber(block.Header.Number)
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

	handler := rpcv10.New(mockReader, mockSyncReader, nil, utils.NewNopZapLogger())

	t.Run("WithResponseFlag", func(t *testing.T) {
		responseFlags := &rpcv10.ResponseFlags{IncludeProofFacts: true}
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
			invokeV3WithProofFactsCount,
			txsWithProofFactsCount,
			"Number of transactions with proof_facts should match",
		)
	})

	t.Run("WithoutResponseFlag", func(t *testing.T) {
		blockWithTxs, rpcErr := handler.BlockWithTxs(&blockID, nil)
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

	// Count invoke v3 transactions with proof_facts
	var invokeV3WithProofFactsCount int
	for _, tx := range block.Transactions {
		if invokeTx, ok := tx.(*core.InvokeTransaction); ok {
			if invokeTx.Version.Is(3) && invokeTx.ProofFacts != nil {
				invokeV3WithProofFactsCount++
			}
		}
	}
	require.Greater(
		t,
		invokeV3WithProofFactsCount, 0,
		"Block should contain at least one invoke v3 transaction with proof_facts",
	)

	// Count all transactions
	totalTxCount := len(block.Transactions)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)

	blockID := rpcv9.BlockIDFromNumber(block.Header.Number)
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

	handler := rpcv10.New(mockReader, mockSyncReader, nil, utils.NewNopZapLogger())

	t.Run("WithResponseFlag", func(t *testing.T) {
		responseFlags := &rpcv10.ResponseFlags{IncludeProofFacts: true}
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
			invokeV3WithProofFactsCount,
			txsWithProofFactsCount,
			"Number of transactions with proof_facts should match",
		)
	})

	t.Run("WithoutResponseFlag", func(t *testing.T) {
		t.Run("WithoutResponseFlag", func(t *testing.T) {
			blockWithReceipts, rpcErr := handler.BlockWithReceipts(&blockID, nil)
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
