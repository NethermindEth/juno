package sequencer

import (
	"crypto/rand"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/stretchr/testify/require"
)

func TestNewBlockSigner(t *testing.T) {
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)

	blockHash := new(felt.Felt).SetUint64(1)
	stateDiffCommitment := new(felt.Felt).SetUint64(2)

	sig, err := newBlockSigner(privKey)(blockHash, stateDiffCommitment)
	require.NoError(t, err)
	require.Len(t, sig, 2)

	var sigBytes [felt.Bytes * 2]byte
	r := sig[0].Bytes()
	s := sig[1].Bytes()
	copy(sigBytes[0:felt.Bytes], r[:])
	copy(sigBytes[felt.Bytes:], s[:])

	msg := crypto.PoseidonArray(blockHash, stateDiffCommitment)
	msgBytes := msg.Bytes()

	ok, err := privKey.PublicKey.Verify(sigBytes[:], msgBytes[:], nil)
	require.NoError(t, err)
	require.True(t, ok)
}
