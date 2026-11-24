package rpcv9_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpctest"
	"github.com/stretchr/testify/require"
)

const rpcVersion = rpctest.Version0_9

var strk = networks.DefaultFeeTokenAddresses.StrkL2TokenAddress

func selector(name string) string {
	s := crypto.StarknetKeccak([]byte(name))
	return s.String()
}

func callParams(contract *felt.Address, function, blockID string, calldata ...string) string {
	if calldata == nil {
		calldata = []string{}
	}
	data, err := json.Marshal(calldata)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(
		`{"request":{"contract_address":%q,"entry_point_selector":%q,"calldata":%s},"block_id":%s}`,
		contract.String(), selector(function), data, blockID,
	)
}

func hexToBig(t *testing.T, hex string) *big.Int {
	t.Helper()
	value, ok := new(big.Int).SetString(hex, 0)
	require.True(t, ok, hex)
	return value
}

// strkBalance returns the STRK balance of the owner from starknet_call balanceOf.
func strkBalance(t *testing.T, node *rpctest.Node, blockID string, owner *felt.Address) *big.Int {
	t.Helper()
	var words []string
	result := node.Result(t, rpcVersion, "starknet_call",
		callParams(&strk, "balanceOf", blockID, owner.String()))
	require.NoError(t, json.Unmarshal(result, &words))
	require.Len(t, words, 2) // u256: low, high
	high := hexToBig(t, words[1])
	return high.Lsh(high, 128).Add(high, hexToBig(t, words[0]))
}

type receipt struct {
	ExecutionStatus string `json:"execution_status"`
	RevertReason    string `json:"revert_reason"`
	ActualFee       struct {
		Amount string `json:"amount"`
		Unit   string `json:"unit"`
	} `json:"actual_fee"`
}

func getReceipt(t *testing.T, node *rpctest.Node, hash *felt.TransactionHash) receipt {
	t.Helper()
	var r receipt
	result := node.Result(t, rpcVersion, "starknet_getTransactionReceipt",
		fmt.Sprintf(`{"transaction_hash":%q}`, hash))
	require.NoError(t, json.Unmarshal(result, &r))
	return r
}

type feeEstimate struct {
	L1GasConsumed     string `json:"l1_gas_consumed"`
	L1GasPrice        string `json:"l1_gas_price"`
	L2GasConsumed     string `json:"l2_gas_consumed"`
	L2GasPrice        string `json:"l2_gas_price"`
	L1DataGasConsumed string `json:"l1_data_gas_consumed"`
	L1DataGasPrice    string `json:"l1_data_gas_price"`
	OverallFee        string `json:"overall_fee"`
	Unit              string `json:"unit"`
}

// gasCost returns the sum of the consumed gas times its price, for each resource.
func (f *feeEstimate) gasCost(t *testing.T) *big.Int {
	t.Helper()
	cost := new(big.Int)
	for _, resource := range [][2]string{
		{f.L1GasConsumed, f.L1GasPrice},
		{f.L2GasConsumed, f.L2GasPrice},
		{f.L1DataGasConsumed, f.L1DataGasPrice},
	} {
		cost.Add(cost, new(big.Int).Mul(hexToBig(t, resource[0]), hexToBig(t, resource[1])))
	}
	return cost
}

func blockHash(t *testing.T, node *rpctest.Node, number uint64) string {
	t.Helper()
	var block struct {
		BlockHash string `json:"block_hash"`
	}
	result := node.Result(t, rpcVersion, "starknet_getBlockWithTxHashes",
		fmt.Sprintf(`{"block_id":{"block_number":%d}}`, number))
	require.NoError(t, json.Unmarshal(result, &block))
	return block.BlockHash
}

// executionError is the data of a TRANSACTION_EXECUTION_ERROR.
type executionError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		TransactionIndex int             `json:"transaction_index"`
		ExecutionError   json.RawMessage `json:"execution_error"`
	} `json:"data"`
}

// requireExecutionError checks that the error is a TRANSACTION_EXECUTION_ERROR of the
// transaction at the index, with the reason in its execution error.
func requireExecutionError(t *testing.T, rpcErr json.RawMessage, index int, reason string) {
	t.Helper()
	var got executionError
	require.NoError(t, json.Unmarshal(rpcErr, &got), string(rpcErr))
	require.Equal(t, 41, got.Code)
	require.Equal(t, "Transaction execution error", got.Message)
	require.Equal(t, index, got.Data.TransactionIndex)
	require.Contains(t, string(got.Data.ExecutionError), reason)
}

// sortedSets decodes the JSON and sorts each array in each state_diff and initial_reads. The
// spec does not fix the order of these arrays.
func sortedSets(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	var value any
	require.NoError(t, json.Unmarshal(raw, &value))
	sortSets(t, value, false)
	return value
}

func sortSets(t *testing.T, value any, inSet bool) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		for key, field := range value {
			sortSets(t, field, inSet || key == "state_diff" || key == "initial_reads")
		}
	case []any:
		for _, element := range value {
			sortSets(t, element, inSet)
		}
		if inSet {
			slices.SortFunc(value, func(a, b any) int {
				aJSON, err := json.Marshal(a)
				require.NoError(t, err)
				bJSON, err := json.Marshal(b)
				require.NoError(t, err)
				return slices.Compare(aJSON, bJSON)
			})
		}
	}
}
