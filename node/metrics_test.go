package node

import (
	"errors"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mocks"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func assertGaugeValue(t *testing.T, reg *prometheus.Registry, name string, expected float64) {
	t.Helper()
	metrics, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, metric := range metrics {
		if metric.GetName() == name {
			found = true
			require.Len(t, metric.GetMetric(), 1, "expected 1 metric value")
			assert.Equal(t, expected, metric.GetMetric()[0].GetGauge().GetValue())
		}
	}
	require.True(t, found, "metric %q not found", name)
}

func TestMakeL1Metrics(t *testing.T) {
	originalRegisterer := prometheus.DefaultRegisterer
	defer func() {
		prometheus.DefaultRegisterer = originalRegisterer
	}()
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	t.Run("successful metric reporting", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockBCReader := mocks.NewMockReader(ctrl)
		mockSubscriber := mocks.NewMockSubscriber(ctrl)

		reg := prometheus.NewRegistry()
		prometheus.DefaultRegisterer = reg

		head := core.L1Head{BlockNumber: 42}
		mockBCReader.EXPECT().L1Head().Return(head, nil).AnyTimes()
		mockSubscriber.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(100), nil).AnyTimes()
		mockSubscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(101), nil).AnyTimes()

		listener := makeL1Metrics(mockBCReader, mockSubscriber)
		require.NotNil(t, listener)

		assertGaugeValue(t, reg, "l1_l2_finalised_height", 42)
		assertGaugeValue(t, reg, "l1_finalised_height", 100)
		assertGaugeValue(t, reg, "l1_latest_height", 101)

		listener.OnL1Call("test_method", time.Second)
		assert.Equal(t, 1, testutil.CollectAndCount(reg, "l1_client_request_latency"), "l1_client_request_latency")
	})

	t.Run("error in metric reporting", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockBCReader := mocks.NewMockReader(ctrl)
		mockSubscriber := mocks.NewMockSubscriber(ctrl)

		reg := prometheus.NewRegistry()
		prometheus.DefaultRegisterer = reg

		mockBCReader.EXPECT().L1Head().Return(core.L1Head{}, errors.New("err")).AnyTimes()
		mockSubscriber.EXPECT().FinalisedHeight(gomock.Any()).Return(uint64(0), errors.New("err")).AnyTimes()
		mockSubscriber.EXPECT().LatestHeight(gomock.Any()).Return(uint64(0), errors.New("err")).AnyTimes()

		listener := makeL1Metrics(mockBCReader, mockSubscriber)
		require.NotNil(t, listener)

		assertGaugeValue(t, reg, "l1_l2_finalised_height", 0)
		assertGaugeValue(t, reg, "l1_finalised_height", 0)
		assertGaugeValue(t, reg, "l1_latest_height", 0)
	})
}
