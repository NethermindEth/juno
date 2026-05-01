package rpccore_test

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils/jsonx"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

const (
	maxSize          = 1000000
	maxFeltSliceSize = 10
)

func TestLazySlice(t *testing.T) {
	t.Run("BroadcastedTransaction", func(t *testing.T) {
		runTest[rpcv10.BroadcastedTransactionInputs, rpccore.SimulationLimit](
			t,
			func(length int) []byte {
				return []byte(jsonArrayString("{}", length))
			},
			func(length int) rpcv10.BroadcastedTransactionInputs {
				return rpcv10.BroadcastedTransactionInputs{
					Data: make([]rpcv10.BroadcastedTransaction, length),
				}
			},
			func(transactions *rpcv10.BroadcastedTransactionInputs) {
				for i := range transactions.Data {
					transactions.Data[i] = randomBroadcastedTransaction(t)
				}
			},
		)
	})

	t.Run("FunctionCall", func(t *testing.T) {
		runTest[rpcv10.FunctionCall, rpccore.FunctionCalldataLimit](
			t,
			func(length int) []byte {
				return []byte(`{"calldata":` + jsonArrayString(`"0x0"`, length) + `}`)
			},
			func(length int) rpcv10.FunctionCall {
				return rpcv10.FunctionCall{
					Calldata: rpcv10.CalldataInputs{
						Data: make([]felt.Felt, length),
					},
				}
			},
			// Cannot test full struct for FunctionCall because its json encoding is not roundtrip
			nil,
		)
	})

	t.Run("FunctionCall rejects non-hex calldata", func(t *testing.T) {
		assertFailed[rpcv10.FunctionCall](t, []byte(`{"calldata":["123"]}`))
		assertFailed[rpcv10.FunctionCall](t, []byte(`{"calldata":["abcd"]}`))
		assertFailed[rpcv10.FunctionCall](t, []byte(`{"calldata":[123]}`))
		assertFailed[rpcv10.FunctionCall](t, []byte(`{"calldata":"0x1g"}`))
	})

	// This test ensures that the validation logic works for the values inside the Data slice.
	t.Run("ValidateRequiredFields", func(t *testing.T) {
		// Random type with validation tags
		type RandType struct {
			A int `validate:"required"`
			B int `validate:"gt=5"`
		}

		type RandLimitSlice = rpccore.LimitSlice[
			RandType,
			rpccore.SimulationLimit,
		]

		withEmptyValues := RandLimitSlice{
			Data: make([]RandType, 10),
		}

		validate := validator.New()
		err := validate.Struct(withEmptyValues)
		require.Error(t, err, "Validation is not working for the values inside the Data slice")
	})
}

// counted is a T whose UnmarshalJSON increments a package-level counter,
// so the test can observe how many elements were actually decoded into T.
type counted struct{ N int }

var countedDecodes atomic.Int64

func (c *counted) UnmarshalJSON(data []byte) error {
	countedDecodes.Add(1)
	return jsonx.Unmarshal(data, &c.N)
}

func TestLimitSliceLaziness(t *testing.T) {
	const (
		total      = 1_000_000 // total elements in the payload
		simulation = 5000      // value of SimulationLimit
	)

	var b strings.Builder
	b.WriteByte('[')
	for i := range total {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("0")
	}
	b.WriteByte(']')
	payload := []byte(b.String())

	type Slice = rpccore.LimitSlice[counted, rpccore.SimulationLimit]
	var s Slice

	countedDecodes.Store(0)
	start := time.Now()
	err := s.UnmarshalJSON(payload)
	elapsed := time.Since(start)
	decoded := countedDecodes.Load()

	require.ErrorContains(t, err, "expected max 5000 items")
	require.EqualValues(t, simulation, decoded,
		"T.UnmarshalJSON ran %d times; expected exactly cap=%d", decoded, simulation)

	t.Logf("payload=%d bytes, total elements=%d, cap=%d, decoded=%d, elapsed=%v",
		len(payload), total, simulation, decoded, elapsed)
}

type smallLimit struct{}

func (smallLimit) Limit() int { return 3 }

func TestLimitSliceEdgeCases(t *testing.T) {
	type IntSlice = rpccore.LimitSlice[int, smallLimit]
	type StrSlice = rpccore.LimitSlice[string, smallLimit]
	type NestedSlice = rpccore.LimitSlice[[]int, smallLimit]

	t.Run("whitespace tolerated", func(t *testing.T) {
		for _, in := range []string{
			"[1,2,3]",
			"[ 1 , 2 , 3 ]",
			"[\n1,\t2,\r3]",
			"[\n\t1\t,\n2\r,3\n]",
		} {
			var s IntSlice
			require.NoError(t, jsonx.Unmarshal([]byte(in), &s), in)
			require.Equal(t, []int{1, 2, 3}, s.Data, in)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		for _, in := range []string{"[]", "[ ]", "[\n\t ]"} {
			var s IntSlice
			require.NoError(t, jsonx.Unmarshal([]byte(in), &s), in)
			require.Equal(t, []int{}, s.Data, in)
		}
	})

	t.Run("trailing comma rejected", func(t *testing.T) {
		for _, in := range []string{"[1,]", "[1,2,]", "[ 1 , ]"} {
			var s IntSlice
			require.Error(t, jsonx.Unmarshal([]byte(in), &s), in)
		}
	})

	t.Run("missing closing bracket", func(t *testing.T) {
		for _, in := range []string{"[", "[1", "[1,", "[1,2"} {
			var s IntSlice
			require.Error(t, jsonx.Unmarshal([]byte(in), &s), in)
		}
	})

	t.Run("not an array", func(t *testing.T) {
		for _, in := range []string{"null", "42", `"x"`, "{}", "true"} {
			var s IntSlice
			require.Error(t, jsonx.Unmarshal([]byte(in), &s), in)
		}
	})

	t.Run("empty or whitespace-only input", func(t *testing.T) {
		for _, in := range []string{"", " ", "\n\t\r"} {
			var s IntSlice
			require.Error(t, jsonx.Unmarshal([]byte(in), &s), in)
		}
	})

	t.Run("invalid separator between values", func(t *testing.T) {
		for _, in := range []string{"[1 2]", "[1;2]", "[1:2]"} {
			var s IntSlice
			require.Error(t, jsonx.Unmarshal([]byte(in), &s), in)
		}
	})

	t.Run("nested arrays counted as one element each", func(t *testing.T) {
		var s NestedSlice
		require.NoError(t, jsonx.Unmarshal([]byte("[[1,2],[3],[]]"), &s))
		require.Equal(t, [][]int{{1, 2}, {3}, {}}, s.Data)
	})

	t.Run("strings containing structural characters", func(t *testing.T) {
		var s StrSlice
		require.NoError(t, jsonx.Unmarshal([]byte(`["a,b","c]d","e[f"]`), &s))
		require.Equal(t, []string{"a,b", "c]d", "e[f"}, s.Data)
	})

	t.Run("escaped characters inside strings", func(t *testing.T) {
		var s StrSlice
		require.NoError(t, jsonx.Unmarshal([]byte(`["a\"b","c\\","\""]`), &s))
		require.Equal(t, []string{`a"b`, `c\`, `"`}, s.Data)
	})

	t.Run("element parse error surfaces", func(t *testing.T) {
		var s IntSlice
		err := jsonx.Unmarshal([]byte(`[1,"x",3]`), &s)
		require.Error(t, err)
	})

	t.Run("at limit boundary succeeds", func(t *testing.T) {
		var s IntSlice
		require.NoError(t, jsonx.Unmarshal([]byte("[1,2,3]"), &s))
		require.Len(t, s.Data, 3)
	})

	t.Run("one over limit fails", func(t *testing.T) {
		var s IntSlice
		err := jsonx.Unmarshal([]byte("[1,2,3,4]"), &s)
		require.ErrorContains(t, err, "expected max 3 items")
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

					input, err := jsonx.Marshal(expected)
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
	err := jsonx.Unmarshal(data, &actual)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func assertFailed[T any](t *testing.T, data []byte) {
	t.Helper()
	var actual T
	require.Error(t, jsonx.Unmarshal(data, &actual))
}

func jsonArrayString(element string, length int) string {
	return `[` + strings.Join(slices.Repeat([]string{element}, length), ",") + `]`
}

func randomFeltSlice(t *testing.T) *[]*felt.Felt {
	t.Helper()
	length := rand.IntN(maxFeltSliceSize)
	randomFeltSlice := make([]*felt.Felt, length)
	for i := range length {
		randomFeltSlice[i] = felt.NewRandom[felt.Felt]()
	}
	return &randomFeltSlice
}

func randomEnum[T any](values ...T) T {
	return values[rand.IntN(len(values))]
}

func randomBroadcastedTransaction(t *testing.T) rpcv10.BroadcastedTransaction {
	t.Helper()
	transactionType := randomEnum(
		rpcv10.TxnInvoke,
		rpcv10.TxnDeploy,
		rpcv10.TxnDeployAccount,
		rpcv10.TxnDeclare,
		rpcv10.TxnL1Handler,
	)
	feeDAMode := randomEnum(rpcv10.DAModeL1, rpcv10.DAModeL2)
	nonceDAMode := randomEnum(rpcv10.DAModeL1, rpcv10.DAModeL2)
	resourceBounds := rpcv10.ResourceBoundsMap{
		L1Gas: &rpcv10.ResourceBounds{
			MaxAmount:       felt.NewRandom[felt.Felt](),
			MaxPricePerUnit: felt.NewRandom[felt.Felt](),
		},
		L2Gas: &rpcv10.ResourceBounds{
			MaxAmount:       felt.NewRandom[felt.Felt](),
			MaxPricePerUnit: felt.NewRandom[felt.Felt](),
		},
		L1DataGas: &rpcv10.ResourceBounds{
			MaxAmount:       felt.NewRandom[felt.Felt](),
			MaxPricePerUnit: felt.NewRandom[felt.Felt](),
		},
	}
	return rpcv10.BroadcastedTransaction{
		Transaction: rpcv10.Transaction{
			Hash:                  felt.NewRandom[felt.Felt](),
			Type:                  transactionType,
			Version:               felt.NewRandom[felt.Felt](),
			Nonce:                 felt.NewRandom[felt.Felt](),
			MaxFee:                felt.NewRandom[felt.Felt](),
			ContractAddress:       felt.NewRandom[felt.Felt](),
			ContractAddressSalt:   felt.NewRandom[felt.Felt](),
			ClassHash:             felt.NewRandom[felt.Felt](),
			ConstructorCallData:   randomFeltSlice(t),
			SenderAddress:         felt.NewRandom[felt.Felt](),
			Signature:             randomFeltSlice(t),
			CallData:              randomFeltSlice(t),
			EntryPointSelector:    felt.NewRandom[felt.Felt](),
			CompiledClassHash:     felt.NewRandom[felt.Felt](),
			ResourceBounds:        &resourceBounds,
			Tip:                   felt.NewRandom[felt.Felt](),
			PaymasterData:         randomFeltSlice(t),
			AccountDeploymentData: randomFeltSlice(t),
			NonceDAMode:           &nonceDAMode,
			FeeDAMode:             &feeDAMode,
		},
		ContractClass: json.RawMessage("[]"),
	}
}
