package rpcv10_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/adapters/adaptfeeder/adaptfeedertest"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/rpc/rpctest"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCall(t *testing.T) {
	t.Parallel()
	recipient := felt.FromUint64[felt.Address](0x105)

	t.Run("balanceOf reads the state of the block", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		sender := devnet.Account(t, "0x101")
		// genesis_prefund_accounts.json mints this amount of STRK to 0x101.
		genesisBalance := hexToBig(t, "0x66bc75e2d63100000")

		transfer := sender.InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000))
		devnet.Submit(t, transfer)
		devnet.MakeBlock(t)
		fee := hexToBig(t, getReceipt(t, devnet.Node, &transfer.Hash).ActualFee.Amount)
		balanceAfter := new(big.Int).Sub(genesisBalance, new(big.Int).Add(big.NewInt(1000), fee))

		block1, err := devnet.Chain.BlockHeaderByNumber(1)
		require.NoError(t, err)
		require.NoError(t, devnet.Chain.SetL1Head(&core.L1Head{
			BlockNumber: block1.Number,
			BlockHash:   block1.Hash,
			StateRoot:   block1.GlobalStateRoot,
		}))

		tests := []struct {
			blockID string
			want    *big.Int
		}{
			{blockID: `"latest"`, want: balanceAfter},
			{blockID: `"pre_confirmed"`, want: balanceAfter},
			{blockID: `{"block_number":2}`, want: balanceAfter},
			{blockID: `{"block_number":1}`, want: genesisBalance},
			{blockID: fmt.Sprintf(`{"block_hash":%q}`, block1.Hash), want: genesisBalance},
			{blockID: `"l1_accepted"`, want: genesisBalance},
		}
		for _, test := range tests {
			t.Run(test.blockID, func(t *testing.T) {
				assert.Equal(t, test.want, strkBalance(t, devnet.Node, test.blockID, &sender.Address))
			})
		}
		assert.Equal(t, big.NewInt(1000), strkBalance(t, devnet.Node, `"latest"`, &recipient))
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		devnet.MakeBlock(t)
		emptyChain := rpctest.NewSepolia(t, 0)
		unknown := felt.FromUint64[felt.Address](0xdead)
		blockNotFound := `{"code":24,"message":"Block not found"}`

		tests := []struct {
			name    string
			node    *rpctest.Node
			params  string
			wantErr string
		}{
			{
				name:    "unknown contract",
				node:    devnet.Node,
				params:  callParams(&unknown, "balanceOf", `"latest"`),
				wantErr: `{"code":20,"message":"Contract not found"}`,
			},
			{
				name:    "unknown contract at an older block",
				node:    devnet.Node,
				params:  callParams(&unknown, "balanceOf", `{"block_number":1}`),
				wantErr: `{"code":20,"message":"Contract not found"}`,
			},
			{
				name:    "missing block number",
				node:    devnet.Node,
				params:  callParams(&strk, "balanceOf", `{"block_number":99}`, "0x101"),
				wantErr: blockNotFound,
			},
			{
				name:    "missing block hash",
				node:    devnet.Node,
				params:  callParams(&strk, "balanceOf", `{"block_hash":"0x123"}`, "0x101"),
				wantErr: blockNotFound,
			},
			{
				name:    "no L1 head",
				node:    devnet.Node,
				params:  callParams(&strk, "balanceOf", `"l1_accepted"`, "0x101"),
				wantErr: blockNotFound,
			},
			{
				name:    "chain with no blocks",
				node:    emptyChain,
				params:  callParams(&strk, "balanceOf", `"latest"`, "0x101"),
				wantErr: blockNotFound,
			},
			{
				name:    "pre_confirmed on a chain with no blocks",
				node:    emptyChain,
				params:  callParams(&strk, "balanceOf", `"pre_confirmed"`, "0x101"),
				wantErr: blockNotFound,
			},
			{
				name:    "unknown entry point",
				node:    devnet.Node,
				params:  callParams(&strk, "no_such_function", `"latest"`),
				wantErr: `{"code":21,"message":"Requested entry point does not exist in the contract"}`,
			},
			{
				name:   "the contract fails",
				node:   devnet.Node,
				params: callParams(&strk, "transfer", `"latest"`, recipient.String(), "0x1", "0x0"),
				// "ERC20: transfer from 0": a call has no caller.
				wantErr: `{"code":40,"message":"Contract error",` +
					`"data":{"revert_error":"0x45524332303a207472616e736665722066726f6d2030"}}`,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response := test.node.Call(t, rpcVersion, "starknet_call", test.params)
				require.Empty(t, response.Result)
				require.JSONEq(t, test.wantErr, string(response.Error))
			})
		}
	})
}

type tracedBlockTransaction struct {
	TransactionHash felt.TransactionHash `json:"transaction_hash"`
	TraceRoot       json.RawMessage      `json:"trace_root"`
}

func TestTraceBlockTransactions(t *testing.T) {
	t.Parallel()
	recipient := felt.FromUint64[felt.Address](0x105)

	t.Run("the traces equal the simulated traces", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		sender := devnet.Account(t, "0x101")
		txns := []rpctest.Txn{
			sender.InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000)),
			sender.InvokeV3(t, 1, rpctest.Invocation{
				To:       strk,
				Selector: felt.UnsafeFromString[felt.Felt](selector("no_such_function")),
			}),
		}
		devnet.Submit(t, txns...)
		header := devnet.MakeBlock(t)

		var simulated []struct {
			TransactionTrace json.RawMessage `json:"transaction_trace"`
		}
		result := devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
			simulateParams(fmt.Sprintf(`{"block_number":%d}`, header.Number-1), `[]`, txns...))
		require.NoError(t, json.Unmarshal(result, &simulated))

		for _, blockID := range []string{
			`"latest"`,
			fmt.Sprintf(`{"block_number":%d}`, header.Number),
			fmt.Sprintf(`{"block_hash":%q}`, header.Hash),
		} {
			t.Run(blockID, func(t *testing.T) {
				var traces []tracedBlockTransaction
				result := devnet.Result(t, rpcVersion, "starknet_traceBlockTransactions",
					fmt.Sprintf(`{"block_id":%s}`, blockID))
				require.NoError(t, json.Unmarshal(result, &traces))
				require.Len(t, traces, len(txns))
				for i := range txns {
					assert.Equal(t, txns[i].Hash, traces[i].TransactionHash)
					assert.Equal(t,
						sortedSets(t, simulated[i].TransactionTrace),
						sortedSets(t, traces[i].TraceRoot),
					)
				}
			})
		}
	})

	t.Run("RETURN_INITIAL_READS returns the initial reads of the simulation", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		transfer := devnet.Account(t, "0x101").InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000))
		devnet.Submit(t, transfer)
		header := devnet.MakeBlock(t)

		parentID := fmt.Sprintf(`{"block_number":%d}`, header.Number-1)
		simulated := devnet.Result(t, rpcVersion, "starknet_simulateTransactions",
			simulateParams(parentID, `["RETURN_INITIAL_READS"]`, transfer))
		blockID := fmt.Sprintf(`{"block_number":%d}`, header.Number)
		traced := devnet.Result(t, rpcVersion, "starknet_traceBlockTransactions",
			fmt.Sprintf(`{"block_id":%s,"trace_flags":["RETURN_INITIAL_READS"]}`, blockID))
		tracedSets := sortedSets(t, traced).(map[string]any)
		assert.Equal(t, sortedSets(t, simulated).(map[string]any)["initial_reads"],
			tracedSets["initial_reads"])

		withoutFlags := devnet.Result(t, rpcVersion, "starknet_traceBlockTransactions",
			fmt.Sprintf(`{"block_id":%s}`, blockID))
		assert.Equal(t, sortedSets(t, withoutFlags), tracedSets["traces"])
	})

	t.Run("old blocks come from the feeder", func(t *testing.T) {
		t.Parallel()
		sepolia := rpctest.NewSepolia(t, 4)
		want, err := os.ReadFile("testdata/traces/sepolia_block_3.json")
		require.NoError(t, err)

		response := sepolia.Call(t, rpcVersion, "starknet_traceBlockTransactions",
			`{"block_id":{"block_number":3}}`)
		require.Empty(t, response.Error)
		assert.JSONEq(t, string(want), string(response.Result))
		assert.Equal(t, "0", response.Header.Get("X-Cairo-Steps"))
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		emptyChain := rpctest.NewSepolia(t, 0)
		blockNotFound := `{"code":24,"message":"Block not found"}`
		preConfirmed := `{"code":70,"message":` +
			`"This method does not support being called on the pre_confirmed block"}`

		tests := []struct {
			name    string
			node    *rpctest.Node
			blockID string
			wantErr string
		}{
			{
				name: "missing block number", node: devnet.Node, blockID: `{"block_number":99}`,
				wantErr: blockNotFound,
			},
			{
				name: "missing block hash", node: devnet.Node, blockID: `{"block_hash":"0x123"}`,
				wantErr: blockNotFound,
			},
			{name: "no L1 head", node: devnet.Node, blockID: `"l1_accepted"`, wantErr: blockNotFound},
			{name: "pre_confirmed", node: devnet.Node, blockID: `"pre_confirmed"`, wantErr: preConfirmed},
			{
				name: "latest of a chain with no blocks", node: emptyChain, blockID: `"latest"`,
				wantErr: blockNotFound,
			},
			{
				name: "pre_confirmed of a chain with no blocks", node: emptyChain, blockID: `"pre_confirmed"`,
				wantErr: preConfirmed,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				response := test.node.Call(t, rpcVersion, "starknet_traceBlockTransactions",
					fmt.Sprintf(`{"block_id":%s}`, test.blockID))
				require.Empty(t, response.Result)
				require.JSONEq(t, test.wantErr, string(response.Error))
				assert.Equal(t, "0", response.Header.Get("X-Cairo-Steps"))
			})
		}
	})
}

func TestTraceTransaction(t *testing.T) {
	t.Parallel()
	recipient := felt.FromUint64[felt.Address](0x105)

	t.Run("the trace equals the trace in the block trace", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		sender := devnet.Account(t, "0x101")
		txns := []rpctest.Txn{
			sender.InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000)),
			sender.InvokeV3(t, 1, rpctest.TransferSTRK(&recipient, 2000)),
		}
		devnet.Submit(t, txns...)
		devnet.MakeBlock(t)

		var traces []tracedBlockTransaction
		result := devnet.Result(t, rpcVersion, "starknet_traceBlockTransactions", `{"block_id":"latest"}`)
		require.NoError(t, json.Unmarshal(result, &traces))
		for i := range txns {
			got := devnet.Result(t, rpcVersion, "starknet_traceTransaction",
				fmt.Sprintf(`{"transaction_hash":%q}`, txns[i].Hash.String()))
			assert.Equal(t, sortedSets(t, traces[i].TraceRoot), sortedSets(t, got))
		}
	})

	t.Run("a reverted transaction has the revert reason of its receipt", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		reverted := devnet.Account(t, "0x101").InvokeV3(t, 0, rpctest.Invocation{
			To:       strk,
			Selector: felt.UnsafeFromString[felt.Felt](selector("no_such_function")),
		})
		devnet.Submit(t, reverted)
		devnet.MakeBlock(t)

		got := getReceipt(t, devnet.Node, &reverted.Hash)
		require.Equal(t, "REVERTED", got.ExecutionStatus)

		var trace struct {
			ExecuteInvocation struct {
				RevertReason string `json:"revert_reason"`
			} `json:"execute_invocation"`
		}
		result := devnet.Result(t, rpcVersion, "starknet_traceTransaction",
			fmt.Sprintf(`{"transaction_hash":%q}`, reverted.Hash.String()))
		require.NoError(t, json.Unmarshal(result, &trace))
		assert.Equal(t, got.RevertReason, trace.ExecuteInvocation.RevertReason)
	})

	t.Run("old blocks come from the feeder", func(t *testing.T) {
		t.Parallel()
		sepolia := rpctest.NewSepolia(t, 4)
		var traces []tracedBlockTransaction
		result := sepolia.Result(t, rpcVersion, "starknet_traceBlockTransactions",
			`{"block_id":{"block_number":3}}`)
		require.NoError(t, json.Unmarshal(result, &traces))

		for _, traced := range traces {
			got := sepolia.Result(t, rpcVersion, "starknet_traceTransaction",
				fmt.Sprintf(`{"transaction_hash":%q}`, traced.TransactionHash.String()))
			assert.JSONEq(t, string(traced.TraceRoot), string(got))
		}
	})

	t.Run("unknown transaction", func(t *testing.T) {
		t.Parallel()
		response := rpctest.NewDevnet(t).Call(t, rpcVersion, "starknet_traceTransaction",
			`{"transaction_hash":"0x123"}`)
		require.JSONEq(t, `{"code":29,"message":"Transaction hash not found"}`, string(response.Error))
	})
}

type expectedBlockTrace struct {
	blockHash   string
	blockNumber uint64
	wantTrace   []rpcv10.TracedBlockTransaction
}

// readTestData reads a JSON file from the testdata directory
// Returns the content as the specified type T
func readTestData[T any](t *testing.T, path string) T {
	var result T

	// Get the current test file directory using runtime.Caller
	_, testFile, _, ok := runtime.Caller(1)
	require.True(t, ok, "failed to get caller info")

	testDir := filepath.Dir(testFile)
	dataPath := filepath.Join(testDir, "testdata", path)

	content, err := os.ReadFile(dataPath)
	require.NoError(t, err)

	require.NoError(
		t,
		json.Unmarshal(content, &result),
	)

	return result
}

// readTestDataAsString reads a JSON file from the testdata directory
// and returns it as a string. Used for JSON format comparison.
func readTestDataAsString(t *testing.T, path string) string {
	t.Helper()

	_, testFile, _, ok := runtime.Caller(1)
	require.True(t, ok, "failed to get caller info")

	testDir := filepath.Dir(testFile)
	dataPath := filepath.Join(testDir, "testdata", path)

	content, err := os.ReadFile(dataPath)
	require.NoError(t, err)

	// Verify it's valid JSON before returning
	var data interface{}
	require.NoError(t, json.Unmarshal(content, &data), "Expected JSON file to contain valid JSON")

	return string(content)
}

func TestTraceFallback(t *testing.T) {
	t.Run("Goerli Integration", func(t *testing.T) {
		tests := map[string]expectedBlockTrace{
			"old block": {
				blockHash:   "0x3ae41b0f023e53151b0c8ab8b9caafb7005d5f41c9ab260276d5bdc49726279",
				blockNumber: 0,
				wantTrace: readTestData[[]rpcv10.TracedBlockTransaction](
					t,
					"traces/goerli_block_0.json",
				),
			},
			// The newer block still needs to have starknet_version <= 0.13.1 to be fetched from the feeder
			"newer block": {
				blockHash:   "0xe3828bd9154ab385e2cbb95b3b650365fb3c6a4321660d98ce8b0a9194f9a3",
				blockNumber: 300000,
				wantTrace: readTestData[[]rpcv10.TracedBlockTransaction](
					t,
					"traces/goerli_block_300_000.json",
				),
			},
		}

		AssertTracedBlockTransactions(t, &networks.Integration, tests)
	})

	t.Run("Sepolia", func(t *testing.T) {
		tests := map[string]expectedBlockTrace{
			// The newer block still needs to have starknet_version <= 0.13.1 to be fetched from the feeder
			"newer block": {
				blockHash:   "0x733495d0744edd9785b400408fa87c8ad599f81859df544897f80a3fceab422",
				blockNumber: 40000,
				wantTrace: readTestData[[]rpcv10.TracedBlockTransaction](
					t,
					"traces/sepolia_block_40_000.json",
				),
			},
		}

		AssertTracedBlockTransactions(t, &networks.Sepolia, tests)
	})
}

func AssertTracedBlockTransactions(
	t *testing.T,
	n *networks.Network,
	tests map[string]expectedBlockTrace,
) {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	client := feeder.NewTestClient(t, n)

	mockReader := mocks.NewMockReader(mockCtrl)

	mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).
		DoAndReturn(func(number uint64) (*core.Header, error) {
			block := adaptfeedertest.Block(t, client, number)
			return block.Header, nil
		}).AnyTimes()

	mockReader.EXPECT().TransactionsAndReceiptsByBlockNumber(gomock.Any()).
		DoAndReturn(func(number uint64) ([]core.Transaction, []*core.TransactionReceipt, error) {
			block := adaptfeedertest.Block(t, client, number)

			// Simulate gas consumption in block receipts
			for _, receipt := range block.Receipts {
				receipt.ExecutionResources.TotalGasConsumed = &core.GasConsumed{
					L1Gas:     5,
					L2Gas:     10,
					L1DataGas: 15,
				}
			}
			return block.Transactions, block.Receipts, nil
		}).AnyTimes()

	mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()
	mockReader.EXPECT().Network().Return(n).AnyTimes()

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			handler := rpcv10.New(mockReader, nil, nil, nil)
			handler = handler.WithFeeder(client)
			blockID := rpcv10.BlockIDFromNumber(test.blockNumber)
			tracesResp, httpHeader, err := handler.TraceBlockTransactions(t.Context(), &blockID, nil)
			require.Nil(t, err)
			traces := tracesResp.Traces
			if n == &networks.Sepolia && description == "newer block" {
				// For the newer block test, we test 3 of the block traces (INVOKE, DEPLOY_ACCOUNT, DECLARE)
				traces = []rpcv10.TracedBlockTransaction{traces[0], traces[7], traces[11]}
			}
			require.Nil(t, err)
			require.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), "0")
			require.Equal(t, test.wantTrace, traces)
		})
	}
}

func TestTraceBlockTransactionsReturnsError(t *testing.T) {
	t.Run("no feeder client set", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockReader := mocks.NewMockReader(mockCtrl)

		network := networks.Sepolia
		client := feeder.NewTestClient(t, &network)

		blockNumber := uint64(40000)

		mockReader.EXPECT().BlockHeaderByNumber(gomock.Any()).
			DoAndReturn(func(number uint64) (*core.Header, error) {
				block := adaptfeedertest.Block(t, client, number)
				return block.Header, nil
			})
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()
		mockReader.EXPECT().Network().Return(&network)

		// No feeder client is set
		handler := rpcv10.New(mockReader, nil, nil, nil)

		blockID := rpcv10.BlockIDFromNumber(blockNumber)
		tracedBlocks, httpHeader, err := handler.TraceBlockTransactions(t.Context(), &blockID, nil)

		require.Nil(t, tracedBlocks.Traces)
		require.Equal(t, rpccore.ErrInternal.Code, err.Code)
		assert.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), "0")
	})
}

func TestTransactionTraceValidation(t *testing.T) {
	validInvokeTransactionTrace := rpcv10.TransactionTrace{
		Type:              rpcv10.TxnInvoke,
		ExecuteInvocation: &rpcv10.ExecuteInvocation{},
	}

	invalidInvokeTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv10.TxnInvoke,
	}

	validDeployAccountTransactionTrace := rpcv10.TransactionTrace{
		Type:                  rpcv10.TxnDeployAccount,
		ConstructorInvocation: &rpcv10.FunctionInvocation{},
	}

	invalidDeployAccountTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv10.TxnDeployAccount,
	}

	validL1HandlerTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv10.TxnL1Handler,
		FunctionInvocation: &rpcv10.ExecuteInvocation{
			FunctionInvocation: &rpcv10.FunctionInvocation{},
		},
	}

	validRevertedL1HandlerTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv10.TxnL1Handler,
		FunctionInvocation: &rpcv10.ExecuteInvocation{
			RevertReason: "Reverted",
		},
	}

	invalidL1HandlerTransactionTrace := rpcv10.TransactionTrace{
		Type: rpcv10.TxnL1Handler,
	}

	tests := []struct {
		name         string
		trace        rpcv10.TransactionTrace
		wantErr      bool
		expectedJSON string // Expected JSON output (empty if validation should fail)
	}{
		{
			name:         "valid INVOKE tx",
			trace:        validInvokeTransactionTrace,
			wantErr:      false,
			expectedJSON: readTestDataAsString(t, "traces/validation/valid_invoke_trace.json"),
		},
		{
			name:         "invalid INVOKE tx",
			trace:        invalidInvokeTransactionTrace,
			wantErr:      true,
			expectedJSON: "",
		},
		{
			name:         "valid DEPLOY_ACCOUNT tx",
			trace:        validDeployAccountTransactionTrace,
			wantErr:      false,
			expectedJSON: readTestDataAsString(t, "traces/validation/valid_deploy_account_trace.json"),
		},
		{
			name:         "invalid DEPLOY_ACCOUNT tx",
			trace:        invalidDeployAccountTransactionTrace,
			wantErr:      true,
			expectedJSON: "",
		},
		{
			name:         "valid L1_HANDLER tx",
			trace:        validL1HandlerTransactionTrace,
			wantErr:      false,
			expectedJSON: readTestDataAsString(t, "traces/validation/valid_l1_handler_trace.json"),
		},
		{
			name:         "valid L1_HANDLER tx reverted",
			trace:        validRevertedL1HandlerTransactionTrace,
			wantErr:      false,
			expectedJSON: readTestDataAsString(t, "traces/validation/valid_l1_handler_reverted_trace.json"),
		},
		{
			name:         "invalid L1_HANDLER tx",
			trace:        invalidL1HandlerTransactionTrace,
			wantErr:      true,
			expectedJSON: "",
		},
	}

	validate := rpcv10.Validator()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validate.Struct(test.trace)

			if test.wantErr {
				assert.Error(t, err, "Expected validation to fail, but it passed")
			} else {
				assert.NoError(t, err, "Expected validation to pass, but it failed")

				jsonBytes, marshalErr := json.Marshal(test.trace)
				require.NoError(t, marshalErr)
				assert.JSONEq(t, test.expectedJSON, string(jsonBytes))
			}
		})
	}
}

func TestTraceTransactionWithMockedReader(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader.EXPECT().Network().Return(&networks.Mainnet).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpcv10.New(mockReader, mockSyncReader, mockVM, log.NewNopZapLogger())

	t.Run("not found", func(t *testing.T) {
		t.Run("other error", func(t *testing.T) {
			hash := felt.NewUnsafeFromString[felt.TransactionHash]("0xBBBB")
			// The tx-hash index lookup fails for a non-missing-key reason
			mockSyncReader.EXPECT().ReadPreConfirmedChain().Return(emptyPreConfirmedChain(t), nil)
			mockReader.EXPECT().BlockNumberAndIndexByTxHash(hash).
				Return(uint64(0), uint64(0), errors.New("database error"))

			trace, httpHeader, err := handler.TraceTransaction(t.Context(), hash)
			assert.Empty(t, trace)
			assert.Equal(t, rpccore.ErrInternal.CloneWithData(errors.New("database error")), err)
			assert.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), "0")
		})
	})

	t.Run("pre_confirmed block", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.TransactionHash](
			"0xceb6a374aff2bbb3537cf35f50df8634b2354a21",
		)
		tx := &core.InvokeTransaction{
			TransactionHash: hash,
			Version:         felt.NewFromUint64[core.TransactionVersion](1),
		}

		header := &core.Header{
			Number:           1,
			SequencerAddress: felt.NewUnsafeFromString[felt.Address]("0X111"),
			ProtocolVersion:  "99.12.3",
			L1DAMode:         core.Calldata,
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x1"),
		}
		require.Nil(t, header.Hash, "hash must be nil for pre_confirmed block")
		require.Nil(t, header.ParentHash, "ParentHash must be nil for pre_confirmed block")

		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}

		preConfirmedStateDiff := core.EmptyStateDiff()
		preConfirmed := pending.PreConfirmed{
			Block: block,
			StateUpdate: &core.StateUpdate{
				StateDiff: &preConfirmedStateDiff,
			},
		}
		mockSyncReader.EXPECT().ReadPreConfirmedChain().Return(mustNewChain(t, &preConfirmed), nil)
		headState := mocks.NewMockStateReader(mockCtrl)
		mockReader.EXPECT().StateAtBlockNumber(header.Number-1).
			Return(headState, nopCloser, nil)

		vmTrace := readTestData[vm.TransactionTrace](t, "traces/vm_transaction_trace.json")

		gc := []core.GasConsumed{{L1Gas: 2, L1DataGas: 3, L2Gas: 4}}
		overallFee := []*felt.Felt{felt.NewFromUint64[felt.Felt](1)}

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Trace(
			[]core.Transaction{tx},
			nil,
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			vm.TraceOptions{},
		).
			Return(vm.ExecutionResults{
				OverallFees: overallFee,
				GasConsumed: gc,
				Traces:      []vm.TransactionTrace{vmTrace},
				NumSteps:    stepsUsed,
			}, nil)

		trace, httpHeader, rpcErr := handler.TraceTransaction(t.Context(), hash)
		require.Nil(t, rpcErr)
		assert.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), stepsUsedStr)

		vmTrace.ExecutionResources = &vm.ExecutionResources{
			L1Gas:     2,
			L1DataGas: 3,
			L2Gas:     4,
		}
		assert.Equal(t, rpcv10.AdaptVMTransactionTrace(&vmTrace), trace)
	})

	// Multi-block pre_confirmed chain: target tx lives in the BASE entry, not
	// the tip. TraceTransaction must walk the chain newest-first and
	// reconstruct state at the matching entry.
	t.Run("pre_confirmed multi-block chain - tx in non-tip entry", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.TransactionHash]("0xdeadbeef")
		tx := &core.InvokeTransaction{
			TransactionHash: hash,
			Version:         felt.NewFromUint64[core.TransactionVersion](1),
		}

		baseHeader := &core.Header{
			Number:           5,
			SequencerAddress: felt.NewUnsafeFromString[felt.Address]("0X111"),
			ProtocolVersion:  "99.12.3",
			L1DAMode:         core.Calldata,
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x1"),
		}
		baseBlock := &core.Block{Header: baseHeader, Transactions: []core.Transaction{tx}}
		baseDiff := core.EmptyStateDiff()
		baseEntry := pending.PreConfirmed{
			Block:                 baseBlock,
			StateUpdate:           &core.StateUpdate{StateDiff: &baseDiff},
			TransactionStateDiffs: []*core.StateDiff{&baseDiff},
		}

		tipHeader := *baseHeader
		tipHeader.Number = baseHeader.Number + 1
		tipDiff := core.EmptyStateDiff()
		tipEntry := pending.PreConfirmed{
			Block:       &core.Block{Header: &tipHeader},
			StateUpdate: &core.StateUpdate{StateDiff: &tipDiff},
		}

		mockSyncReader.EXPECT().ReadPreConfirmedChain().
			Return(mustNewChain(t, &baseEntry, &tipEntry), nil)
		// Base resolution: bottom (= baseHeader.Number) - 1.
		mockReader.EXPECT().StateAtBlockNumber(baseHeader.Number-1).
			Return(mocks.NewMockStateReader(mockCtrl), nopCloser, nil)

		vmTrace := readTestData[vm.TransactionTrace](t, "traces/vm_transaction_trace.json")
		mockVM.EXPECT().Trace(
			[]core.Transaction{tx},
			nil,
			[]*felt.Felt{},
			&vm.BlockInfo{Header: baseHeader},
			gomock.Any(),
			vm.TraceOptions{},
		).Return(vm.ExecutionResults{
			OverallFees: []*felt.Felt{felt.NewFromUint64[felt.Felt](1)},
			GasConsumed: []core.GasConsumed{{L1Gas: 2, L1DataGas: 3, L2Gas: 4}},
			Traces:      []vm.TransactionTrace{vmTrace},
			NumSteps:    uint64(7),
		}, nil)

		trace, httpHeader, rpcErr := handler.TraceTransaction(t.Context(), hash)
		require.Nil(t, rpcErr)
		require.Equal(t, "7", httpHeader.Get(rpcv10.ExecutionStepsHeader))
		vmTrace.ExecutionResources = &vm.ExecutionResources{L1Gas: 2, L1DataGas: 3, L2Gas: 4}
		assert.Equal(t, rpcv10.AdaptVMTransactionTrace(&vmTrace), trace)
	})

	t.Run("reverted INVOKE tx from feeder", func(t *testing.T) {
		n := &networks.Sepolia

		handler := rpcv10.New(mockReader, mockSyncReader, mockVM, log.NewNopZapLogger())

		client := feeder.NewTestClient(t, n)
		handler.WithFeeder(client)

		// Tx at index 3 in the block
		revertedTxHash := felt.NewUnsafeFromString[felt.TransactionHash](
			"0x2f00c7f28df2197196440747f97baa63d0851e3b0cfc2efedb6a88a7ef78cb1",
		)

		blockNumber := uint64(18)

		gatewayBlock := adaptfeedertest.Block(t, client, blockNumber)
		revertedTxIndex := slices.IndexFunc(gatewayBlock.Transactions, func(tx core.Transaction) bool {
			return tx.Hash().Equal(revertedTxHash)
		})
		require.NotEqual(t, -1, revertedTxIndex)

		mockSyncReader.EXPECT().ReadPreConfirmedChain().Return(emptyPreConfirmedChain(t), nil)
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(revertedTxHash).
			Return(blockNumber, uint64(revertedTxIndex), nil)
		mockReader.EXPECT().BlockHeaderByNumber(blockNumber).
			DoAndReturn(func(number uint64) (*core.Header, error) {
				block := adaptfeedertest.Block(t, client, number)
				return block.Header, nil
			})
		mockReader.EXPECT().TransactionsAndReceiptsByBlockNumber(blockNumber).
			DoAndReturn(func(number uint64) ([]core.Transaction, []*core.TransactionReceipt, error) {
				block := adaptfeedertest.Block(t, client, number)
				return block.Transactions, block.Receipts, nil
			}).AnyTimes()

		expectedRevertedTrace := readTestData[rpcv10.TransactionTrace](
			t,
			"traces/reverted_invoke_trace.json",
		)

		trace, httpHeader, err := handler.TraceTransaction(t.Context(), revertedTxHash)

		require.Nil(t, err)
		assert.Equal(t, "0", httpHeader.Get(rpcv10.ExecutionStepsHeader))
		assert.Equal(t, expectedRevertedTrace, trace)
	})
}

func TestTraceBlockTransactionsWithMockedReader(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	n := &networks.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	logger := log.NewNopZapLogger()

	handler := rpcv10.New(mockReader, mockSyncReader, mockVM, logger)

	t.Run("regular block", func(t *testing.T) {
		blockHash := felt.NewUnsafeFromString[felt.Felt](
			"0x37b244ea7dc6b3f9735fba02d183ef0d6807a572dd91a63cc1b14b923c1ac0",
		)
		tx := &core.DeclareTransaction{
			TransactionHash: felt.NewUnsafeFromString[felt.TransactionHash]("0x000000001"),
			ClassHash:       felt.NewUnsafeFromString[felt.ClassHash]("0x000000000"),
		}

		header := &core.Header{
			Hash:             blockHash,
			ParentHash:       felt.NewUnsafeFromString[felt.Felt]("0x0"),
			Number:           0,
			SequencerAddress: felt.NewUnsafeFromString[felt.Address]("0X111"),
			L1GasPriceETH:    felt.NewUnsafeFromString[felt.Felt]("0x777"),
			ProtocolVersion:  "99.12.3",
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{tx},
		}
		declaredClass := &core.DeclaredClassDefinition{
			At:    3002,
			Class: &core.SierraClass{},
		}

		mockReader.EXPECT().BlockHeaderByHash(blockHash).Return(header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(header.Number).
			Return(block.Transactions, nil)

		mockReader.EXPECT().StateAtBlockHash(header.ParentHash).Return(nil, nopCloser, nil)
		headState := mocks.NewMockStateReader(mockCtrl)
		headState.EXPECT().Class(tx.ClassHash).Return(declaredClass, nil)
		mockReader.EXPECT().HeadState().Return(headState, nopCloser, nil)

		vmTrace := readTestData[vm.TransactionTrace](t, "traces/vm_block_trace.json")

		stepsUsed := uint64(123)
		stepsUsedStr := "123"

		mockVM.EXPECT().Trace(
			[]core.Transaction{tx},
			[]core.ClassDefinition{declaredClass.Class},
			[]*felt.Felt{},
			&vm.BlockInfo{Header: header},
			gomock.Any(),
			vm.TraceOptions{}).
			Return(vm.ExecutionResults{
				OverallFees:      nil,
				DataAvailability: []core.DataAvailability{{}, {}},
				GasConsumed:      []core.GasConsumed{{}},
				Traces:           []vm.TransactionTrace{vmTrace},
				NumSteps:         stepsUsed,
			}, nil)

		expectedTrace := rpcv10.AdaptVMTransactionTrace(&vmTrace)
		expectedResult := []rpcv10.TracedBlockTransaction{
			{
				TransactionHash: tx.Hash(),
				TraceRoot:       &expectedTrace,
			},
		}

		blockID := rpcv10.BlockIDFromHash(blockHash)
		result, httpHeader, rpcErr := handler.TraceBlockTransactions(t.Context(), &blockID, nil)
		require.Nil(t, rpcErr)
		assert.Equal(t, httpHeader.Get(rpcv10.ExecutionStepsHeader), stepsUsedStr)
		assert.Equal(t, expectedResult, result.Traces)
		assert.Nil(t, result.InitialReads)
	})
}

func TestAdaptVMTransactionTrace(t *testing.T) {
	t.Run("successfully adapt INVOKE trace from vm", func(t *testing.T) {
		fromAddr := felt.NewUnsafeFromString[felt.Address](
			"0x4c5772d1914fe6ce891b64eb35bf3522aeae1315647314aac58b01137607f3f",
		)
		toAddr := felt.NewUnsafeFromString[felt.Address](
			"0x540552aae708306346466633036396334303062342d24292eadbdc777db86e5",
		)

		payload0 := felt.Zero[felt.Felt]()
		payload1 := felt.UnsafeFromString[felt.Felt]("0x5ba586f822ce9debae27fa04a3e71721fdc90ff")
		payload2 := felt.FromUint64[felt.Felt](0x455448)
		payload3 := felt.FromUint64[felt.Felt](0x31da07977d000)
		payload4 := felt.Zero[felt.Felt]()

		vmTrace := vm.TransactionTrace{
			Type: vm.TxnInvoke,
			ValidateInvocation: &vm.FunctionInvocation{
				Messages: []vm.OrderedL2toL1Message{
					{
						Order: 0,
						From:  fromAddr,
						To:    toAddr,
						Payload: []felt.Felt{
							payload0,
							payload1,
							payload2,
							payload3,
							payload4,
						},
					},
				},
				ExecutionResources: &vm.ExecutionResources{
					L1Gas:     1,
					L1DataGas: 2,
					L2Gas:     3,
					ComputationResources: vm.ComputationResources{
						Steps:        1,
						MemoryHoles:  2,
						Pedersen:     3,
						RangeCheck:   4,
						Bitwise:      5,
						Ecdsa:        6,
						EcOp:         7,
						Keccak:       8,
						Poseidon:     9,
						SegmentArena: 10,
						AddMod:       11,
						MulMod:       12,
						RangeCheck96: 13,
						Output:       14,
					},
					DataAvailability: &vm.DataAvailability{
						L1Gas:     1,
						L1DataGas: 2,
					},
				},
			},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation:    &vm.ExecuteInvocation{},
			StateDiff: &vm.StateDiff{ //nolint:dupl // false positive with rpcv10.StateDiff
				StorageDiffs: []vm.StorageDiff{
					{
						Address: felt.Zero[felt.Address](),
						StorageEntries: []vm.Entry{
							{
								Key:   felt.Zero[felt.Felt](),
								Value: felt.Zero[felt.Felt](),
							},
						},
					},
				},
				Nonces: []vm.Nonce{
					{
						ContractAddress: felt.Zero[felt.Address](),
						Nonce:           felt.Zero[felt.Felt](),
					},
				},
				DeployedContracts: []vm.DeployedContract{
					{
						Address:   felt.Zero[felt.Address](),
						ClassHash: felt.Zero[felt.ClassHash](),
					},
				},
				DeprecatedDeclaredClasses: []*felt.ClassHash{
					new(felt.Zero[felt.ClassHash]()),
				},
				DeclaredClasses: []vm.DeclaredClass{
					{
						ClassHash:         felt.Zero[felt.SierraClassHash](),
						CompiledClassHash: felt.Zero[felt.CasmClassHash](),
					},
				},
				ReplacedClasses: []vm.ReplacedClass{
					{
						ContractAddress: felt.Zero[felt.Address](),
						ClassHash:       felt.Zero[felt.ClassHash](),
					},
				},
				MigratedCompiledClasses: []vm.MigratedCompiledClass{
					{
						ClassHash:         felt.Zero[felt.SierraClassHash](),
						CompiledClassHash: felt.Zero[felt.CasmClassHash](),
					},
				},
			},
		}

		expectedAdaptedTrace := rpcv10.TransactionTrace{
			Type: rpcv10.TxnInvoke,
			ValidateInvocation: &rpcv10.FunctionInvocation{
				Calls:  []rpcv10.FunctionInvocation{},
				Events: []rpcv10.OrderedEvent{},
				Messages: []rpcv10.OrderedL2toL1Message{
					{
						Order: 0,
						From:  fromAddr,
						// todo(rdr): we shouldn't need this conversion but the right fix is
						//            refactor which is a whole stream of work on itself
						To: (*felt.Felt)(toAddr),
						Payload: []felt.Felt{
							payload0,
							payload1,
							payload2,
							payload3,
							payload4,
						},
					},
				},
				ExecutionResources: &rpcv10.InnerExecutionResources{
					L1Gas: 1,
					L2Gas: 3,
				},
				IsReverted: false,
			},
			FeeTransferInvocation: &rpcv10.FunctionInvocation{
				Calls:      []rpcv10.FunctionInvocation{},
				Events:     []rpcv10.OrderedEvent{},
				Messages:   []rpcv10.OrderedL2toL1Message{},
				IsReverted: false,
			},
			ExecuteInvocation: &rpcv10.ExecuteInvocation{
				RevertReason: "",
				FunctionInvocation: &rpcv10.FunctionInvocation{
					Calls:      []rpcv10.FunctionInvocation{},
					Events:     []rpcv10.OrderedEvent{},
					Messages:   []rpcv10.OrderedL2toL1Message{},
					IsReverted: false,
				},
			},
			StateDiff: &rpcv10.StateDiff{ //nolint:dupl // false positive with vm.StateDiff
				StorageDiffs: []rpcv10.StorageDiff{
					{
						Address: felt.Zero[felt.Address](),
						StorageEntries: []rpcv10.Entry{
							{
								Key:   felt.Zero[felt.Felt](),
								Value: felt.Zero[felt.Felt](),
							},
						},
					},
				},
				Nonces: []rpcv10.Nonce{
					{
						ContractAddress: felt.Zero[felt.Address](),
						Nonce:           felt.Zero[felt.Felt](),
					},
				},
				DeployedContracts: []rpcv10.DeployedContract{
					{
						Address:   felt.Zero[felt.Address](),
						ClassHash: felt.Zero[felt.ClassHash](),
					},
				},
				DeprecatedDeclaredClasses: []*felt.ClassHash{
					new(felt.Zero[felt.ClassHash]()),
				},
				DeclaredClasses: []rpcv10.DeclaredClass{
					{
						ClassHash:         felt.Zero[felt.SierraClassHash](),
						CompiledClassHash: felt.Zero[felt.CasmClassHash](),
					},
				},
				ReplacedClasses: []rpcv10.ReplacedClass{
					{
						ContractAddress: felt.Zero[felt.Address](),
						ClassHash:       felt.Zero[felt.ClassHash](),
					},
				},
				MigratedCompiledClasses: []rpcv10.MigratedCompiledClass{
					{
						ClassHash:         felt.Zero[felt.SierraClassHash](),
						CompiledClassHash: felt.Zero[felt.CasmClassHash](),
					},
				},
			},
		}

		assert.Equal(t, expectedAdaptedTrace, rpcv10.AdaptVMTransactionTrace(&vmTrace))
	})

	t.Run("successfully adapt DEPLOY_ACCOUNT tx from vm", func(t *testing.T) {
		vmTrace := vm.TransactionTrace{
			Type:                  vm.TxnDeployAccount,
			ValidateInvocation:    &vm.FunctionInvocation{},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation:    &vm.ExecuteInvocation{},
		}

		expectedAdaptedTrace := rpcv10.TransactionTrace{
			Type: rpcv10.TxnDeployAccount,
			ValidateInvocation: &rpcv10.FunctionInvocation{
				Calls:    []rpcv10.FunctionInvocation{},
				Events:   []rpcv10.OrderedEvent{},
				Messages: []rpcv10.OrderedL2toL1Message{},
			},
			FeeTransferInvocation: &rpcv10.FunctionInvocation{
				Calls:    []rpcv10.FunctionInvocation{},
				Events:   []rpcv10.OrderedEvent{},
				Messages: []rpcv10.OrderedL2toL1Message{},
			},
			ConstructorInvocation: &rpcv10.FunctionInvocation{
				Calls:    []rpcv10.FunctionInvocation{},
				Events:   []rpcv10.OrderedEvent{},
				Messages: []rpcv10.OrderedL2toL1Message{},
			},
		}

		adaptedTrace := rpcv10.AdaptVMTransactionTrace(&vmTrace)

		require.Equal(t, expectedAdaptedTrace, adaptedTrace)
	})

	t.Run("successfully adapt L1_HANDLER tx from vm", func(t *testing.T) {
		vmTrace := vm.TransactionTrace{
			Type:                  vm.TxnL1Handler,
			ValidateInvocation:    &vm.FunctionInvocation{},
			FeeTransferInvocation: &vm.FunctionInvocation{},
			ExecuteInvocation: &vm.ExecuteInvocation{
				RevertReason:       "",
				FunctionInvocation: &vm.FunctionInvocation{},
			},
			ConstructorInvocation: &vm.FunctionInvocation{},
			FunctionInvocation: &vm.ExecuteInvocation{
				FunctionInvocation: &vm.FunctionInvocation{},
			},
		}

		expectedAdaptedTrace := rpcv10.TransactionTrace{
			Type: rpcv10.TxnL1Handler,
			FunctionInvocation: &rpcv10.ExecuteInvocation{
				RevertReason: "",
				FunctionInvocation: &rpcv10.FunctionInvocation{
					Calls:      []rpcv10.FunctionInvocation{},
					Events:     []rpcv10.OrderedEvent{},
					Messages:   []rpcv10.OrderedL2toL1Message{},
					IsReverted: false,
				},
			},
		}

		adaptedTrace := rpcv10.AdaptVMTransactionTrace(&vmTrace)

		require.Equal(t, expectedAdaptedTrace, adaptedTrace)
	})
}

func TestAdaptFeederBlockTrace(t *testing.T) {
	t.Run("nil block trace", func(t *testing.T) {
		block := &core.Block{}

		res, err := rpcv10.AdaptFeederBlockTrace(block.Transactions, nil)
		require.Nil(t, res)
		require.Nil(t, err)
	})

	t.Run("inconsistent block and blockTrace", func(t *testing.T) {
		block := &core.Block{
			Transactions: []core.Transaction{&core.InvokeTransaction{}},
		}
		blockTrace := &starknet.BlockTrace{}

		res, err := rpcv10.AdaptFeederBlockTrace(block.Transactions, blockTrace)
		require.Nil(t, res)
		require.Equal(t, errors.New("mismatched number of txs and traces"), err)
	})

	t.Run("L1_HANDLER tx gets successfully adapted", func(t *testing.T) {
		block := &core.Block{
			Transactions: []core.Transaction{&core.L1HandlerTransaction{}},
		}
		blockTrace := &starknet.BlockTrace{
			Traces: []starknet.TransactionTrace{
				{
					TransactionHash:       felt.FromUint64[felt.TransactionHash](1),
					FeeTransferInvocation: &starknet.FunctionInvocation{},
					ValidateInvocation:    &starknet.FunctionInvocation{},
					FunctionInvocation: &starknet.FunctionInvocation{
						Events: []starknet.OrderedEvent{{
							Order: 1,
							Keys:  []felt.Felt{felt.FromUint64[felt.Felt](2)},
							Data:  []felt.Felt{felt.FromUint64[felt.Felt](3)},
						}},
						ExecutionResources: starknet.ExecutionResources{
							TotalGasConsumed: &starknet.GasConsumed{
								L1Gas:     10,
								L2Gas:     11,
								L1DataGas: 12,
							},
						},
					},
				},
			},
		}

		expectedAdaptedTrace := []rpcv10.TracedBlockTransaction{
			{
				TransactionHash: felt.NewFromUint64[felt.TransactionHash](1),
				TraceRoot: &rpcv10.TransactionTrace{
					Type: rpcv10.TxnL1Handler,
					FunctionInvocation: &rpcv10.ExecuteInvocation{
						RevertReason: "",
						FunctionInvocation: &rpcv10.FunctionInvocation{
							Calls: []rpcv10.FunctionInvocation{},
							Events: []rpcv10.OrderedEvent{{
								Order: 1,
								Keys:  []felt.Felt{felt.FromUint64[felt.Felt](2)},
								Data:  []felt.Felt{felt.FromUint64[felt.Felt](3)},
							}},
							Messages: []rpcv10.OrderedL2toL1Message{},
							ExecutionResources: &rpcv10.InnerExecutionResources{
								L1Gas: 10,
								L2Gas: 11,
							},
						},
					},
				},
			},
		}

		res, err := rpcv10.AdaptFeederBlockTrace(block.Transactions, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})

	t.Run("INVOKE tx gets successfully adapted (with revert error)", func(t *testing.T) {
		block := &core.Block{
			Transactions: []core.Transaction{&core.InvokeTransaction{}},
		}
		blockTrace := &starknet.BlockTrace{
			Traces: []starknet.TransactionTrace{
				{
					TransactionHash:       felt.FromUint64[felt.TransactionHash](1),
					FeeTransferInvocation: &starknet.FunctionInvocation{},
					ValidateInvocation:    &starknet.FunctionInvocation{},
					// When revert error, feeder trace has no FunctionInvocation only RevertError is set
					RevertError: "some error",
				},
			},
		}

		expectedAdaptedTrace := []rpcv10.TracedBlockTransaction{
			{
				TransactionHash: felt.NewFromUint64[felt.TransactionHash](1),
				TraceRoot: &rpcv10.TransactionTrace{
					Type: rpcv10.TxnInvoke,
					FeeTransferInvocation: &rpcv10.FunctionInvocation{
						Calls:              []rpcv10.FunctionInvocation{},
						Events:             []rpcv10.OrderedEvent{},
						Messages:           []rpcv10.OrderedL2toL1Message{},
						ExecutionResources: &rpcv10.InnerExecutionResources{},
					},
					ValidateInvocation: &rpcv10.FunctionInvocation{
						Calls:              []rpcv10.FunctionInvocation{},
						Events:             []rpcv10.OrderedEvent{},
						Messages:           []rpcv10.OrderedL2toL1Message{},
						ExecutionResources: &rpcv10.InnerExecutionResources{},
					},
					ExecuteInvocation: &rpcv10.ExecuteInvocation{
						RevertReason: "some error",
					},
				},
			},
		}

		res, err := rpcv10.AdaptFeederBlockTrace(block.Transactions, blockTrace)
		require.Nil(t, err)
		require.Equal(t, expectedAdaptedTrace, res)
	})
}

func TestCallWithMockedVM(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpcv10.New(mockReader, mockSyncReader, mockVM, log.NewNopZapLogger())

	mockState := mocks.NewMockStateReader(mockCtrl)

	t.Run("execution failed with execution failure and empty result", func(t *testing.T) {
		handler = handler.WithCallMaxSteps(1337).WithCallMaxGas(1338)

		contractAddr := felt.NewFromUint64[felt.Address](1)
		selector := felt.NewFromUint64[felt.Felt](2)
		classHash := felt.NewFromUint64[felt.ClassHash](3)
		calldata := []felt.Felt{felt.FromUint64[felt.Felt](4)}
		expectedRes := vm.CallResult{
			ExecutionFailed: true,
		}
		expectedErr := rpcv10.MakeContractError(json.RawMessage(""))

		headsHeader := &core.Header{
			Number:    9,
			Timestamp: 101,
		}

		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil)
		mockState.EXPECT().ContractClassHash(contractAddr).Return(*classHash, nil)
		mockVM.EXPECT().Call(
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
			gomock.Any(),
		).Return(expectedRes, nil)

		blockID := rpcv10.BlockIDLatest()
		res, rpcErr := handler.Call(
			&rpcv10.FunctionCall{
				ContractAddress:    *contractAddr,
				EntryPointSelector: *selector,
				Calldata:           rpcv10.CalldataInputs{Data: calldata},
			},
			&blockID,
		)
		require.Nil(t, res)
		require.Equal(t, expectedErr, rpcErr)
	})
}

// TestTraceBlockTransactionsInitialReadsCacheCoherence verifies that the
// block-trace cache does not poison RETURN_INITIAL_READS responses. The cache
// key is the block hash only, so a prior call without the flag used to cause
// subsequent calls with the flag to return empty initial reads.
func TestTraceBlockTransactionsInitialReadsCacheCoherence(t *testing.T) {
	t.Parallel()
	n := &networks.Mainnet

	addr := felt.FromUint64[felt.Address](123)
	storageKey := felt.FromUint64[felt.Felt](456)
	storageValue := felt.FromUint64[felt.Felt](789)

	populatedVMReads := func() *vm.InitialReads {
		return &vm.InitialReads{
			Storage: []vm.InitialReadsStorageEntry{
				{ContractAddress: addr, Key: storageKey, Value: storageValue},
			},
			Nonces:            []vm.InitialReadsNonceEntry{},
			ClassHashes:       []vm.InitialReadsClassHashEntry{},
			DeclaredContracts: []vm.InitialReadsDeclaredContractEntry{},
		}
	}
	expectedPopulatedReads := &rpcv10.InitialReads{
		Storage: []rpcv10.StorageEntry{
			{ContractAddress: addr, Key: storageKey, Value: storageValue},
		},
		Nonces:            []rpcv10.NonceEntry{},
		ClassHashes:       []rpcv10.ClassHashEntry{},
		DeclaredContracts: []rpcv10.DeclaredContractEntry{},
	}

	buildBlock := func() (*core.Block, *felt.Felt, *felt.TransactionHash, *core.Header) {
		blockHash := felt.FromUint64[felt.Felt](999)
		parentHash := felt.FromUint64[felt.Felt](998)
		revealedHash := felt.FromUint64[felt.Felt](90)
		txHash := felt.FromUint64[felt.TransactionHash](888)
		header := &core.Header{
			SequencerAddress: n.BlockHashMetaInfo.FallBackSequencerAddress,
			L1GasPriceETH:    new(felt.Zero[felt.Felt]()),
			L1GasPriceSTRK:   new(felt.Zero[felt.Felt]()),
			L1DAMode:         0,
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: new(felt.Zero[felt.Felt]()),
				PriceInFri: new(felt.Zero[felt.Felt]()),
			},
			L2GasPrice: &core.GasPrice{
				PriceInWei: new(felt.Zero[felt.Felt]()),
				PriceInFri: new(felt.Zero[felt.Felt]()),
			},
			Number:          100,
			Hash:            &blockHash,
			ParentHash:      &parentHash,
			ProtocolVersion: "0.13.2",
		}
		block := &core.Block{
			Header:       header,
			Transactions: []core.Transaction{&core.InvokeTransaction{TransactionHash: &txHash}},
		}
		return block, &blockHash, &txHash, &core.Header{Hash: &revealedHash}
	}

	execResultWithReads := func(reads *vm.InitialReads) vm.ExecutionResults {
		return vm.ExecutionResults{
			OverallFees:      []*felt.Felt{new(felt.Zero[felt.Felt]())},
			DataAvailability: []core.DataAvailability{{L1Gas: 0}},
			GasConsumed:      []core.GasConsumed{{L1Gas: 0, L1DataGas: 0, L2Gas: 0}},
			Traces:           []vm.TransactionTrace{{StateDiff: &vm.StateDiff{}}},
			NumSteps:         100,
			InitialReads:     reads,
		}
	}

	// After TraceTransaction has cached the block without initial reads,
	// a follow-up TraceBlockTransactions with RETURN_INITIAL_READS must
	// re-execute the VM and return populated reads — not the empty struct
	// that was served from the poisoned cache.
	t.Run("TraceTransaction does not poison RETURN_INITIAL_READS", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockReader := mocks.NewMockReader(mockCtrl)
		mockVM := mocks.NewMockVM(mockCtrl)
		mockState := mocks.NewMockStateReader(mockCtrl)
		block, blockHash, txHash, revealedHeader := buildBlock()

		mockReader.EXPECT().Network().Return(n).AnyTimes()
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()
		mockReader.EXPECT().
			BlockHeaderHashByNumber(uint64(90)).
			Return(revealedHeader.Hash, nil).
			AnyTimes()
		mockReader.EXPECT().BlockNumberAndIndexByTxHash(txHash).
			Return(block.Number, uint64(0), nil)
		mockReader.EXPECT().BlockHeaderByHash(blockHash).Return(block.Header, nil)
		mockReader.EXPECT().BlockHeaderByNumber(block.Number).Return(block.Header, nil)
		mockReader.EXPECT().TransactionsByBlockNumber(block.Number).
			Return(block.Transactions, nil).Times(2)
		mockReader.EXPECT().StateAtBlockHash(block.ParentHash).Return(mockState, nopCloser, nil).Times(2)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).Times(2)

		// First VM call: no initial reads requested.
		gomock.InOrder(
			mockVM.EXPECT().Trace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), mockState,
				vm.TraceOptions{},
			).Return(execResultWithReads(nil), nil),
			// Second VM call: flag set, VM produces populated reads.
			mockVM.EXPECT().Trace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), mockState,
				vm.TraceOptions{ReturnInitialReads: true},
			).Return(execResultWithReads(populatedVMReads()), nil),
		)

		mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
		mockSyncReader.EXPECT().ReadPreConfirmedChain().Return(emptyPreConfirmedChain(t), nil)
		handler := rpcv10.New(mockReader, mockSyncReader, mockVM, log.NewNopZapLogger())

		_, _, err := handler.TraceTransaction(t.Context(), txHash)
		require.Nil(t, err)

		blockID := rpcv10.BlockIDFromHash(blockHash)
		result, _, err := handler.TraceBlockTransactions(
			t.Context(), &blockID,
			[]rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag},
		)
		require.Nil(t, err)
		require.Equal(t, expectedPopulatedReads, result.InitialReads)
	})

	// With the flag first, the cached entry already has initial reads, so a
	// repeat call must be served from cache (VM invoked exactly once).
	t.Run("cache reused when initial reads already cached", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockReader := mocks.NewMockReader(mockCtrl)
		mockVM := mocks.NewMockVM(mockCtrl)
		mockState := mocks.NewMockStateReader(mockCtrl)
		block, blockHash, _, revealedHeader := buildBlock()

		mockReader.EXPECT().Network().Return(n).AnyTimes()
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()
		mockReader.EXPECT().
			BlockHeaderHashByNumber(uint64(90)).
			Return(revealedHeader.Hash, nil).
			AnyTimes()
		mockReader.EXPECT().BlockHeaderByHash(blockHash).Return(block.Header, nil).Times(2)
		// The cached follow-up serves from the header alone, so transactions are read once.
		mockReader.EXPECT().TransactionsByBlockNumber(block.Number).
			Return(block.Transactions, nil)
		mockReader.EXPECT().StateAtBlockHash(block.ParentHash).Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)

		mockVM.EXPECT().Trace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), mockState,
			vm.TraceOptions{ReturnInitialReads: true},
		).Return(execResultWithReads(populatedVMReads()), nil)

		handler := rpcv10.New(mockReader, nil, mockVM, log.NewNopZapLogger())

		blockID := rpcv10.BlockIDFromHash(blockHash)
		for range 2 {
			result, _, err := handler.TraceBlockTransactions(
				t.Context(), &blockID,
				[]rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag},
			)
			require.Nil(t, err)
			require.Equal(t, expectedPopulatedReads, result.InitialReads)
		}
	})

	// With the flag first, then without: the second call must be served from
	// cache and strip initial reads from the response.
	t.Run("cache reused for flag-less follow-up", func(t *testing.T) {
		t.Parallel()
		mockCtrl := gomock.NewController(t)
		t.Cleanup(mockCtrl.Finish)

		mockReader := mocks.NewMockReader(mockCtrl)
		mockVM := mocks.NewMockVM(mockCtrl)
		mockState := mocks.NewMockStateReader(mockCtrl)
		block, blockHash, _, revealedHeader := buildBlock()

		mockReader.EXPECT().Network().Return(n).AnyTimes()
		mockReader.EXPECT().L1Head().Return(core.L1Head{}, db.ErrKeyNotFound).AnyTimes()
		mockReader.EXPECT().
			BlockHeaderHashByNumber(uint64(90)).
			Return(revealedHeader.Hash, nil).
			AnyTimes()
		mockReader.EXPECT().BlockHeaderByHash(blockHash).Return(block.Header, nil).Times(2)
		// The cached follow-up serves from the header alone, so transactions are read once.
		mockReader.EXPECT().TransactionsByBlockNumber(block.Number).
			Return(block.Transactions, nil)
		mockReader.EXPECT().StateAtBlockHash(block.ParentHash).Return(mockState, nopCloser, nil)
		mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)

		mockVM.EXPECT().Trace(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), mockState,
			vm.TraceOptions{ReturnInitialReads: true},
		).Return(execResultWithReads(populatedVMReads()), nil)

		handler := rpcv10.New(mockReader, nil, mockVM, log.NewNopZapLogger())

		blockID := rpcv10.BlockIDFromHash(blockHash)
		first, _, err := handler.TraceBlockTransactions(
			t.Context(), &blockID,
			[]rpcv10.TraceFlag{rpcv10.TraceReturnInitialReadsFlag},
		)
		require.Nil(t, err)
		require.Equal(t, expectedPopulatedReads, first.InitialReads)

		second, _, err := handler.TraceBlockTransactions(t.Context(), &blockID, nil)
		require.Nil(t, err)
		require.Nil(t, second.InitialReads)
	})
}
