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

	t.Run("NewFromUint64", func(t *testing.T) {
		actual := felt.NewFromUint64[f](posValue)
		assert.Equal(t, expectedPos, actual)
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

		t.Run("UnsafeFromString"+test.value, func(t *testing.T) {
			if test.error {
				assert.PanicsWithError(t, expectedErr.Error(), func() {
					felt.UnsafeFromString[f](test.value)
				})
			} else {
				actual := felt.UnsafeFromString[f](test.value)
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

		t.Run("NewUnsafeFromString"+test.value, func(t *testing.T) {
			if test.error {
				assert.PanicsWithError(t, expectedErr.Error(), func() {
					felt.NewUnsafeFromString[f](test.value)
				})
			} else {
				actual := felt.NewUnsafeFromString[f](test.value)
				assert.Equal(t, expected, actual)
			}
		})
	}
}

func TestRandomCtor(t *testing.T) {
	//  Normal random doesn't error
	_, err := felt.Random[f]()
	assert.NoError(t, err)

	// Unsafe random doesn't panic
	_ = felt.UnsafeRandom[f]()

	// New normal random doesn't error
	_, err = felt.NewRandom[f]()
	assert.NoError(t, err)

	// New unsafe random doesn't panic
	_ = felt.NewUnsafeRandom[f]()
}
