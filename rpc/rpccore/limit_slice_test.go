package rpccore_test

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/stretchr/testify/require"
)

const (
	maxSize          = 1000000
	maxFeltSliceSize = 10
)

func TestLazySlice(t *testing.T) {
	t.Run("BroadcastedTransaction", func(t *testing.T) {
		runTest[rpcv9.BroadcastedTransactionInputs, rpccore.SimulationLimit](
			t,
			func(length int) []byte {
				return []byte(jsonArrayString("{}", length))
			},
			func(length int) rpcv9.BroadcastedTransactionInputs {
				return rpcv9.BroadcastedTransactionInputs{
					Data: make([]rpcv9.BroadcastedTransaction, length),
				}
			},
			func(transactions *rpcv9.BroadcastedTransactionInputs) {
				for i := range transactions.Data {
					transactions.Data[i] = randomBroadcastedTransaction(t)
				}
			},
		)
	})

	t.Run("FunctionCall", func(t *testing.T) {
		runTest[rpcv9.FunctionCall, rpccore.FunctionCalldataLimit](
			t,
			func(length int) []byte {
				return []byte(`{"calldata":` + jsonArrayString("0", length) + `}`)
			},
			func(length int) rpcv9.FunctionCall {
				return rpcv9.FunctionCall{
					Calldata: rpcv9.CalldataInputs{
						Data: make([]felt.Felt, length),
					},
				}
			},
			// Cannot test full struct for FunctionCall because its json encoding is not roundtrip
			nil,
		)
	})
}

func runTest[T any, L rpccore.Limit](
	t *testing.T,
	buildEmptyInput func(int) []byte,
	buildEmptyExpected func(int) T,
	populateFullStruct func(*T),
) {
	var limit L
	testCases := []struct {
		name               string
		length             int
		expected           bool
		skipFullStructTest bool
	}{
		{name: "Nil", length: 0, expected: true},
		{name: "1", length: 1, expected: true},
		{name: "2", length: 2, expected: true},
		{name: "Less than limit", length: limit.Limit() - 1, expected: true},
		{name: "Limit", length: limit.Limit(), expected: true},
		{name: "Longer than limit", length: limit.Limit() + 1, expected: false},
		{name: "Very long", length: maxSize, expected: false, skipFullStructTest: true},
	}

	for _, testCase := range testCases {
		var outcome string
		if testCase.expected {
			outcome = "pass"
		} else {
			outcome = "fail"
		}
		t.Run(fmt.Sprintf("%s should %s", testCase.name, outcome), func(t *testing.T) {
			t.Run("Empty struct", func(t *testing.T) {
				if testCase.expected {
					expected := buildEmptyExpected(testCase.length)
					assertPassed(t, buildEmptyInput(testCase.length), expected)
				} else {
					assertFailed[T](t, buildEmptyInput(testCase.length))
				}
			})

			if populateFullStruct != nil && !testCase.skipFullStructTest {
				t.Run("Full struct", func(t *testing.T) {
					expected := buildEmptyExpected(testCase.length)
					populateFullStruct(&expected)

					input, err := json.Marshal(expected)
					require.NoError(t, err)

					if testCase.expected {
						assertPassed(t, input, expected)
					} else {
						assertFailed[T](t, input)
					}
				})
			}
		})
	}
}

func assertPassed[T any](t *testing.T, data []byte, expected T) {
	t.Helper()
	var actual T
	err := json.Unmarshal(data, &actual)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func assertFailed[T any](t *testing.T, data []byte) {
	t.Helper()
	var actual T
	require.Error(t, json.Unmarshal(data, &actual))
}

func jsonArrayString(element string, length int) string {
	return `[` + strings.Join(slices.Repeat([]string{element}, length), ",") + `]`
}

func randomFelt(t *testing.T) *felt.Felt {
	t.Helper()
	randomFelt, err := felt.NewRandom[felt.Felt]()
	require.NoError(t, err)
	return randomFelt
}

func randomFeltSlice(t *testing.T) *[]*felt.Felt {
	t.Helper()
	length := rand.IntN(maxFeltSliceSize)
	randomFeltSlice := make([]*felt.Felt, length)
	for i := range length {
		randomFeltSlice[i] = randomFelt(t)
	}
	return &randomFeltSlice
}

func randomEnum[T any](values ...T) T {
	return values[rand.IntN(len(values))]
}

func randomBroadcastedTransaction(t *testing.T) rpcv9.BroadcastedTransaction {
	t.Helper()
	transactionType := randomEnum(
		rpcv9.TxnInvoke,
		rpcv9.TxnDeploy,
		rpcv9.TxnDeployAccount,
		rpcv9.TxnDeclare,
		rpcv9.TxnL1Handler,
	)
	feeDAMode := randomEnum(rpcv9.DAModeL1, rpcv9.DAModeL2)
	nonceDAMode := randomEnum(rpcv9.DAModeL1, rpcv9.DAModeL2)
	resourceBounds := rpcv9.ResourceBoundsMap{
		L1Gas: &rpcv9.ResourceBounds{
			MaxAmount:       randomFelt(t),
			MaxPricePerUnit: randomFelt(t),
		},
		L2Gas: &rpcv9.ResourceBounds{
			MaxAmount:       randomFelt(t),
			MaxPricePerUnit: randomFelt(t),
		},
		L1DataGas: &rpcv9.ResourceBounds{
			MaxAmount:       randomFelt(t),
			MaxPricePerUnit: randomFelt(t),
		},
	}
	return rpcv9.BroadcastedTransaction{
		Transaction: rpcv9.Transaction{
			Hash:                  randomFelt(t),
			Type:                  transactionType,
			Version:               randomFelt(t),
			Nonce:                 randomFelt(t),
			MaxFee:                randomFelt(t),
			ContractAddress:       randomFelt(t),
			ContractAddressSalt:   randomFelt(t),
			ClassHash:             randomFelt(t),
			ConstructorCallData:   randomFeltSlice(t),
			SenderAddress:         randomFelt(t),
			Signature:             randomFeltSlice(t),
			CallData:              randomFeltSlice(t),
			EntryPointSelector:    randomFelt(t),
			CompiledClassHash:     randomFelt(t),
			ResourceBounds:        &resourceBounds,
			Tip:                   randomFelt(t),
			PaymasterData:         randomFeltSlice(t),
			AccountDeploymentData: randomFeltSlice(t),
			NonceDAMode:           &nonceDAMode,
			FeeDAMode:             &feeDAMode,
		},
		ContractClass: json.RawMessage("[]"),
		PaidFeeOnL1:   randomFelt(t),
	}
}
