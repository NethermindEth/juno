package server_test

import (
	"bufio"
	"context"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/p2p/server"
	"github.com/NethermindEth/juno/p2p/starknetp2p"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	synccommon "github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/common"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/header"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
)

const streamReadTimeout = 5 * time.Second

func TestServer_HeadersRequest(t *testing.T) {
	starknetNetwork := &networks.Sepolia
	protoID := starknetp2p.Sync(starknetNetwork, starknetp2p.HeadersSyncSubProtocol)

	setup := func(t *testing.T) (*mocks.MockReader, host.Host, host.Host) {
		t.Helper()
		mockCtrl := gomock.NewController(t)
		reader := mocks.NewMockReader(mockCtrl)
		reader.EXPECT().Network().Return(starknetNetwork).AnyTimes()

		serverHost, err := libp2p.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = serverHost.Close() })

		clientHost, err := libp2p.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = clientHost.Close() })

		require.NoError(t, clientHost.Connect(t.Context(), peer.AddrInfo{
			ID:    serverHost.ID(),
			Addrs: serverHost.Addrs(),
		}))

		srv := server.New(serverHost, reader, log.NewNopZapLogger())
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			_ = srv.Run(ctx)
			close(done)
		}()
		t.Cleanup(func() {
			cancel()
			<-done
		})

		require.Eventually(t, func() bool {
			return slices.Contains(serverHost.Mux().Protocols(), protoID)
		}, 2*time.Second, 10*time.Millisecond, "headers handler not registered")

		return reader, serverHost, clientHost
	}

	openRequestStream := func(
		t *testing.T,
		clientHost, serverHost host.Host,
		direction synccommon.Iteration_Direction,
	) network.Stream {
		t.Helper()
		stream, err := clientHost.NewStream(t.Context(), serverHost.ID(), protoID)
		require.NoError(t, err)
		t.Cleanup(func() { _ = stream.Close() })

		req := &header.BlockHeadersRequest{
			Iteration: &synccommon.Iteration{
				Direction: direction,
				Start:     &synccommon.Iteration_BlockNumber{BlockNumber: 1},
				Limit:     1,
				Step:      1,
			},
		}
		reqBytes, err := proto.Marshal(req)
		require.NoError(t, err)
		_, err = stream.Write(reqBytes)
		require.NoError(t, err)
		require.NoError(t, stream.CloseWrite())
		require.NoError(t, stream.SetReadDeadline(time.Now().Add(streamReadTimeout)))
		return stream
	}

	t.Run("responds with fin message for known direction", func(t *testing.T) {
		reader, serverHost, clientHost := setup(t)
		// Reader has no block 1 — iterator marks reachedEnd and the server sends the fin message.
		reader.EXPECT().BlockHeaderByNumber(uint64(1)).Return(nil, db.ErrKeyNotFound)

		stream := openRequestStream(t, clientHost, serverHost, synccommon.Iteration_Forward)

		var resp header.BlockHeadersResponse
		require.NoError(t, protodelim.UnmarshalFrom(bufio.NewReader(stream), &resp))
		require.IsType(t, &header.BlockHeadersResponse_Fin{}, resp.HeaderMessage)
	})

	t.Run("drops request for unknown direction", func(t *testing.T) {
		_, serverHost, clientHost := setup(t)
		// No reader EXPECTs: validation must fail before any DB call.

		stream := openRequestStream(t, clientHost, serverHost, synccommon.Iteration_Direction(99))

		data, err := io.ReadAll(stream)
		require.NoError(t, err, "expected clean EOF when server drops the request")
		require.Empty(t, data, "server must not write any response for an unknown direction")
	})
}
