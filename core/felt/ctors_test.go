package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

type f = [4]uint64

func TestNumberCtor(t *testing.T) {
	const posValue = 100
	expectedPos := (*f)(new(felt.Felt).SetUint64(posValue))

	t.Run("FromUint64", func(t *testing.T) {
		actual := felt.FromUint64[f](uint64(posValue))
		assert.Equal(t, *expectedPos, actual)
	})
	t.Run("FronUint", func(t *testing.T) {
		actual := felt.FromUint[f](uint32(posValue))
		assert.Equal(t, *expectedPos, actual)
	})

	const negValue = -100
	expectedNeg := (*f)(new(felt.Felt).Sub(&felt.Zero, (*felt.Felt)(expectedPos)))
	t.Run("FromInt64", func(t *testing.T) {
		actual := felt.FromInt64[f](int64(posValue))
		assert.Equal(t, *expectedPos, actual)

		actual = felt.FromInt64[f](int64(negValue))
		assert.Equal(t, *expectedNeg, actual)
	})
	t.Run("FromInt", func(t *testing.T) {
		actual := felt.FromInt[f](int32(posValue))
		assert.Equal(t, *expectedPos, actual)

		actual = felt.FromInt[f](int(negValue))
		assert.Equal(t, *expectedNeg, actual)
	})

	t.Run("New", func(t *testing.T) {
		actual := felt.New[f](posValue)
		assert.Equal(t, expectedPos, actual)

		actual = felt.New[f](negValue)
		assert.Equal(t, expectedNeg, actual)
	})
}

func TestBytesCtor(t *testing.T) {
	value := []byte("holahola")
	expected := (*f)(new(felt.Felt).SetBytes(value))

	t.Run("FromBytes", func(t *testing.T) {
		actual := felt.FromBytes[f](value)
		assert.Equal(t, *expected, actual)
	})
	t.Run("NewBytes", func(t *testing.T) {
		actual := felt.NewFromBytes[f](value)
		assert.Equal(t, expected, actual)
	})
}

func TestStringCtor(t *testing.T) {
	values := []struct {
		value string
		error bool
	}{
		{
			value: "123",
			error: false,
		},
		{
			value: "0x123abcdef",
			error: false,
		},
		{
			value: "ghijklmnopq",
			error: true,
		},
	}

	for _, test := range values {
		expectedFelt, expectedErr := new(felt.Felt).SetString(test.value)
		expected := (*f)(expectedFelt)
		t.Run("FromString"+test.value, func(t *testing.T) {
			actual, actualErr := felt.FromString[f](test.value)
			if test.error {
				assert.EqualError(t, actualErr, expectedErr.Error())
			} else {
				assert.Equal(t, *expected, actual)
			}
		})

		t.Run("NewString"+test.value, func(t *testing.T) {
			actual, actualErr := felt.NewFromString[f](test.value)
			if test.error {
				assert.EqualError(t, actualErr, expectedErr.Error())
			} else {
				assert.Equal(t, expected, actual)
			}
		})
	}
}
