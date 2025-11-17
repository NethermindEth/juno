package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/mocks"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestVersion(t *testing.T) {
	const version = "1.2.3-rc1"

	handler := New(nil, nil, nil, version, nil, nil)

	ver, err := handler.Version()
	require.Nil(t, err)
	assert.Equal(t, version, ver)
}

func TestRun(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	l1Sub := feed.New[*core.L1Head]()
	newHeadsSub := feed.New[*core.Block]()
	reorgSub := feed.New[*sync.ReorgBlockRange]()
	pendingDataSub := feed.New[core.PendingData]()
	preLatestSub := feed.New[*core.PreLatest]()

	mockBcReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockBcReader.EXPECT().SubscribeL1Head().Return(
		blockchain.L1HeadSubscription{Subscription: l1Sub.Subscribe()},
	).AnyTimes()
	mockSyncReader.EXPECT().SubscribeNewHeads().Return(
		sync.NewHeadSubscription{Subscription: newHeadsSub.Subscribe()},
	).AnyTimes()
	mockSyncReader.EXPECT().SubscribeReorg().Return(
		sync.ReorgSubscription{Subscription: reorgSub.Subscribe()},
	).AnyTimes()
	mockSyncReader.EXPECT().SubscribePendingData().Return(
		sync.PendingDataSubscription{Subscription: pendingDataSub.Subscribe()},
	).AnyTimes()
	mockSyncReader.EXPECT().SubscribePreLatest().Return(
		sync.PreLatestDataSubscription{Subscription: preLatestSub.Subscribe()},
	).AnyTimes()

	handler := &Handler{
		rpcv6Handler:  rpcv6.New(mockBcReader, mockSyncReader, nil, nil, nil),
		rpcv7Handler:  rpcv7.New(mockBcReader, mockSyncReader, nil, nil, nil),
		rpcv8Handler:  rpcv8.New(mockBcReader, mockSyncReader, nil, nil),
		rpcv9Handler:  rpcv9.New(mockBcReader, mockSyncReader, nil, nil),
		rpcv10Handler: rpcv10.New(mockBcReader, mockSyncReader, nil, nil),
		version:       "",
	}

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	t.Cleanup(cancel)

	err := handler.Run(ctx)
	require.NoError(t, err)
}
