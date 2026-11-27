package rpcv10_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/rpc/rpctest"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func simulateParams(blockID, flags string, txns ...rpctest.Txn) string {
	transactions := make([]json.RawMessage, len(txns))
	for i := range txns {
		transactions[i] = json.RawMessage(txns[i].String())
	}
	data, err := json.Marshal(transactions)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(`{"block_id":%s,"transactions":%s,"simulation_flags":%s}`, blockID, data, flags)
}

type simulatedTransaction struct {
	TransactionTrace map[string]json.RawMessage `json:"transaction_trace"`
	FeeEstimation    json.RawMessage            `json:"fee_estimation"`
}

func TestSimulateTransactions(t *testing.T) {
	t.Parallel()
	recipient := felt.FromUint64[felt.Address](0x105)

	t.Run("the flags remove the validate and the fee transfer invocations", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		transfer := devnet.Account(t, "0x101").InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000))

		tests := []struct {
			flags           string
			wantValidate    bool
			wantFeeTransfer bool
		}{
			{flags: `[]`, wantValidate: true, wantFeeTransfer: true},
			{flags: `["SKIP_VALIDATE"]`, wantValidate: false, wantFeeTransfer: true},
			{flags: `["SKIP_FEE_CHARGE"]`, wantValidate: true, wantFeeTransfer: false},
			{flags: `["SKIP_VALIDATE","SKIP_FEE_CHARGE"]`, wantValidate: false, wantFeeTransfer: false},
		}
		for _, test := range tests {
			t.Run(test.flags, func(t *testing.T) {
				response := devnet.Call(t, rpcVersion, "starknet_simulateTransactions",
					simulateParams(`"latest"`, test.flags, transfer))
				require.Empty(t, response.Error)
				steps, err := strconv.Atoi(response.Header.Get("X-Cairo-Steps"))
				require.NoError(t, err)
				assert.Positive(t, steps)

				var simulated []simulatedTransaction
				require.NoError(t, json.Unmarshal(response.Result, &simulated))
				require.Len(t, simulated, 1)
				trace := simulated[0].TransactionTrace
				assert.JSONEq(t, `"INVOKE"`, string(trace["type"]))
				assert.Contains(t, trace, "execute_invocation")
				assert.Contains(t, trace, "state_diff")
				assert.Equal(t, test.wantValidate, trace["validate_invocation"] != nil)
				assert.Equal(t, test.wantFeeTransfer, trace["fee_transfer_invocation"] != nil)
			})
		}
	})

	t.Run("the fee and the trace gas equal the result of estimateFee", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		sender := devnet.Account(t, "0x101")
		txns := []rpctest.Txn{
			sender.InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000)),
			sender.InvokeV3(t, 1, rpctest.TransferSTRK(&recipient, 2000)),
		}
		for _, blockID := range []string{`"latest"`, `"pre_confirmed"`} {
			t.Run(blockID, func(t *testing.T) {
				var simulated []simulatedTransaction
				result := devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
					simulateParams(blockID, `[]`, txns...))
				require.NoError(t, json.Unmarshal(result, &simulated))

				var estimates []json.RawMessage
				result = devnet.Result(t, rpcVersion, "starknet_estimateFee",
					estimateFeeParams(blockID, `[]`, txns...))
				require.NoError(t, json.Unmarshal(result, &estimates))

				require.Len(t, simulated, len(estimates))
				for i := range estimates {
					assert.JSONEq(t, string(estimates[i]), string(simulated[i].FeeEstimation))

					var fee feeEstimate
					require.NoError(t, json.Unmarshal(estimates[i], &fee))
					var resources struct {
						L1Gas     uint64 `json:"l1_gas"`
						L2Gas     uint64 `json:"l2_gas"`
						L1DataGas uint64 `json:"l1_data_gas"`
					}
					trace := simulated[i].TransactionTrace
					require.NoError(t, json.Unmarshal(trace["execution_resources"], &resources))
					assert.Equal(t, hexToBig(t, fee.L1GasConsumed).Uint64(), resources.L1Gas)
					assert.Equal(t, hexToBig(t, fee.L2GasConsumed).Uint64(), resources.L2Gas)
					assert.Equal(t, hexToBig(t, fee.L1DataGasConsumed).Uint64(), resources.L1DataGas)
				}
			})
		}
	})

	t.Run("SKIP_FEE_CHARGE simulates a sender with no fee token balance", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		transfer := devnet.Account(t, "0x102").InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 0))

		response := devnet.Call(t, rpcVersion, "starknet_simulateTransactions",
			simulateParams(`"latest"`, `[]`, transfer))
		requireExecutionError(t, response.Error, 0, "exceed balance (0)")

		devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
			simulateParams(`"latest"`, `["SKIP_FEE_CHARGE"]`, transfer))
	})

	t.Run("deploy account", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		account := devnet.NewAccount(t)
		deploy := account.DeployAccountV3(t)
		badSignature := deploy.With("signature", []string{"0x1", "0x2"})

		t.Run("the account has no fee token balance", func(t *testing.T) {
			response := devnet.Call(t, rpcVersion, "starknet_simulateTransactions",
				simulateParams(`"latest"`, `[]`, deploy))
			requireExecutionError(t, response.Error, 0, "Resource bounds were not satisfied")
		})

		t.Run("SKIP_FEE_CHARGE deploys the account", func(t *testing.T) {
			var simulated []simulatedTransaction
			result := devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
				simulateParams(`"latest"`, `["SKIP_FEE_CHARGE"]`, deploy))
			require.NoError(t, json.Unmarshal(result, &simulated))
			trace := simulated[0].TransactionTrace
			assert.JSONEq(t, `"DEPLOY_ACCOUNT"`, string(trace["type"]))

			var constructor struct {
				ContractAddress felt.Address   `json:"contract_address"`
				ClassHash       felt.ClassHash `json:"class_hash"`
			}
			require.NoError(t, json.Unmarshal(trace["constructor_invocation"], &constructor))
			assert.Equal(t, account.Address, constructor.ContractAddress)
			assert.Equal(t, rpctest.AccountClassHash, constructor.ClassHash)

			var stateDiff struct {
				DeployedContracts []struct {
					Address   felt.Address   `json:"address"`
					ClassHash felt.ClassHash `json:"class_hash"`
				} `json:"deployed_contracts"`
			}
			require.NoError(t, json.Unmarshal(trace["state_diff"], &stateDiff))
			require.Len(t, stateDiff.DeployedContracts, 1)
			assert.Equal(t, account.Address, stateDiff.DeployedContracts[0].Address)
			assert.Equal(t, rpctest.AccountClassHash, stateDiff.DeployedContracts[0].ClassHash)
		})

		t.Run("SKIP_VALIDATE accepts a wrong signature", func(t *testing.T) {
			response := devnet.Call(t, rpcVersion, "starknet_simulateTransactions",
				simulateParams(`"latest"`, `["SKIP_FEE_CHARGE"]`, badSignature))
			requireExecutionError(t, response.Error, 0, "Account: invalid signature")

			devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
				simulateParams(`"latest"`, `["SKIP_FEE_CHARGE","SKIP_VALIDATE"]`, badSignature))
		})
	})

	t.Run("RETURN_INITIAL_READS", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		sender := devnet.Account(t, "0x101")
		transfer := sender.InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000))

		var withReads struct {
			SimulatedTransactions json.RawMessage `json:"simulated_transactions"`
			InitialReads          struct {
				Storage []struct {
					ContractAddress felt.Address `json:"contract_address"`
					Value           string       `json:"value"`
				} `json:"storage"`
				Nonces []struct {
					ContractAddress felt.Address `json:"contract_address"`
					Nonce           string       `json:"nonce"`
				} `json:"nonces"`
				ClassHashes []struct {
					ContractAddress felt.Address   `json:"contract_address"`
					ClassHash       felt.ClassHash `json:"class_hash"`
				} `json:"class_hashes"`
			} `json:"initial_reads"`
		}
		result := devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
			simulateParams(`"latest"`, `["RETURN_INITIAL_READS"]`, transfer))
		require.NoError(t, json.Unmarshal(result, &withReads))

		withoutReads := devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
			simulateParams(`"latest"`, `[]`, transfer))
		assert.Equal(t, sortedSets(t, withoutReads), sortedSets(t, withReads.SimulatedTransactions))

		reads := withReads.InitialReads
		var nonce string
		result = devnet.Result(t, rpcVersion, "starknet_getNonce",
			fmt.Sprintf(`{"block_id":"latest","contract_address":%q}`, sender.Address.String()))
		require.NoError(t, json.Unmarshal(result, &nonce))
		assert.Contains(t, reads.Nonces, struct {
			ContractAddress felt.Address `json:"contract_address"`
			Nonce           string       `json:"nonce"`
		}{sender.Address, nonce})

		var classHash felt.ClassHash
		result = devnet.Result(t, rpcVersion, "starknet_getClassHashAt",
			fmt.Sprintf(`{"block_id":"latest","contract_address":%q}`, sender.Address.String()))
		require.NoError(t, json.Unmarshal(result, &classHash))
		assert.Contains(t, reads.ClassHashes, struct {
			ContractAddress felt.Address   `json:"contract_address"`
			ClassHash       felt.ClassHash `json:"class_hash"`
		}{sender.Address, classHash})

		// The balance of the sender is less than 2^128, so the VM reads it from one storage slot.
		balance := strkBalance(t, devnet.Node, `"latest"`, &sender.Address)
		var strkValues []*big.Int
		for _, entry := range reads.Storage {
			if entry.ContractAddress == strk {
				strkValues = append(strkValues, hexToBig(t, entry.Value))
			}
		}
		assert.Contains(t, strkValues, balance)
	})

	t.Run("a transaction that fails in __execute__ is reverted", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		sender := devnet.Account(t, "0x101")
		noSuchFunction := sender.InvokeV3(t, 0, rpctest.Invocation{
			To:       strk,
			Selector: felt.UnsafeFromString[felt.Felt](selector("no_such_function")),
		})

		var simulated []simulatedTransaction
		result := devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
			simulateParams(`"latest"`, `[]`, noSuchFunction))
		require.NoError(t, json.Unmarshal(result, &simulated))
		var execute struct {
			RevertReason string `json:"revert_reason"`
		}
		require.NoError(t, json.Unmarshal(simulated[0].TransactionTrace["execute_invocation"], &execute))
		assert.Contains(t, execute.RevertReason, "Result::unwrap failed")
		assert.NotNil(t, simulated[0].TransactionTrace["fee_transfer_invocation"])
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

		tests := []struct {
			name   string
			params string
			check  func(t *testing.T, rpcErr json.RawMessage)
		}{
			{
				name:   "missing block",
				params: simulateParams(`{"block_number":99}`, `[]`, transfer),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					require.JSONEq(t, `{"code":24,"message":"Block not found"}`, string(rpcErr))
				},
			},
			{
				name: "the second transaction fails validation",
				params: simulateParams(`"latest"`, `[]`,
					transfer, noSuchFunction.With("signature", []string{"0x1", "0x2"})),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireExecutionError(t, rpcErr, 1, "Account: invalid signature")
				},
			},
			{
				name:   "unknown flag",
				params: simulateParams(`"latest"`, `["SKIP_EXECUTE"]`, transfer),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireInvalidParams(t, rpcErr, "SKIP_EXECUTE")
				},
			},
			{
				name:   "invoke with no sender address",
				params: simulateParams(`"latest"`, `[]`, transfer.Without("sender_address")),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireInvalidParams(t, rpcErr, "SenderAddress")
				},
			},
			{
				name:   "invoke with no resource bounds",
				params: simulateParams(`"latest"`, `[]`, transfer.Without("resource_bounds")),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireInvalidParams(t, rpcErr, "ResourceBounds")
				},
			},
			{
				name: "deploy account with no resource bounds",
				params: simulateParams(`"latest"`, `[]`,
					devnet.NewAccount(t).DeployAccountV3(t).Without("resource_bounds")),
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireInvalidParams(t, rpcErr, "ResourceBounds")
				},
			},
			{
				name: "declare with no sender address",
				params: `{"block_id":"latest","transactions":[{"type":"DECLARE","version":"0x3"}],` +
					`"simulation_flags":[]}`,
				check: func(t *testing.T, rpcErr json.RawMessage) {
					requireInvalidParams(t, rpcErr, "SenderAddress")
				},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response := devnet.Call(t, rpcVersion, "starknet_simulateTransactions", test.params)
				require.Empty(t, response.Result)
				test.check(t, response.Error)
			})
		}
	})
}

func TestSimulateTransactionsInconsistentVMResults(t *testing.T) {
	t.Parallel()
	mockCtrl := gomock.NewController(t)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	mockState := mocks.NewMockStateReader(mockCtrl)
	header := &core.Header{}
	mockReader.EXPECT().Network().Return(&networks.Mainnet)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
	mockReader.EXPECT().HeadsHeader().Return(header, nil)
	mockVM.EXPECT().Simulate([]core.Transaction{}, nil, &vm.BlockInfo{Header: header}, mockState,
		vm.SimulateOptions{SkipValidate: true}).
		Return(vm.ExecutionResults{
			OverallFees:      []*felt.Felt{new(felt.Zero[felt.Felt]())},
			DataAvailability: []core.DataAvailability{{L1Gas: 0}, {L1Gas: 0}},
			GasConsumed:      []core.GasConsumed{{L1Gas: 0, L1DataGas: 0, L2Gas: 0}},
			Traces:           []vm.TransactionTrace{{}},
		}, nil)

	handler := rpcv10.New(mockReader, nil, mockVM, log.NewNopZapLogger())
	blockID := rpcv10.BlockIDLatest()
	flags := []rpcv10.SimulationFlag{rpcv10.SkipValidateFlag}
	_, _, err := handler.SimulateTransactions(
		t.Context(), &blockID, rpcv10.BroadcastedTransactionInputs{}, flags,
	)
	require.Equal(t, rpccore.ErrInternal.CloneWithData(errors.New(
		"inconsistent lengths: 1 overall fees, 1 traces, 1 gas consumed, 2 data availability, 0 txns",
	)), err)
}

func TestSimulateTransactionsShouldErrorWithoutSenderAddressOrResourceBounds(t *testing.T) {
	t.Parallel()
	n := &networks.Mainnet

	version3 := felt.FromUint64[felt.Felt](3)

	tests := []struct {
		name         string
		transactions []rpcv10.BroadcastedTransaction
		err          *jsonrpc.Error
	}{
		{
			name: "declare transaction without sender address",
			transactions: []rpcv10.BroadcastedTransaction{
				{
					Transaction: rpcv10.Transaction{
						Version: &version3,
						Type:    rpcv10.TxnDeclare,
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"sender_address is required for this transaction type",
			),
		},
		{
			name: "declare transaction without resource bounds",
			transactions: []rpcv10.BroadcastedTransaction{
				{
					Transaction: rpcv10.Transaction{
						Version:       &version3,
						Type:          rpcv10.TxnDeclare,
						SenderAddress: new(felt.Zero[felt.Address]()),
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"resource_bounds is required for this transaction type",
			),
		},
		{
			name: "invoke transaction without sender address",
			transactions: []rpcv10.BroadcastedTransaction{
				{
					Transaction: rpcv10.Transaction{
						Version: &version3,
						Type:    rpcv10.TxnInvoke,
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"sender_address is required for this transaction type",
			),
		},
		{
			name: "invoke transaction without resource bounds",
			transactions: []rpcv10.BroadcastedTransaction{
				{
					Transaction: rpcv10.Transaction{
						Version:       &version3,
						Type:          rpcv10.TxnInvoke,
						SenderAddress: new(felt.Zero[felt.Address]()),
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"resource_bounds is required for this transaction type",
			),
		},
		{
			name: "deploy account transaction without resource bounds",
			transactions: []rpcv10.BroadcastedTransaction{
				{
					Transaction: rpcv10.Transaction{
						Version: &version3,
						Type:    rpcv10.TxnDeployAccount,
					},
				},
			},
			err: jsonrpc.Err(
				jsonrpc.InvalidParams,
				"resource_bounds is required for this transaction type",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockReader := mocks.NewMockReader(mockCtrl)
			mockVM := mocks.NewMockVM(mockCtrl)

			mockReader.EXPECT().Network().Return(n)

			handler := rpcv10.New(mockReader, nil, mockVM, log.NewNopZapLogger())

			blockID := rpcv10.BlockIDLatest()
			_, _, err := handler.SimulateTransactions(
				t.Context(),
				&blockID,
				rpcv10.BroadcastedTransactionInputs{Data: test.transactions},
				[]rpcv10.SimulationFlag{},
			)
			if test.err != nil {
				require.Equal(t, test.err, err)
				return
			}
			require.Nil(t, err)
		})
	}
}

func TestSimulateTransactionsResponseMarshalJSON(t *testing.T) {
	simulated := []rpcv10.SimulatedTransaction{{
		FeeEstimation: rpcv10.FeeEstimate{OverallFee: felt.NewFromUint64[felt.Felt](7)},
	}}

	t.Run("without initial reads marshals as an array", func(t *testing.T) {
		data, err := json.Marshal(rpcv10.SimulateTransactionsResponse{SimulatedTransactions: simulated})
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(string(data), "["), string(data))
		require.NotContains(t, string(data), "initial_reads")
	})

	t.Run("with initial reads marshals as an object", func(t *testing.T) {
		data, err := json.Marshal(rpcv10.SimulateTransactionsResponse{
			SimulatedTransactions: simulated,
			InitialReads:          &rpcv10.InitialReads{},
		})
		require.NoError(t, err)
		var decoded map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &decoded))
		require.Len(t, decoded, 2)
		require.Contains(t, decoded, "simulated_transactions")
		require.Contains(t, decoded, "initial_reads")
	})
}

func TestTraceBlockTransactionsResponseMarshalJSON(t *testing.T) {
	traces := []rpcv10.TracedBlockTransaction{
		{TransactionHash: felt.NewFromUint64[felt.TransactionHash](9)},
	}

	t.Run("without initial reads marshals as an array", func(t *testing.T) {
		data, err := json.Marshal(rpcv10.TraceBlockTransactionsResponse{Traces: traces})
		require.NoError(t, err)
		require.JSONEq(t, `[{"transaction_hash":"0x9"}]`, string(data))
	})

	t.Run("with initial reads marshals as an object", func(t *testing.T) {
		data, err := json.Marshal(rpcv10.TraceBlockTransactionsResponse{
			Traces:       traces,
			InitialReads: &rpcv10.InitialReads{},
		})
		require.NoError(t, err)
		var decoded map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &decoded))
		require.Len(t, decoded, 2)
		require.JSONEq(t, `[{"transaction_hash":"0x9"}]`, string(decoded["traces"]))
		require.Contains(t, decoded, "initial_reads")
	})
}
