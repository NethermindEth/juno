package monitor_test

import (
	"testing"

	"github.com/NethermindEth/juno/monitor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncGatewayCounter(t *testing.T) {
	m := monitor.New()

	t.Run("Increments counter correctly", func(t *testing.T) {
		chainIDCounter, err := m.RPCCounterVec.GetMetricWith(prometheus.Labels{"method": "test_method", "path": "/metrics"})
		require.Nil(t, err)
		assert.Equal(t, float64(0), testutil.ToFloat64(chainIDCounter), "The count was not incremented 0")
		m.RPCCounterInc("test_method", "/metrics")
		assert.Equal(t, float64(1), testutil.ToFloat64(chainIDCounter), "The count was not incremented 1")
		m.RPCCounterInc("test_method", "/metrics")
		assert.Equal(t, float64(2), testutil.ToFloat64(chainIDCounter), "The count was not incremented 2")
	})
}
