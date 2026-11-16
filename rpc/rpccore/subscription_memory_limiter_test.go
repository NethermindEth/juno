package rpccore_test

import (
	"testing"

	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/assert"
)

func TestSubscriptionMemoryLimiter(t *testing.T) {
	l := rpccore.NewSubscriptionMemoryLimiter(10)
	assert.Equal(t, int64(10), l.Limit())

	assert.True(t, l.TryAcquire(6))
	assert.False(t, l.TryAcquire(5))
	assert.True(t, l.TryAcquire(4))
	assert.Equal(t, int64(10), l.Used())
	assert.Equal(t, uint64(1), l.Rejected())

	l.Release(6)
	assert.Equal(t, int64(4), l.Used())
	assert.True(t, l.TryAcquire(5))
	assert.Equal(t, uint64(1), l.Rejected())
}
