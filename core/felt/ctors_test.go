package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

// feltoid is the minimal implementation satisfying [felt.FeltLike] defined to test
// the package generic methods.
// A kind declared outside core/felt still gets the full Typed API.
type feltoidKind struct{}

type feltoid = felt.Typed[feltoidKind]

func TestNumberCtor(t *testing.T) {
	const posValue = 100
	expectedPos := (*feltoid)(new(felt.Felt).SetUint64(posValue))

	t.Run("FromUint64", func(t *testing.T) {
		actual := felt.FromUint64[feltoid](uint64(posValue))
		assert.Equal(t, *expectedPos, actual)
	})

	t.Run("NewFromUint64", func(t *testing.T) {
		actual := felt.NewFromUint64[feltoid](posValue)
		assert.Equal(t, expectedPos, actual)
	})
}

func TestBytesCtor(t *testing.T) {
	value := []byte("holahola")
	expected := (*feltoid)(new(felt.Felt).SetBytes(value))

	t.Run("FromBytes", func(t *testing.T) {
		actual := felt.FromBytes[feltoid](value)
		assert.Equal(t, *expected, actual)
	})
	t.Run("NewBytes", func(t *testing.T) {
		actual := felt.NewFromBytes[feltoid](value)
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
		expected := (*feltoid)(expectedFelt)
		t.Run("FromString"+test.value, func(t *testing.T) {
			actual, actualErr := felt.FromString[feltoid](test.value)
			if test.error {
				assert.EqualError(t, actualErr, expectedErr.Error())
			} else {
				assert.Equal(t, *expected, actual)
			}
		})

		t.Run("UnsafeFromString"+test.value, func(t *testing.T) {
			if test.error {
				assert.PanicsWithError(t, expectedErr.Error(), func() {
					felt.UnsafeFromString[feltoid](test.value)
				})
			} else {
				actual := felt.UnsafeFromString[feltoid](test.value)
				assert.Equal(t, *expected, actual)
			}
		})

		t.Run("NewString"+test.value, func(t *testing.T) {
			actual, actualErr := felt.NewFromString[feltoid](test.value)
			if test.error {
				assert.EqualError(t, actualErr, expectedErr.Error())
			} else {
				assert.Equal(t, expected, actual)
			}
		})

		t.Run("NewUnsafeFromString"+test.value, func(t *testing.T) {
			if test.error {
				assert.PanicsWithError(t, expectedErr.Error(), func() {
					felt.NewUnsafeFromString[feltoid](test.value)
				})
			} else {
				actual := felt.NewUnsafeFromString[feltoid](test.value)
				assert.Equal(t, expected, actual)
			}
		})
	}
}

func TestRandomCtor(t *testing.T) {
	//  Normal random doesn't error
	assert.NotPanics(t, func() {
		felt.Random[feltoid]()
	})

	assert.NotPanics(t, func() {
		felt.NewRandom[feltoid]()
	})
}

func TestZeroOneCtor(t *testing.T) {
	t.Run("values match the field constants for any felt type", func(t *testing.T) {
		assert.Equal(t, felt.FromUint64[feltoid](0), felt.Zero[feltoid]())
		assert.Equal(t, felt.FromUint64[feltoid](1), felt.One[feltoid]())
		assert.Equal(t, felt.FromUint64[felt.Felt](1), felt.One[felt.Felt]())
	})

	t.Run("new(One) returns independent values", func(t *testing.T) {
		first := new(felt.One[felt.Felt]())
		first.SetUint64(7)
		assert.True(t, new(felt.One[felt.Felt]()).IsOne())
	})

	t.Run("shared pointers hold the constants for any felt type", func(t *testing.T) {
		assert.True(t, felt.ZeroPtr[feltoid]().IsZero())
		assert.True(t, felt.OnePtr[feltoid]().IsOne())
		assert.True(t, felt.OnePtr[felt.Felt]().IsOne())
	})

	t.Run("value constructors do not allocate", func(t *testing.T) {
		var sink feltoid
		allocs := testing.AllocsPerRun(100, func() {
			sink = felt.Zero[feltoid]()
			sink = felt.One[feltoid]()
		})
		assert.Zero(t, allocs)
		assert.True(t, sink.IsOne())
	})

	t.Run("shared pointers do not allocate", func(t *testing.T) {
		var sink *feltoid
		allocs := testing.AllocsPerRun(100, func() {
			sink = felt.ZeroPtr[feltoid]()
			sink = felt.OnePtr[feltoid]()
		})
		assert.Zero(t, allocs)
		assert.True(t, sink.IsOne())
	})
}
