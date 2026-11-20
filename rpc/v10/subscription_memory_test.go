//go:build !race

package rpcv10_test

import (
	"context"
	"io"
	"runtime"
	"testing"
	"testing/synctest"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// discardConn is a websocket connection without limits that drops what it is sent
type discardConn struct {
	ctx context.Context
}

func (c *discardConn) Write(p []byte) (int, error)   { return io.Discard.Write(p) }
func (c *discardConn) Equal(other jsonrpc.Conn) bool { return c == other }
func (c *discardConn) Context() context.Context      { return c.ctx }
func (c *discardConn) TryAcquireSubscription() bool  { return true }
func (c *discardConn) ReleaseSubscription()          {}

type subscribeFunc func(*rpcv10.Handler, context.Context) *jsonrpc.Error

// The memory charged for a subscription must cover what it really holds
func TestSubscriptionMemoryEstimateCoversRealUsage(t *testing.T) {
	const n = 1000

	addrs := func(count int) []felt.Address {
		out := make([]felt.Address, count)
		for i := range out {
			out[i] = felt.Random[felt.Address]()
		}
		return out
	}
	keys := func(positions, perPosition int) [][]felt.Felt {
		out := make([][]felt.Felt, positions)
		for i := range out {
			out[i] = make([]felt.Felt, perPosition)
			for j := range out[i] {
				out[i][j] = felt.Random[felt.Felt]()
			}
		}
		return out
	}
	events := func(addresses, positions, perPosition int) subscribeFunc {
		return func(h *rpcv10.Handler, ctx context.Context) *jsonrpc.Error {
			_, err := h.SubscribeEvents(
				ctx, addrs(addresses), keys(positions, perPosition), nil, nil,
			)
			return err
		}
	}

	tests := []struct {
		name      string
		subscribe subscribeFunc
	}{
		{"newHeads", func(h *rpcv10.Handler, ctx context.Context) *jsonrpc.Error {
			_, err := h.SubscribeNewHeads(ctx, nil)
			return err
		}},
		{"events without a filter", events(0, 0, 0)},
		{"events, 1024 addresses", events(1024, 0, 0)},
		{"events, 1 position x 1023 keys", events(0, 1, 1023)},
		{"events, 341 positions x 2 keys", events(0, 341, 2)},
		{"events, 512 positions x 1 key", events(0, 512, 1)},
		{"events, 1024 addresses + 512 positions x 1 key", events(1024, 512, 1)},
		{"newTransactions, 1024 senders", func(h *rpcv10.Handler, ctx context.Context) *jsonrpc.Error {
			_, err := h.SubscribeNewTransactions(ctx, nil, addrs(1024), rpcv10.SubscriptionTags{})
			return err
		}},
		{"receipts, 1024 senders", func(h *rpcv10.Handler, ctx context.Context) *jsonrpc.Error {
			_, err := h.SubscribeNewTransactionReceipts(ctx, addrs(1024), nil)
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				chain := mocks.NewMockReader(ctrl)
				filter := mocks.NewMockEventFilterer(ctrl)
				chain.EXPECT().Height().Return(uint64(10), nil).AnyTimes()
				chain.EXPECT().L1Head().Return(core.L1Head{BlockNumber: 9}, nil).AnyTimes()
				chain.EXPECT().BlockHeaderByNumber(gomock.Any()).
					Return(&core.Header{Number: 10}, nil).AnyTimes()
				chain.EXPECT().BlockCommitmentsByNumber(gomock.Any()).
					Return(&core.BlockCommitments{}, nil).AnyTimes()
				chain.EXPECT().EventFilter(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(filter, nil).AnyTimes()
				filter.EXPECT().SetRangeEndBlockByNumber(gomock.Any(), gomock.Any()).
					Return(nil).AnyTimes()
				filter.EXPECT().Events(gomock.Any(), gomock.Any()).
					Return(nil, blockchain.ContinuationToken{}, nil).AnyTimes()
				filter.EXPECT().Close().Return(nil).AnyTimes()

				limiter := rpccore.NewSubscriptionMemoryLimiter(0)
				handler := rpcv10.New(chain, nil, nil, log.NewNopZapLogger()).
					WithSubscriptionMemoryLimiter(limiter)

				connCtx, cancel := context.WithCancel(t.Context())
				ctx := context.WithValue(connCtx, jsonrpc.ConnKey{}, &discardConn{ctx: connCtx})

				before := memoryInUse()
				for range n {
					require.Nil(t, tc.subscribe(handler, ctx))
				}
				// Every subscription has replayed history and waits for new data
				synctest.Wait()
				held := memoryInUse() - before

				t.Logf("held %d B, charged %d B per subscription", held/n, limiter.Used()/n)
				assert.LessOrEqual(t, held, limiter.Used())

				cancel()
				synctest.Wait()
				assert.Zero(t, limiter.Used(), "closing the connection releases the memory")
			})
		})
	}
}

// memoryInUse is the live heap plus goroutine stacks
func memoryInUse() int64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc + m.StackInuse)
}
