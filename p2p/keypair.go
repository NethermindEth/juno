package p2p

import (
	"crypto/rand"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
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
