package encoder_test

import (
	"bytes"
	cryptorand "crypto/rand"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/require"
)

const sliceLen = 10

type testStruct struct {
	A []int64
	B []string
}

func generateTestStruct() testStruct {
	res := testStruct{
		A: make([]int64, sliceLen),
		B: make([]string, sliceLen),
	}
	for i := range sliceLen {
		res.A[i] = rand.Int64()
		res.B[i] = cryptorand.Text()
	}
	return res
}

func generateTestStructSlice() []testStruct {
	res := make([]testStruct, sliceLen)
	for i := range sliceLen {
		res[i] = generateTestStruct()
	}
	return res
}

type testCase interface {
	encode(*testing.T, encoder.Encoder)
	assert(*testing.T, []byte) []byte
}

type testCaseData[T any] struct {
	expected T
}

func testData[T any](expected T) testCase {
	return testCaseData[T]{expected: expected}
}

func (c testCaseData[T]) encode(t *testing.T, encoder encoder.Encoder) {
	t.Helper()
	require.NoError(t, encoder.Encode(c.expected))
}

func (c testCaseData[T]) assert(t *testing.T, data []byte) []byte {
	t.Helper()
	var actual T
	remaining, err := encoder.UnmarshalFirst(data, &actual)
	require.NoError(t, err)
	require.Equal(t, c.expected, actual)
	return remaining
}

func TestUnmarshalFirst(t *testing.T) {
	var buf bytes.Buffer
	encoder := encoder.NewEncoder(&buf)

	testCases := []testCase{
		testData(felt.Random[felt.Felt]()),
		testData(cryptorand.Text()),
		testData(rand.Int64()),
		testData(generateTestStructSlice()),
	}
	expectedExtraData := []byte(cryptorand.Text())

	for _, testCase := range testCases {
		testCase.encode(t, encoder)
	}
	_, err := buf.Write(expectedExtraData)
	require.NoError(t, err)

	remaining := buf.Bytes()
	for _, testCase := range testCases {
		remaining = testCase.assert(t, remaining)
	}
	require.Equal(t, expectedExtraData, remaining)
}
