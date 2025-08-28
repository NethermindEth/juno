package node_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/node"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestHandleReadySync(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	synchronizer := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	readinessBlockTolerance := uint(6)
	readinessHandlers := node.NewReadinessHandlers(
		mockReader,
		synchronizer,
		readinessBlockTolerance,
	)
	ctx := t.Context()

	t.Run("ready and blockNumber outside blockRange to highestBlock", func(t *testing.T) {
		blockNum := uint64(2)
		highestBlock := blockNum + uint64(readinessBlockTolerance) + 1
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().
			HighestBlockHeader().
			Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("ready & blockNumber is larger than highestBlock", func(t *testing.T) {
		blockNum := uint64(2)
		highestBlock := uint64(1)

		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().
			HighestBlockHeader().
			Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("ready & blockNumber is in blockRange of highestBlock", func(t *testing.T) {
		blockNum := uint64(3)
		highestBlock := blockNum + uint64(readinessBlockTolerance)

		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().
			HighestBlockHeader().
			Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
