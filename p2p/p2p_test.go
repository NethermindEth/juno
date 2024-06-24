package p2p_test

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	t.Skip("TestService")
	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	peerHosts := net.Hosts()
	require.Len(t, peerHosts, 2)

	timeout := time.Second
	testCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	peerA, err := p2p.NewWithHost(
		peerHosts[0],
		"",
		false,
		nil,
		&utils.SepoliaIntegration,
		utils.NewNopZapLogger(),
	)
	require.NoError(t, err)

	events, err := peerA.SubscribePeerConnectednessChanged(testCtx)
	require.NoError(t, err)

	peerAddrs, err := peerA.ListenAddrs()
	require.NoError(t, err)

	var peerAddrsString []string
	for _, addr := range peerAddrs {
		peerAddrsString = append(peerAddrsString, addr.String())
	}

	peerB, err := p2p.NewWithHost(
		peerHosts[1],
		strings.Join(peerAddrsString, ","),
		true,
		nil,
		&utils.SepoliaIntegration,
		utils.NewNopZapLogger(),
	)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, peerA.Run(testCtx))
	}()
	go func() {
		defer wg.Done()
		require.NoError(t, peerB.Run(testCtx))
	}()

	select {
	case evt := <-events:
		require.Equal(t, network.Connected, evt.Connectedness)
	case <-time.After(timeout):
		require.True(t, false, "no events were emitted")
	}

	t.Run("gossip", func(t *testing.T) {
		t.Skip() // todo: flaky test
		topic := "coolTopic"
		ch, closer, err := peerA.SubscribeToTopic(topic)
		require.NoError(t, err)
		t.Cleanup(closer)

		maxRetries := 4
	RetryLoop:
		for i := 0; i < maxRetries; i++ {
			gossipedMessage := []byte(`veryImportantMessage`)
			require.NoError(t, peerB.PublishOnTopic(topic))

			select {
			case <-time.After(time.Second):
				if i == maxRetries-1 {
					require.Fail(t, "timeout: never received the message")
				}
			case msg := <-ch:
				require.Equal(t, gossipedMessage, msg)
				break RetryLoop
			}
		}
	})

	t.Run("protocol handler", func(t *testing.T) {
		ch := make(chan []byte)

		superSecretProtocol := protocol.ID("superSecretProtocol")
		peerA.SetProtocolHandler(superSecretProtocol, func(stream network.Stream) {
			read, err := io.ReadAll(stream)
			require.NoError(t, err)
			ch <- read
		})

		peerAStream, err := peerB.NewStream(testCtx, superSecretProtocol)
		require.NoError(t, err)

		superSecretMessage := []byte(`superSecretMessage`)
		_, err = peerAStream.Write(superSecretMessage)
		require.NoError(t, err)
		require.NoError(t, peerAStream.Close())

		select {
		case <-time.After(timeout):
			require.Equal(t, true, false)
		case msg := <-ch:
			require.Equal(t, superSecretMessage, msg)
		}
	})

	cancel()
	wg.Wait()
}

func TestInvalidKey(t *testing.T) {
	_, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"peerA",
		"",
		"something",
		false,
		nil,
		&utils.SepoliaIntegration,
		utils.NewNopZapLogger(),
	)

	require.Error(t, err)
}

func TestValidKey(t *testing.T) {
	t.Skip("TestValidKey")
	_, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"peerA",
		"",
		"08011240333b4a433f16d7ca225c0e99d0d8c437b835cb74a98d9279c561977690c80f681b25ccf3fa45e2f2de260149c112fa516b69057dd3b0151a879416c0cb12d9b3",
		false,
		nil,
		&utils.SepoliaIntegration,
		utils.NewNopZapLogger(),
	)

	require.NoError(t, err)
}
