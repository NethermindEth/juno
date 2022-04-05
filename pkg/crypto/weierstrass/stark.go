package weierstrass

import "math/big"

// Stark returns a Curve which implements the STARK curve as described
// in https://docs.starkware.co/starkex-v4/crypto/stark-curve.
func Stark() Curve {
	stark := CurveParams{Name: "STARK", BitSize: 252, A: big.NewInt(1)}
	stark.B, _ = new(big.Int).SetString("6f21413efbe40de150e596d72f7a8c5609ad26c15c915c1f4cdfcb99cee9e89", 16)
	stark.Gx, _ = new(big.Int).SetString("1ef15c18599971b7beced415a40f0c7deacfd9b0d1819e03d723d8bc943cfca", 16)
	stark.Gy, _ = new(big.Int).SetString("5668060aa49730b7be4801df46ec62de53ecd11abe43a32873000c36e8dc1f", 16)
	stark.N, _ = new(big.Int).SetString("800000000000010ffffffffffffffffb781126dcae7b2321e66a241adc64d2f", 16)
	stark.P, _ = new(big.Int).SetString("800000000000011000000000000000000000000000000000000000000000001", 16)
	return &stark
}
