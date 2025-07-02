package p2p_test

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
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
		nil,
	)

	require.Error(t, err)
}

func TestLoadAndPersistPeers(t *testing.T) {
	testDB := memory.New()
	txn := testDB.NewBatch()

	decodedID, err := peer.Decode("12D3KooWLdURCjbp1D7hkXWk6ZVfcMDPtsNnPHuxoTcWXFtvrxGG")
	require.NoError(t, err)

	addrs := []multiaddr.Multiaddr{
		multiaddr.StringCast("/ip4/127.0.0.1/tcp/7777"),
	}
	encAddrs, err := p2p.EncodeAddrs(addrs)
	require.NoError(t, err)

	err = txn.Put(db.Peer.Key([]byte(decodedID)), encAddrs)
	require.NoError(t, err)

	err = txn.Write()
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
		nil,
	)
	require.NoError(t, err)
}
