package p2p_test

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/multiformats/go-multiaddr"
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
		p2p.FullSync,
		nil,
		&utils.Integration,
		utils.NewNopZapLogger(),
		nil,
	)
	require.NoError(t, err)

	events, err := peerA.SubscribePeerConnectednessChanged(testCtx)
	require.NoError(t, err)

	peerAddrs, err := peerA.ListenAddrs()
	require.NoError(t, err)

	peerAddrsString := make([]string, 0, len(peerAddrs))
	for _, addr := range peerAddrs {
		peerAddrsString = append(peerAddrsString, addr.String())
	}

	peerB, err := p2p.NewWithHost(
		peerHosts[1],
		strings.Join(peerAddrsString, ","),
		true,
		p2p.FullSync,
		nil,
		&utils.Integration,
		utils.NewNopZapLogger(),
		nil,
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
		"",
		"peerA",
		"",
		"something",
		false,
		p2p.FullSync,
		nil,
		&utils.Integration,
		utils.NewNopZapLogger(),
		nil,
	)

	require.Error(t, err)
}

func TestValidKey(t *testing.T) {
	t.Skip("TestValidKey")
	_, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"",
		"peerA",
		"",
		"08011240333b4a433f16d7ca225c0e99d0d8c437b835cb74a98d9279c561977690c80f681b25ccf3fa45e2f2de260149c112fa516b69057dd3b0151a879416c0cb12d9b3",
		false,
		p2p.FullSync,
		nil,
		&utils.Integration,
		utils.NewNopZapLogger(),
		nil,
	)

	require.NoError(t, err)
}

func TestLoadAndPersistPeers(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)

	decodedID, err := peer.Decode("12D3KooWLdURCjbp1D7hkXWk6ZVfcMDPtsNnPHuxoTcWXFtvrxGG")
	require.NoError(t, err)

	addrs := []multiaddr.Multiaddr{
		multiaddr.StringCast("/ip4/127.0.0.1/tcp/7777"),
	}
	encAddrs, err := p2p.EncodeAddrs(addrs)
	require.NoError(t, err)

	err = txn.Set(db.Peer.Key([]byte(decodedID)), encAddrs)
	require.NoError(t, err)

	err = txn.Commit()
	require.NoError(t, err)

	_, err = p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"",
		"peerA",
		"",
		"5f6cdc3aebcc74af494df054876100368ef6126e3a33fa65b90c765b381ffc37a0a63bbeeefab0740f24a6a38dabb513b9233254ad0020c721c23e69bc820089",
		false,
		p2p.FullSync,
		nil,
		&utils.Integration,
		utils.NewNopZapLogger(),
		testDB,
	)
	require.NoError(t, err)
}
