package utils_test

import (
	"crypto/rand"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/stretchr/testify/require"
)

func TestSign(t *testing.T) {
	pKrivKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, err = utils.Sign(pKrivKey, new(felt.Felt), new(felt.Felt))
	require.NoError(t, err)
	// We don't check the signature since the private key generation is not deterministic.
}
