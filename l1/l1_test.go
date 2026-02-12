package l1_test

import (
	"context"
	"errors"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/l1"
	"github.com/NethermindEth/juno/l1/contract"
	l1types "github.com/NethermindEth/juno/l1/types"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type fakeSubscription struct {
	errChan chan error
	closed  bool
}

func newFakeSubscription() *fakeSubscription {
	return &fakeSubscription{
		errChan: make(chan error),
	}
}

func (s *fakeSubscription) Err() <-chan error {
	return s.errChan
}

func (s *fakeSubscription) Unsubscribe() {
	if !s.closed {
		close(s.errChan)
		s.closed = true
	}
}

func TestFailToCreateSubscription(t *testing.T) {
	t.Parallel()

	err := errors.New("test error")

	network := utils.Mainnet
	ctrl := gomock.NewController(t)
	nopLog := utils.NewNopZapLogger()
	chain := blockchain.New(memory.New(), &network)

	subscriber := mocks.NewMockSubscriber(ctrl)

	subscriber.
		EXPECT().
		WatchLogStateUpdate(gomock.Any(), gomock.Any()).
		Return(newFakeSubscription(), err).
		AnyTimes()

	subscriber.
		EXPECT().
		ChainID(gomock.Any()).
		Return(network.L1ChainID, nil).
		Times(1)

	subscriber.EXPECT().Close().Times(1)

	client := l1.NewClient(subscriber, chain, nopLog).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	require.ErrorContains(t, client.Run(ctx), "context canceled before resubscribe was successful")
	cancel()
}

func TestMismatchedChainID(t *testing.T) {
	t.Parallel()

	network := utils.Mainnet
	ctrl := gomock.NewController(t)
	nopLog := utils.NewNopZapLogger()
	chain := blockchain.New(memory.New(), &network)

	subscriber := mocks.NewMockSubscriber(ctrl)

	subscriber.EXPECT().Close().Times(1)
	subscriber.
		EXPECT().
		ChainID(gomock.Any()).
		Return(new(big.Int), nil).
		Times(1)

	client := l1.NewClient(subscriber, chain, nopLog).WithResubscribeDelay(0).WithPollFinalisedInterval(time.Nanosecond)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)
	err := client.Run(ctx)
	require.ErrorContains(t, err, "mismatched L1 and L2 networks")
}

func TestEventListener(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nopLog := utils.NewNopZapLogger()
	network := utils.Mainnet
	chain := blockchain.New(memory.New(), &network)

	subscriber := mocks.NewMockSubscriber(ctrl)
	subscriber.
		EXPECT().
		WatchLogStateUpdate(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, sink chan<- *contract.StarknetLogStateUpdate) {
			sink <- &contract.StarknetLogStateUpdate{
				GlobalRoot:  new(big.Int),
				BlockNumber: new(big.Int),
				BlockHash:   new(big.Int),
			}
		}).
		Return(newFakeSubscription(), nil).
		Times(1)

	subscriber.
		EXPECT().
		FinalisedHeight(gomock.Any()).
		Return(uint64(0), nil).
		AnyTimes()

	subscriber.
		EXPECT().
		ChainID(gomock.Any()).
		Return(network.L1ChainID, nil).
		Times(1)

	subscriber.EXPECT().Close().Times(1)

	var got *core.L1Head
	client := l1.NewClient(subscriber, chain, nopLog).
		WithResubscribeDelay(0).
		WithPollFinalisedInterval(time.Nanosecond).
		WithEventListener(l1.SelectiveListener{
			OnNewL1HeadCb: func(head *core.L1Head) {
				got = head
			},
		})

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	require.NoError(t, client.Run(ctx))
	cancel()

	require.Equal(t, &core.L1Head{
		BlockHash: new(felt.Felt),
		StateRoot: new(felt.Felt),
	}, got)
}

func newTestL1Client(service service) *rpc.Server {
	server := rpc.NewServer()
	if err := server.RegisterName("eth", service); err != nil {
		panic(err)
	}
	return server
}

type service interface {
	GetBlockByNumber(ctx context.Context, number string, fullTx bool) (any, error)
	BlockNumber(ctx context.Context) (string, error)
}

type testService struct{}

func (testService) GetBlockByNumber(ctx context.Context, number string, fullTx bool) (any, error) {
	blockHeight := big.NewInt(100)
	return types.Header{
		ParentHash:  common.Hash{},
		UncleHash:   common.Hash{},
		Root:        common.Hash{},
		TxHash:      common.Hash{},
		ReceiptHash: common.Hash{},
		Bloom:       types.Bloom{},
		Difficulty:  big.NewInt(0),
		Number:      blockHeight,
		GasLimit:    0,
		GasUsed:     0,
		Time:        0,
		Extra:       []byte{},
	}, nil
}

func (testService) BlockNumber(ctx context.Context) (string, error) {
	return "0xc8", nil // 200 in hex
}

type testEmptyService struct{}

func (testEmptyService) GetBlockByNumber(ctx context.Context, number string, fullTx bool) (any, error) {
	return nil, nil
}

func (testEmptyService) BlockNumber(ctx context.Context) (string, error) {
	return "", errors.New("empty service")
}

type testFaultyService struct{}

func (testFaultyService) GetBlockByNumber(ctx context.Context, number string, fullTx bool) (any, error) {
	return uint(0), nil
}

func (testFaultyService) BlockNumber(ctx context.Context) (string, error) {
	return "invalid", nil
}

func TestEthSubscriber_FinalisedHeight(t *testing.T) {
	tests := createEthSubscriberTests(100)
	testEthSubscriberHeight(t, tests, func(subscriber *l1.EthSubscriber, ctx context.Context) (uint64, error) {
		return subscriber.FinalisedHeight(ctx)
	})
}

func TestEthSubscriber_LatestHeight(t *testing.T) {
	tests := createEthSubscriberTests(200)
	testEthSubscriberHeight(t, tests, func(subscriber *l1.EthSubscriber, ctx context.Context) (uint64, error) {
		return subscriber.LatestHeight(ctx)
	})
}

func createEthSubscriberTests(testServiceExpectedHeight uint64) map[string]struct {
	service        service
	expectedHeight uint64
	expectedError  bool
} {
	return map[string]struct {
		service        service
		expectedHeight uint64
		expectedError  bool
	}{
		"testService": {
			service:        testService{},
			expectedHeight: testServiceExpectedHeight,
			expectedError:  false,
		},
		"testEmptyService": {
			service:        testEmptyService{},
			expectedHeight: 0,
			expectedError:  true,
		},
		"testFaultyService": {
			service:        testFaultyService{},
			expectedHeight: 0,
			expectedError:  true,
		},
	}
}

func testEthSubscriberHeight(t *testing.T, tests map[string]struct {
	service        service
	expectedHeight uint64
	expectedError  bool
}, heightFunc func(*l1.EthSubscriber, context.Context) (uint64, error),
) {
	startServer := func(addr string, service service) (*rpc.Server, net.Listener) {
		srv := newTestL1Client(service)
		var lc net.ListenConfig
		l, err := lc.Listen(t.Context(), "tcp", addr)
		if err != nil {
			t.Fatal("can't listen:", err)
		}
		go func() {
			_ = http.Serve(l, srv.WebsocketHandler([]string{"*"}))
		}()
		return srv, l
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()

			server, listener := startServer("127.0.0.1:0", test.service)
			defer server.Stop()

			subscriber, err := l1.NewEthSubscriber("ws://"+listener.Addr().String(), &l1types.L1Address{})
			require.NoError(t, err)
			defer subscriber.Close()

			height, err := heightFunc(subscriber, ctx)
			require.Equal(t, test.expectedHeight, height)
			require.Equal(t, test.expectedError, err != nil)
		})
	}
}
