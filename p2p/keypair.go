package p2p

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
)

// Note: the curve used to generate the key pair will mostly likely change in the future.
func GenKeyPair() (crypto.PrivKey, crypto.PubKey, peer.ID, error) {
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, nil, "", err
	}

	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, nil, "", err
	}

	return priv, pub, id, nil
}

func Ping(addr string) {
	// Create a new libp2p host
	host, err := libp2p.New()
	if err != nil {
		panic(err)
	}
	defer host.Close()

	// Parse the target peer's multiaddr
	targetAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		panic(err)
	}

	info, err := peer.AddrInfoFromP2pAddr(targetAddr)
	if err != nil {
		panic(err)
	}

	// Connect to the target peer
	if err := host.Connect(context.Background(), *info); err != nil {
		panic(err)
	}

	// Create a ping service
	pingService := ping.NewPingService(host)

	// Ping the peer
	fmt.Printf("Pinging peer %s\n", info.ID)
	ch := pingService.Ping(context.Background(), info.ID)

	// Wait for the result only for maximum 30 seconds
	select {
	case res := <-ch:
		if res.Error != nil {
			fmt.Printf("Ping error: %s\n", res.Error)
		} else {
			fmt.Printf("Ping RTT: %s\n", res.RTT)
		}
	case <-time.After(30 * time.Second):
		fmt.Println("Ping timeout")
	}

	return
}
