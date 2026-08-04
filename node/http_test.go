package node_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/node"
	junosync "github.com/NethermindEth/juno/sync"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestHandleReadySync(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	synchronizer := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	readinessBlockTolerance := uint(6)
	readinessHandlers := node.NewReadinessHandlers(mockReader, synchronizer, readinessBlockTolerance)
	ctx := t.Context()

	t.Run("ready and blockNumber outside blockRange to highestBlock", func(t *testing.T) {
		blockNum := uint64(2)
		highestBlock := blockNum + uint64(readinessBlockTolerance) + 1
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

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
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

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
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestHandleReadySyncWithoutSynchronizer(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
	readinessHandlers := node.NewReadinessHandlers(mockReader, &junosync.NoopSynchronizer{}, 6)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ready/sync", http.NoBody)
	rr := httptest.NewRecorder()

	readinessHandlers.HandleReadySync(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHandleReadyRPC(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	t.Run("database serving returns 200", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: 1}, nil)
		readinessHandlers := node.NewReadinessHandlers(mockReader, nil, 0)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ready/rpc", http.NoBody)
		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadyRPC(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("missing canonical head returns 503", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		mockReader.EXPECT().HeadsHeader().Return(nil, db.ErrKeyNotFound)
		readinessHandlers := node.NewReadinessHandlers(mockReader, nil, 0)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ready/rpc", http.NoBody)
		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadyRPC(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		assert.Equal(t, "RPC not ready: failed to read canonical head.", rr.Body.String())
	})

	t.Run("database error returns 503", func(t *testing.T) {
		mockReader := mocks.NewMockReader(mockCtrl)
		mockReader.EXPECT().HeadsHeader().Return(nil, errors.New("database unavailable"))
		readinessHandlers := node.NewReadinessHandlers(mockReader, nil, 0)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ready/rpc", http.NoBody)
		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadyRPC(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
		assert.Equal(t, "RPC not ready: failed to read canonical head.", rr.Body.String())
	})
}
