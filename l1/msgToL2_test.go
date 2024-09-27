package l1_test

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/test-go/testify/mock"
)

// MockEthClient is a mock for the ethclient.Client
type MockEthClient struct {
	mock.Mock
}

// TransactionReceipt mocks the TransactionReceipt method of ethclient.Client
func (m *MockEthClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	args := m.Called(ctx, txHash)
	if receipt, ok := args.Get(0).(*types.Receipt); ok {
		return receipt, args.Error(1)
	}
	return nil, args.Error(1)
}

// NewMockEthClient creates and returns a new instance of MockEthClient
func NewMockEthClient() *MockEthClient {
	return &MockEthClient{}
}

// TestMessageToL2Logs tests the MessageToL2Logs function using a JSON receipt
// func TestMessageToL2Logs(t *testing.T) {
// 	ethClient := NewMockEthClient()
// 	// Create a new mock Ethereum client
// 	ctrl := gomock.NewController(t)
// 	mockSubscriber := mocks.NewMockSubscriber(ctrl)

// 	var receiptJSON = `{}`
// 	txHash := common.HexToHash("0xYourTestTransactionHash")
// 	logMsgToL2SigHash := common.HexToHash("0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b")

// 	var receipt types.Receipt
// 	require.NoError(t, json.Unmarshal([]byte(receiptJSON), &receipt))

// 	// Mock the TransactionReceipt method to return the unmarshaled receipt
// 	// ethClient. ("TransactionReceipt", mock.Anything, txHash).Return(receipt, nil)

// 	// Call the MessageToL2Logs method
// 	results, err := mockSubscriber.MessageToL2Logs(txHash, big.NewInt(0), big.NewInt(10))

// 	// Verify expectations
// 	assert.Nil(t, err)
// 	assert.NotNil(t, results)
// 	assert.Len(t, results, 1)

// 	// Check the result
// 	expectedMsgHash := [32]byte{} // Placeholder for the expected hash, adjust based on your logic
// 	assert.Equal(t, expectedMsgHash, results[0].MsgHash)
// 	assert.Equal(t, txHash, *results[0].L1TxnHash)

// 	// Ensure that the mock expectations were met
// 	mockClient.AssertExpectations(t)
// }
