package crypto

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	starkcurve "github.com/consensys/gnark-crypto/ecc/stark-curve"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

type Signature struct {
	R, S felt.Felt
}

type PublicKey ecdsa.PublicKey

func NewPublicKey(x *felt.Felt) PublicKey {
	return PublicKey(ecdsa.PublicKey{
		A: starkcurve.G1Affine{
			X: *x.Impl(),
		},
	})
}

func yCoord(x *fp.Element) *fp.Element {
	var ySquared fp.Element
	ySquared.Mul(x, x).Mul(&ySquared, x) // x^3
	ySquared.Add(&ySquared, x)           // + x

	_, b := starkcurve.CurveCoefficients()
	ySquared.Add(&ySquared, &b) // ySquared equals to (x^3 + x + b)
	return ySquared.Sqrt(&ySquared)
}

func (k *PublicKey) Verify(sig *Signature, msg *felt.Felt) (bool, error) {
	// only X coordinate is provided
	if k.A.Y.IsZero() {
		y := yCoord(&k.A.X)
		if y == nil {
			return false, errors.New("not a valid public key")
		}

		// set calculated Y and try to verify the signature
		maybeK := *k
		maybeK.A.Y = *y

		if verified, err := maybeK.Verify(sig, msg); err != nil {
			return false, err
		} else if verified {
			*k = maybeK
			return true, nil
		}

		// verification failed, try -y
		maybeK.A.Y.Neg(&maybeK.A.Y)
		if verified, err := maybeK.Verify(sig, msg); err != nil {
			return false, err
		} else if verified {
			*k = maybeK
			return true, nil
		}

		return false, nil
	}

	gnarkPk := (*ecdsa.PublicKey)(k)

	var sigBytes [felt.Bytes * 2]byte
	rBytes := sig.R.Bytes()
	copy(sigBytes[0:felt.Bytes], rBytes[:])
	sBytes := sig.S.Bytes()
	copy(sigBytes[felt.Bytes:], sBytes[:])

	msgBytes := msg.Bytes()
	return gnarkPk.Verify(sigBytes[:], msgBytes[:], nil)
}
