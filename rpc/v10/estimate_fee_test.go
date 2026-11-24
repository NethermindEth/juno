package rpcv10_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpctest"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func estimateFeeParams(blockID, flags string, txns ...rpctest.Txn) string {
	request := make([]json.RawMessage, len(txns))
	for i := range txns {
		request[i] = json.RawMessage(txns[i].String())
	}
	data, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(`{"request":%s,"simulation_flags":%s,"block_id":%s}`, data, flags, blockID)
}

func TestEstimateFee(t *testing.T) {
	t.Parallel()
	recipient := felt.FromUint64[felt.Address](0x105)

	t.Run("the estimates equal the fees of the mined transactions", func(t *testing.T) {
		t.Parallel()
		constant := func(blockID string) func(*testing.T, *rpctest.Devnet) string {
			return func(*testing.T, *rpctest.Devnet) string { return blockID }
		}
		tests := []struct {
			name    string
			blockID func(*testing.T, *rpctest.Devnet) string
		}{
			{name: "latest", blockID: constant(`"latest"`)},
			{name: "pre_confirmed", blockID: constant(`"pre_confirmed"`)},
			{name: "block number", blockID: constant(`{"block_number":1}`)},
			{name: "block hash", blockID: func(t *testing.T, d *rpctest.Devnet) string {
				return fmt.Sprintf(`{"block_hash":%q}`, blockHash(t, d.Node, 1))
			}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				devnet := rpctest.NewDevnet(t)
				sender := devnet.Account(t, "0x101")
				txns := []rpctest.Txn{
					sender.InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000)),
					sender.InvokeV3(t, 1, rpctest.TransferSTRK(&recipient, 2000)),
				}

				response := devnet.Call(t, rpcVersion, "starknet_estimateFee",
					estimateFeeParams(test.blockID(t, devnet), `[]`, txns...))
				require.Empty(t, response.Error)
				steps, err := strconv.Atoi(response.Header.Get("X-Cairo-Steps"))
				require.NoError(t, err)
				assert.Positive(t, steps)

				var estimates []feeEstimate
				require.NoError(t, json.Unmarshal(response.Result, &estimates))
				require.Len(t, estimates, len(txns))

				devnet.Submit(t, txns...)
				devnet.MakeBlock(t)
				for i, txn := range txns {
					got := getReceipt(t, devnet.Node, &txn.Hash)
					assert.Equal(t, "SUCCEEDED", got.ExecutionStatus)
					assert.Equal(t, got.ActualFee.Amount, estimates[i].OverallFee)
					assert.Equal(t, "FRI", got.ActualFee.Unit)
					assert.Equal(t, "FRI", estimates[i].Unit)
					assert.Equal(t, hexToBig(t, estimates[i].OverallFee), estimates[i].gasCost(t))
				}
			})
		}
	})

	// The VM searches the L2 gas of a deploy account and adds 10 % to the gas that it finds.
	t.Run("the estimate of a deploy account is at most 10 % above its fee", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		account := devnet.NewAccount(t)
		funder := devnet.Account(t, "0x101")
		devnet.Submit(t, funder.InvokeV3(t, 0, rpctest.TransferSTRK(&account.Address, 1e15)))
		devnet.MakeBlock(t)

		deploy := account.DeployAccountV3(t)
		var estimates []feeEstimate
		result := devnet.Result(t, rpcVersion, "starknet_estimateFee",
			estimateFeeParams(`"latest"`, `[]`, deploy))
		require.NoError(t, json.Unmarshal(result, &estimates))

		devnet.Submit(t, deploy)
		devnet.MakeBlock(t)
		got := getReceipt(t, devnet.Node, &deploy.Hash)
		require.Equal(t, "SUCCEEDED", got.ExecutionStatus)
		actual := hexToBig(t, got.ActualFee.Amount)
		estimate := hexToBig(t, estimates[0].OverallFee)
		assert.GreaterOrEqual(t, estimate.Cmp(actual), 0)
		maxEstimate := new(big.Int).Div(new(big.Int).Mul(actual, big.NewInt(11)), big.NewInt(10))
		assert.LessOrEqual(t, estimate.Cmp(maxEstimate), 0)
	})

	t.Run("SKIP_VALIDATE estimates a transaction with a wrong signature", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		valid := devnet.Account(t, "0x101").InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000))
		invalid := valid.With("signature", []string{"0x1", "0x2"})

		response := devnet.Call(t, rpcVersion, "starknet_estimateFee",
			estimateFeeParams(`"latest"`, `[]`, invalid))
		requireExecutionError(t, response.Error, 0, "Account: invalid signature")
		assert.Equal(t, "0", response.Header.Get("X-Cairo-Steps"))

		var validated, unvalidated []feeEstimate
		result := devnet.Result(t, rpcVersion, "starknet_estimateFee",
			estimateFeeParams(`"latest"`, `[]`, valid))
		require.NoError(t, json.Unmarshal(result, &validated))
		result = devnet.Result(t, rpcVersion, "starknet_estimateFee",
			estimateFeeParams(`"latest"`, `["SKIP_VALIDATE"]`, invalid))
		require.NoError(t, json.Unmarshal(result, &unvalidated))
		unvalidatedFee := hexToBig(t, unvalidated[0].OverallFee)
		assert.Negative(t, unvalidatedFee.Cmp(hexToBig(t, validated[0].OverallFee)))
	})

	t.Run("a block with no L2 and no L1 data gas price uses the price 1", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		transfer := devnet.Account(t, "0x101").InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000))

		// The genesis block has an L1 gas price of 0 and no other gas prices.
		var estimates []feeEstimate
		result := devnet.Result(t, rpcVersion, "starknet_estimateFee",
			estimateFeeParams(`{"block_number":0}`, `[]`, transfer))
		require.NoError(t, json.Unmarshal(result, &estimates))
		assert.Equal(t, "0x0", estimates[0].L1GasPrice)
		assert.Equal(t, "0x1", estimates[0].L2GasPrice)
		assert.Equal(t, "0x1", estimates[0].L1DataGasPrice)
		// The VM charges the price 1 for the L1 gas price 0, so overall_fee is not the sum of the
		// gas costs.
		assert.Positive(t, hexToBig(t, estimates[0].OverallFee).Sign())
	})

	t.Run("an estimate does not need a fee token balance", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		noSTRK := devnet.Account(t, "0x102")
		view := rpctest.Invocation{
			To:       strk,
			Selector: felt.UnsafeFromString[felt.Felt](selector("balanceOf")),
			Calldata: []felt.Felt{felt.Felt(noSTRK.Address)},
		}
		require.Zero(t, strkBalance(t, devnet.Node, `"latest"`, &noSTRK.Address).Sign())

		devnet.Result(t, rpcVersion, "starknet_estimateFee",
			estimateFeeParams(`"latest"`, `[]`, noSTRK.InvokeV3(t, 0, view)))
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		sender := devnet.Account(t, "0x101")
		transfer := sender.InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000))
		noSuchFunction := sender.InvokeV3(t, 1, rpctest.Invocation{
			To:       strk,
			Selector: felt.UnsafeFromString[felt.Felt](selector("no_such_function")),
		})
		blockNotFound := `{"code":24,"message":"Block not found"}`

		tests := []struct {
			name   string
			params string
			check  func(t *testing.T, rpcErr json.RawMessage)
		}{
			{
				name:   "missing block number",
				params: estimateFeeParams(`{"block_number":99}`, `[]`, transfer),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					require.JSONEq(t, blockNotFound, string(rpcErr))
				},
			},
			{
				name:   "missing block hash",
				params: estimateFeeParams(`{"block_hash":"0x123"}`, `[]`, transfer),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					require.JSONEq(t, blockNotFound, string(rpcErr))
				},
			},
			{
				name:   "the second transaction fails",
				params: estimateFeeParams(`"latest"`, `[]`, transfer, noSuchFunction),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireExecutionError(t, rpcErr, 1, "Result::unwrap failed")
				},
			},
			{
				name:   "simulation flag that estimateFee does not accept",
				params: estimateFeeParams(`"latest"`, `["SKIP_FEE_CHARGE"]`, transfer),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireInvalidParams(t, rpcErr, "SKIP_FEE_CHARGE")
				},
			},
			{
				name:   "no sender address",
				params: estimateFeeParams(`"latest"`, `[]`, transfer.Without("sender_address")),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireInvalidParams(t, rpcErr, "SenderAddress")
				},
			},
			{
				name:   "no resource bounds",
				params: estimateFeeParams(`"latest"`, `[]`, transfer.Without("resource_bounds")),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireInvalidParams(t, rpcErr, "ResourceBounds")
				},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response := devnet.Call(t, rpcVersion, "starknet_estimateFee", test.params)
				require.Empty(t, response.Result)
				test.check(t, response.Error)
			})
		}
	})
}

// requireInvalidParams checks that the error is INVALID_PARAMS and that its data names the
// field.
func requireInvalidParams(t *testing.T, rpcErr json.RawMessage, field string) {
	t.Helper()
	var got struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rpcErr, &got), string(rpcErr))
	require.Equal(t, -32602, got.Code)
	require.Equal(t, "Invalid Params", got.Message)
	require.Contains(t, got.Data, field)
}

func TestEstimateMessageFee(t *testing.T) {
	t.Parallel()
	devnet := rpctest.NewDevnet(t)
	// The deposit handler accepts messages from this L1 address only.
	const l1Sender = "0x8359e4b0152ed5a731162d3c7b0d8d56edb165a0"
	message := func(from, to, function string) string {
		return fmt.Sprintf(
			`{"from_address":%q,"to_address":%q,"entry_point_selector":%q,"payload":["0x105","0x10"]}`,
			from, to, selector(function),
		)
	}
	deposit := message(l1Sender, rpctest.L1HandlerAddress.String(), "deposit")
	// 5e13 wei, the lowest fee that the L1 core contract accepts for a message.
	minFee := big.NewInt(50_000_000_000_000)

	t.Run("the overall fee is at least the minimum fee", func(t *testing.T) {
		t.Parallel()
		for _, blockID := range []string{`"latest"`, `"pre_confirmed"`, `{"block_number":1}`} {
			t.Run(blockID, func(t *testing.T) {
				var got feeEstimate
				result := devnet.Result(t, rpcVersion, "starknet_estimateMessageFee",
					fmt.Sprintf(`{"message":%s,"block_id":%s}`, deposit, blockID))
				require.NoError(t, json.Unmarshal(result, &got))
				assert.Equal(t, "WEI", got.Unit)
				require.Negative(t, got.gasCost(t).Cmp(minFee), "the devnet gas prices are 1")
				assert.Equal(t, minFee, hexToBig(t, got.OverallFee))
			})
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			params  string
			wantErr string
		}{
			{
				name:    "missing block",
				params:  fmt.Sprintf(`{"message":%s,"block_id":{"block_number":99}}`, deposit),
				wantErr: `{"code":24,"message":"Block not found"}`,
			},
			{
				name: "L1 address that is not 20 bytes",
				params: fmt.Sprintf(`{"message":%s,"block_id":"latest"}`,
					message("0x1", rpctest.L1HandlerAddress.String(), "deposit")),
				wantErr: `{"code":-32602,"message":"Invalid Params","data":"invalid length 1, want 40"}`,
			},
			{
				name: "unknown contract",
				params: fmt.Sprintf(`{"message":%s,"block_id":"latest"}`,
					message(l1Sender, "0xdead", "deposit")),
				wantErr: `{"code":20,"message":"Contract not found"}`,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response := devnet.Call(t, rpcVersion, "starknet_estimateMessageFee", test.params)
				require.JSONEq(t, test.wantErr, string(response.Error))
			})
		}
	})

	t.Run("the handler fails", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name    string
			message string
			reason  string
		}{
			{
				name: "message from another L1 address",
				message: message("0x0000000000000000000000000000000000000001",
					rpctest.L1HandlerAddress.String(), "deposit"),
				reason: "An ASSERT_EQ instruction failed",
			},
			{
				name:    "no such L1 handler",
				message: message(l1Sender, rpctest.L1HandlerAddress.String(), "no_such_handler"),
				reason:  "ENTRYPOINT_NOT_FOUND",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response := devnet.Call(t, rpcVersion, "starknet_estimateMessageFee",
					fmt.Sprintf(`{"message":%s,"block_id":"latest"}`, test.message))
				var got struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
					Data    struct {
						RevertError json.RawMessage `json:"revert_error"`
					} `json:"data"`
				}
				require.NoError(t, json.Unmarshal(response.Error, &got), string(response.Error))
				assert.Equal(t, 40, got.Code)
				assert.Equal(t, "Contract error", got.Message)
				assert.Contains(t, string(got.Data.RevertError), test.reason)
			})
		}
	})
}

func TestEstimateMessageFeeAboveMinimumIsUnchanged(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpcv10.New(mockReader, mocks.NewMockSyncReader(mockCtrl), mockVM, log.NewNopZapLogger())

	msg := &rpcv10.MsgFromL1{
		To:       felt.FromUint64[felt.Address](0xABCD),
		Selector: felt.FromUint64[felt.Felt](0x1),
		Payload:  []felt.Felt{felt.FromUint64[felt.Felt](0xCAFE)},
	}
	mockState := mocks.NewMockStateReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{}, nil)
	mockState.EXPECT().ContractClassHash(&msg.To).Return(felt.ClassHash{}, nil)

	want := felt.FromUint64[felt.Felt](60_000_000_000_000)
	mockVM.EXPECT().EstimateFee(gomock.Any(), nil, gomock.Any(), mockState, gomock.Any()).
		Return(vm.ExecutionResults{
			OverallFees:      []*felt.Felt{&want},
			DataAvailability: []core.DataAvailability{{}},
			GasConsumed:      []core.GasConsumed{{}},
			Traces:           []vm.TransactionTrace{{Type: vm.TxnL1Handler}},
		}, nil)

	blockID := rpcv10.BlockIDLatest()
	got, _, err := handler.EstimateMessageFee(t.Context(), msg, &blockID)
	require.Nil(t, err)
	assert.Equal(t, want, *got.OverallFee)
}
