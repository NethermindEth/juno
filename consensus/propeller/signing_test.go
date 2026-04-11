package propeller_test

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/propeller"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/require"
)

func generateKey(t *testing.T, seed byte) (crypto.PrivKey, crypto.PubKey) {
	t.Helper()
	s := make([]byte, ed25519.SeedSize)
	s[0] = seed
	priv, pub, err := crypto.GenerateEd25519Key(bytes.NewReader(s))
	require.NoError(t, err)
	return priv, pub
}

func TestSignAndVerify(t *testing.T) {
	privA, pubA := generateKey(t, 1)

	root := propeller.MessageRoot{0xAA}
	committeeID := propeller.CommitteeID{0xBB}
	nonce := propeller.Nonce(time.Second)

	sig, err := propeller.SignMessage(privA, &root, &committeeID, nonce)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		tests := []struct {
			name        string
			root        propeller.MessageRoot
			committeeID propeller.CommitteeID
			nonce       propeller.Nonce
		}{
			{
				name:        "valid roundtrip",
				root:        root,
				committeeID: committeeID,
				nonce:       nonce,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := propeller.VerifyMessageSignature(
					pubA, &tc.root, &tc.committeeID, tc.nonce, sig,
				)
				require.NoError(t, err)
			})
		}
	})

	t.Run("error", func(t *testing.T) {
		_, pubB := generateKey(t, 2)

		tests := []struct {
			name        string
			pubKey      crypto.PubKey
			root        propeller.MessageRoot
			committeeID propeller.CommitteeID
			nonce       propeller.Nonce
			signature   propeller.Signature
			wantErr     string
		}{
			{
				name:        "wrong public key",
				pubKey:      pubB,
				root:        root,
				committeeID: committeeID,
				nonce:       nonce,
				signature:   sig,
				wantErr:     "signature is invalid",
			},
			{
				name:        "tampered root",
				pubKey:      pubA,
				root:        propeller.MessageRoot{0xFF},
				committeeID: committeeID,
				nonce:       nonce,
				signature:   sig,
				wantErr:     "signature is invalid",
			},
			{
				name:        "tampered committee ID",
				pubKey:      pubA,
				root:        root,
				committeeID: propeller.CommitteeID{0xFF},
				nonce:       nonce,
				signature:   sig,
				wantErr:     "signature is invalid",
			},
			{
				name:        "tampered nonce",
				pubKey:      pubA,
				root:        root,
				committeeID: committeeID,
				nonce:       propeller.Nonce(time.Hour),
				signature:   sig,
				wantErr:     "signature is invalid",
			},
			{
				name:        "empty signature",
				pubKey:      pubA,
				root:        root,
				committeeID: committeeID,
				nonce:       nonce,
				signature:   nil,
				wantErr:     "empty signature",
			},
			{
				name:        "corrupted signature",
				pubKey:      pubA,
				root:        root,
				committeeID: committeeID,
				nonce:       nonce,
				signature:   append(append([]byte{}, sig...), 0xFF),
				wantErr:     "signature is invalid",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				err := propeller.VerifyMessageSignature(
					tc.pubKey, &tc.root, &tc.committeeID, tc.nonce, tc.signature,
				)
				require.ErrorContains(t, err, tc.wantErr)
			})
		}
	})
}
