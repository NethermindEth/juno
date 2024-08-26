package rpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestHandleReadyRequest(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	synchronizer := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	rpchandler := rpc.New(mockReader, synchronizer, nil, "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	syncer := sync.New(nil, nil, nil, 0, false)
	sub := syncer.SubscribeNewHeads()

	t.Run("not ready", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(rpchandler.HandleReadyRequest)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	synchronizer.EXPECT().SubscribeNewHeads().Return(sub)
	assert.Nil(t, rpchandler.Run(ctx))

	t.Run("ready", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(rpchandler.HandleReadyRequest)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestHandleReadySyncRequest(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	synchronizer := mocks.NewMockSyncReader(mockCtrl)
	mockReader := mocks.NewMockReader(mockCtrl)
	rpchandler := rpc.New(mockReader, synchronizer, nil, "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	syncer := sync.New(nil, nil, nil, 0, false)
	sub := syncer.SubscribeNewHeads()

	t.Run("not ready", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(rpchandler.HandleReadySyncRequest)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	synchronizer.EXPECT().SubscribeNewHeads().Return(sub)
	assert.Nil(t, rpchandler.Run(ctx))

	t.Run("ready and blockNumber outside blockRange to highestBlock", func(t *testing.T) {
		blockNum := uint64(2)
		highestBlock := blockNum + rpc.BlockRange + 1
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(rpchandler.HandleReadySyncRequest)
		handler.ServeHTTP(rr, req)

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

		handler := http.HandlerFunc(rpchandler.HandleReadySyncRequest)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("ready & blockNumber is in blockRange of highestBlock", func(t *testing.T) {
		blockNum := uint64(3)
		highestBlock := blockNum + rpc.BlockRange

		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(rpchandler.HandleReadySyncRequest)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
