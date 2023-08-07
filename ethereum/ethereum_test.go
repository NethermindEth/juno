package ethereum_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/ethereum"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// Keccak-256 of `LogStateUpdate(uint256,int256,uint256)`
var logStatUpdateTopic = common.HexToHash("0xd342ddf7a308dec111745b00315c14b7efb2bdae570a6856e088ed0c65a3576c")

type mockTicker struct {
	c chan time.Time
}

func newMockTicker() *mockTicker {
	return &mockTicker{
		c: make(chan time.Time),
	}
}

func (mt *mockTicker) sendTick() {
	mt.c <- time.Time{}
}

func (mt *mockTicker) Tick() <-chan time.Time {
	return mt.c
}

func (mt *mockTicker) Stop() {
	close(mt.c)
}

type ethHandler struct {
	t        *testing.T
	ethBlock *ethBlock
	network  utils.Network
}

func (e *ethHandler) ChainId() *hexutil.Big { //nolint:stylecheck
	return (*hexutil.Big)(e.network.DefaultL1ChainID())
}

func (e *ethHandler) GetBlockByNumber(num rpc.BlockNumber, withTxs bool) (map[string]any, error) {
	require.False(e.t, withTxs)
	require.Equal(e.t, rpc.FinalizedBlockNumber, num)
	result := make(map[string]any)
	result["number"] = fmt.Sprintf("0x%x", e.ethBlock.finalisedHeight)
	return result, nil
}

func (e *ethHandler) Logs(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error) {
	require.Len(e.t, crit.Addresses, 1)
	addr, err := e.network.CoreContractAddress()
	require.NoError(e.t, err)
	require.Equal(e.t, crit.Addresses[0], addr)
	topics := [][]common.Hash{{logStatUpdateTopic}}
	require.Equal(e.t, topics, crit.Topics)

	notifier, supported := rpc.NotifierFromContext(ctx)
	require.True(e.t, supported)
	sub := notifier.CreateSubscription()
	for _, update := range e.ethBlock.updates {
		err := notifier.Notify(sub.ID, update.ToEthLog())
		require.NoError(e.t, err)
	}
	return sub, nil
}

type logStateUpdate struct {
	// The number of the Ethereum block in which the update was emitted.
	l1BlockNumber uint64
	// The Starknet block to which the update corresponds.
	l2BlockNumber uint64
	// This update was previously emitted and has now been reorged.
	removed bool
}

func (update *logStateUpdate) ToCore() *core.L1Head {
	return &core.L1Head{
		BlockNumber: update.l2BlockNumber,
		BlockHash:   new(felt.Felt).SetUint64(update.l2BlockNumber),
		StateRoot:   new(felt.Felt).SetUint64(update.l2BlockNumber),
	}
}

func (update *logStateUpdate) ToEthLog() types.Log {
	stateRootBytes := new(felt.Felt).SetUint64(update.l2BlockNumber).Bytes()
	blockHashBytes := new(felt.Felt).SetUint64(update.l2BlockNumber).Bytes()
	blockNumberBytes := make([]byte, 256/8)
	binary.BigEndian.PutUint64(blockNumberBytes, update.l2BlockNumber)
	data := make([]byte, 0)
	data = append(append(append(data, stateRootBytes[:]...), blockNumberBytes...), blockHashBytes[:]...)
	return types.Log{
		Topics:      []common.Hash{logStatUpdateTopic},
		Data:        data,
		BlockNumber: update.l1BlockNumber,
		Removed:     update.removed,
	}
}

type ethBlock struct {
	finalisedHeight     uint64
	updates             []*logStateUpdate
	expectedL2BlockHash *felt.Felt
}

func TestEthereum(t *testing.T) {
	t.Parallel()
	tests := map[string][]*ethBlock{
		"update L1 head": {
			{
				finalisedHeight: 1,
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(1),
			},
		},
		"ignore removed log": {
			{
				finalisedHeight: 1,
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 3, removed: true},
				},
			},
		},
		"wait for log to be finalised": {
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
				},
			},
		},
		"do not update without logs": {
			{
				finalisedHeight: 1,
				updates:         []*logStateUpdate{},
			},
		},
		"handle updates that appear in the same l1 block": {
			{
				finalisedHeight: 1,
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
					{l1BlockNumber: 1, l2BlockNumber: 2},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(2),
			},
		},
		"multiple blocks and logs finalised every block": {
			{
				finalisedHeight: 1,
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
					{l1BlockNumber: 1, l2BlockNumber: 2},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(2),
			},
			{
				finalisedHeight: 2,
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 3},
					{l1BlockNumber: 2, l2BlockNumber: 4},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(4),
			},
		},
		"multiple blocks and logs finalised irregularly": {
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
					{l1BlockNumber: 1, l2BlockNumber: 2},
				},
			},
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 3},
					{l1BlockNumber: 2, l2BlockNumber: 4},
				},
			},
			{
				finalisedHeight:     2,
				updates:             []*logStateUpdate{},
				expectedL2BlockHash: new(felt.Felt).SetUint64(4),
			},
		},
		"multiple blocks with removed log": {
			{
				finalisedHeight: 0,
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
					{l1BlockNumber: 1, l2BlockNumber: 2},
				},
			},
			{
				finalisedHeight: 1,
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 3},
					{l1BlockNumber: 2, l2BlockNumber: 4},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(2),
			},
			{
				finalisedHeight: 2,
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 4, removed: true},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(2),
			},
		},
		"reorg then finalise earlier block": {
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
				},
			},
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 2},
				},
			},
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 2, removed: true},
				},
			},
			{
				finalisedHeight:     1,
				updates:             []*logStateUpdate{},
				expectedL2BlockHash: new(felt.Felt).SetUint64(1),
			},
		},
		"reorg then finalise later block": {
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
				},
			},
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 2},
					{l1BlockNumber: 2, l2BlockNumber: 3},
				},
			},
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 3, l2BlockNumber: 4},
				},
			},
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 2, removed: true},
				},
			},
			{
				finalisedHeight:     2,
				updates:             []*logStateUpdate{},
				expectedL2BlockHash: new(felt.Felt).SetUint64(1),
			},
		},
		"reorg affecting initial updates": {
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
					{l1BlockNumber: 1, l2BlockNumber: 2},
				},
			},
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 3},
					{l1BlockNumber: 2, l2BlockNumber: 4},
				},
			},
			{
				finalisedHeight: 0,
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 2, removed: true},
				},
			},
		},
		"long sequence of blocks": {
			{
				updates: []*logStateUpdate{
					{l1BlockNumber: 1, l2BlockNumber: 1},
					{l1BlockNumber: 1, l2BlockNumber: 2},
				},
			},
			{
				finalisedHeight: 1,
				updates: []*logStateUpdate{
					{l1BlockNumber: 2, l2BlockNumber: 3},
					{l1BlockNumber: 2, l2BlockNumber: 4},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(2),
			},
			{
				finalisedHeight: 1,
				updates: []*logStateUpdate{
					{l1BlockNumber: 3, l2BlockNumber: 5},
					{l1BlockNumber: 3, l2BlockNumber: 6},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(2),
			},
			{
				finalisedHeight: 2,
				updates: []*logStateUpdate{
					{l1BlockNumber: 4, l2BlockNumber: 7},
					{l1BlockNumber: 4, l2BlockNumber: 8},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(4),
			},
			{
				finalisedHeight: 5,
				updates: []*logStateUpdate{
					{l1BlockNumber: 5, l2BlockNumber: 9},
				},
				expectedL2BlockHash: new(felt.Felt).SetUint64(9),
			},
		},
	}

	for description, ethBlocks := range tests {
		ethBlocks := ethBlocks
		t.Run(description, func(t *testing.T) {
			t.Parallel()
			log := utils.NewNopZapLogger()
			network := utils.MAINNET

			// Set up an RPC server that fakes an Ethereum node.
			handler := &ethHandler{
				t:       t,
				network: network,
			}
			rpcServer := rpc.NewServer()
			require.NoError(t, rpcServer.RegisterName("eth", handler))
			srv := httptest.NewServer(rpcServer.WebsocketHandler([]string{"*"}))
			t.Cleanup(srv.Close)
			url := strings.Replace(srv.URL, "http", "ws", 1)

			eth, err := ethereum.DialWithContext(context.Background(), url, network, log)
			require.NoError(t, err)
			t.Cleanup(eth.Close)
			mt := newMockTicker()
			t.Cleanup(mt.Stop)
			eth.WithCheckFinalisedTicker(mt)

			var currentL2BlockHash *felt.Felt
			for _, ethBlock := range ethBlocks {
				handler.ethBlock = ethBlock
				headsChan := make(chan *core.L1Head)
				sub := eth.WatchL1Heads(context.Background(), headsChan)
				// Give more time for the logs to be received. A little hacky, but it works.
				time.Sleep(50 * time.Millisecond)
				mt.sendTick()

				select {
				case <-time.After(100 * time.Millisecond):
					// We weren't expecting to receive an update for this block.
					require.Equal(t, ethBlock.expectedL2BlockHash, currentL2BlockHash)
				case err := <-sub.Err():
					require.NoError(t, err) // Print the error in the usual format.
				case got := <-headsChan:
					require.Equal(t, ethBlock.expectedL2BlockHash, got.BlockHash)
					currentL2BlockHash = got.BlockHash
				}

				sub.Unsubscribe()
			}
		})
	}
}

type unreliableEthHandler struct {
	h *ethHandler
}

func (e *unreliableEthHandler) ChainId() *hexutil.Big { //nolint:stylecheck
	return e.h.ChainId()
}

func (e *unreliableEthHandler) GetBlockByNumber(num rpc.BlockNumber, withTxs bool) (map[string]any, error) {
	return e.h.GetBlockByNumber(num, withTxs)
}

func (e *unreliableEthHandler) Logs(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error) {
	return nil, errors.New("test error")
}

type logger struct {
	warnMsg string
}

func (l *logger) Debugw(msg string, keysAndValues ...any) {}
func (l *logger) Infow(msg string, keysAndValues ...any)  {}
func (l *logger) Warnw(msg string, keysAndValues ...any) {
	l.warnMsg = msg
}
func (l *logger) Errorw(msg string, keysAndValues ...any) {}

func TestUnreliableSubscription(t *testing.T) {
	t.Parallel()

	network := utils.MAINNET
	handler := &unreliableEthHandler{
		h: &ethHandler{
			t:       t,
			network: network,
		},
	}
	rpcServer := rpc.NewServer()
	require.NoError(t, rpcServer.RegisterName("eth", handler))
	srv := httptest.NewServer(rpcServer.WebsocketHandler([]string{"*"}))
	t.Cleanup(srv.Close)
	url := strings.Replace(srv.URL, "http", "ws", 1)

	log := &logger{}
	eth, err := ethereum.DialWithContext(context.Background(), url, network, log)
	require.NoError(t, err)
	t.Cleanup(eth.Close)
	mt := newMockTicker()
	t.Cleanup(mt.Stop)
	eth.WithCheckFinalisedTicker(mt).WithMaxResubscribeWaitTime(time.Millisecond)

	headsChan := make(chan *core.L1Head)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	t.Cleanup(cancel)
	sub := eth.WatchL1Heads(ctx, headsChan)
	t.Cleanup(sub.Unsubscribe)
	select {
	case err := <-sub.Err():
		require.NoError(t, err) // Print the error in the usual format.
	case <-ctx.Done():
		require.Contains(t, log.warnMsg, "Subscription to Ethereum client failed, resubscribing...")
	}
}

// TODO test mismatched chain ID
