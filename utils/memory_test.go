package utils_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestAvailableMemoryMB(t *testing.T) {
	assert.NotZero(t, utils.AvailableMemoryMB())
}
