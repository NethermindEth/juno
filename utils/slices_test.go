package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlatten(t *testing.T) {
	expected := []int{1, 2, 3, 4, 5, 6}
	halfLen := len(expected) / 2
	actual := Flatten(expected[:halfLen], expected[halfLen:])
	assert.Equal(t, expected, actual)
}
