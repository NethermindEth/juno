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
	readinessHandlers := node.NewReadinessHandlers(mockReader, synchronizer)
	ctx := t.Context()

	t.Run("not ready when blockNumber is behind highestBlock", func(t *testing.T) {
		blockNum := uint64(2)
		highestBlock := blockNum + 1
		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	})

	t.Run("not ready when blockNumber is ahead of highestBlock", func(t *testing.T) {
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

	t.Run("ready when blockNumber equals highestBlock", func(t *testing.T) {
		blockNum := uint64(3)
		highestBlock := blockNum

		mockReader.EXPECT().HeadsHeader().Return(&core.Header{Number: blockNum}, nil)
		synchronizer.EXPECT().HighestBlockHeader().Return(&core.Header{Number: highestBlock, Hash: new(felt.Felt).SetUint64(highestBlock)})

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/ready/sync", http.NoBody)
		assert.Nil(t, err)

		rr := httptest.NewRecorder()

		readinessHandlers.HandleReadySync(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
