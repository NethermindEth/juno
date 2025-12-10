package utils

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
)

type BlockSignFunc func(blockHash, stateDiffCommitment *felt.Felt) ([]*felt.Felt, error)

// Sign returns the builder's signature over data.
func Sign(privKey *ecdsa.PrivateKey) BlockSignFunc {
	return func(blockHash, stateDiffCommitment *felt.Felt) ([]*felt.Felt, error) {
		data := crypto.PoseidonArray(blockHash, stateDiffCommitment)
		dataBytes := data.Bytes()
		signatureBytes, err := privKey.Sign(dataBytes[:], nil)
		if err != nil {
			return nil, err
		}
		sig := make([]*felt.Felt, 0)
		for start := 0; start < len(signatureBytes); {
			step := min(len(signatureBytes[start:]), felt.Bytes)
			sig = append(sig, new(felt.Felt).SetBytes(signatureBytes[start:step]))
			start += step
		}
		return sig, nil
	}
}
