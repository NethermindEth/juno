package p2p_test

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidKey(t *testing.T) {
	_, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"",
		"peerA",
		"",
		"something",
		false,
		nil,
		&utils.Integration,
		utils.NewNopZapLogger(),
		nil,
	)

	require.Error(t, err)
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
		nil,
		&utils.Integration,
		utils.NewNopZapLogger(),
		testDB,
	)
	require.NoError(t, err)
}

func TestMakeDHTProtocolName(t *testing.T) {
	net, err := mocknet.FullMeshLinked(1)
	require.NoError(t, err)
	testHost := net.Hosts()[0]

	testCases := []struct {
		name     string
		network  *utils.Network
		expected string
	}{
		{
			name:     "sepolia network",
			network:  &utils.Sepolia,
			expected: "/starknet/SN_SEPOLIA/sync/kad/1.0.0",
		},
		{
			name:     "mainnet network",
			network:  &utils.Mainnet,
			expected: "/starknet/SN_MAIN/sync/kad/1.0.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dht, err := p2p.MakeDHT(testHost, nil, tc.network)
			require.NoError(t, err)

			protocols := dht.Host().Mux().Protocols()
			assert.Contains(t, protocols, protocol.ID(tc.expected), "protocol list: %v", protocols)
		})
	}
}
