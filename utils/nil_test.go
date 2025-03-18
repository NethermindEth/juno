package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNil(t *testing.T) {
	t.Run("both interface's type and value are nil", func(t *testing.T) {
		var i any
		// assert.Nil() is not used to avoid assert package from using reflection
		assert.True(t, IsNil(i))
	})
	t.Run("only interface's value is nil", func(t *testing.T) {
		var arr []int
		var i any = arr
		// assert.Nil() is not used to avoid assert package from using reflection
		assert.True(t, IsNil(i))
	})
}
