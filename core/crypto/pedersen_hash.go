package crypto

import (
	"github.com/NethermindEth/juno/core/felt"
	starkcurve "github.com/consensys/gnark-crypto/ecc/stark-curve"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

const (
	// Stark field elements fit in 31 full bytes plus one top nibble.
	pedersenLowByteCount        = fp.Bytes - 1
	pedersenLowWindowSelectors  = 1 << 8
	pedersenHighWindowSelectors = 1 << 4
)

var (
	// Fixed Starknet Pedersen points and precomputed lookup tables.
	pedersenShiftPoint  starkcurve.G1Jac
	pedersenLowIndexed  [2][pedersenLowByteCount][pedersenLowWindowSelectors]starkcurve.G1Affine
	pedersenHighIndexed [2][pedersenHighWindowSelectors]starkcurve.G1Affine
	pedersenBasePoints  [4]starkcurve.G1Affine
)

// Parse decimal x/y coordinates into an affine curve point and panic if the
// hardcoded constants are invalid.
func mustSetAffinePoint(point *starkcurve.G1Affine, x, y string) {
	if _, err := point.X.SetString(x); err != nil {
		panic(err)
	}
	if _, err := point.Y.SetString(y); err != nil {
		panic(err)
	}
}

// Parse decimal x/y coordinates into a Jacobian curve point and panic if the
// hardcoded constants are invalid. Z is set separately by the caller.
func mustSetJacobianPoint(point *starkcurve.G1Jac, x, y string) {
	if _, err := point.X.SetString(x); err != nil {
		panic(err)
	}
	if _, err := point.Y.SetString(y); err != nil {
		panic(err)
	}
}

//nolint:gochecknoinits // We precompute Pedersen lookup tables once at package load.
func init() {
	// Match gnark-crypto's constants, but precompute 8-bit tables for the lower
	// 31 bytes and a 4-bit table for the top nibble. That keeps runtime hashing
	// to one lookup per low byte instead of two nibble lookups.
	mustSetJacobianPoint(
		&pedersenShiftPoint,
		"2089986280348253421170679821480865132823066470938446095505822317253594081284",
		"1713931329540660377023406109199410414810705867260802078187082345529207694986",
	)
	pedersenShiftPoint.Z.SetOne()

	mustSetAffinePoint(
		&pedersenBasePoints[0],
		"996781205833008774514500082376783249102396023663454813447423147977397232763",
		"1668503676786377725805489344771023921079126552019160156920634619255970485781",
	)

	mustSetAffinePoint(
		&pedersenBasePoints[1],
		"2251563274489750535117886426533222435294046428347329203627021249169616184184",
		"1798716007562728905295480679789526322175868328062420237419143593021674992973",
	)

	mustSetAffinePoint(
		&pedersenBasePoints[2],
		"2138414695194151160943305727036575959195309218611738193261179310511854807447",
		"113410276730064486255102093846540133784865286929052426931474106396135072156",
	)

	mustSetAffinePoint(
		&pedersenBasePoints[3],
		"2379962749567351885752724891227938183011949129833673362440656643086021394946",
		"776496453633298175483985398648758586525933812536653089401905292063708816422",
	)

	for tableIndex, pointIndex := range [...]int{0, 2} {
		// For each low-byte position, store all 256 possible multiples of the
		// correctly shifted base point.
		byteBase := new(starkcurve.G1Jac).FromAffine(&pedersenBasePoints[pointIndex])
		for byteIndex := range pedersenLowByteCount {
			affineMultiples := buildAffineMultiples(byteBase, pedersenLowWindowSelectors)
			copy(pedersenLowIndexed[tableIndex][byteIndex][:], affineMultiples)
			mulBy256(byteBase)
		}
	}

	for tableIndex, pointIndex := range [...]int{1, 3} {
		// The top nibble has a single fixed position, so one 16-entry table is enough.
		point := new(starkcurve.G1Jac).FromAffine(&pedersenBasePoints[pointIndex])
		affineMultiples := buildAffineMultiples(point, pedersenHighWindowSelectors)
		copy(pedersenHighIndexed[tableIndex][:], affineMultiples)
	}
}

// buildAffineMultiples returns [0*base, 1*base, 2*base, ...] in affine form.
// Repeated Jacobian additions plus one batched affine conversion are cheaper
// here than scalar-multiplying every table entry independently.
func buildAffineMultiples(base *starkcurve.G1Jac, selectorCount int) []starkcurve.G1Affine {
	multiples := make([]starkcurve.G1Affine, selectorCount)
	if selectorCount <= 1 {
		return multiples
	}

	baseAffine := new(starkcurve.G1Affine).FromJacobian(base)
	multiplesJacobian := make([]starkcurve.G1Jac, selectorCount)

	var running starkcurve.G1Jac
	for selector := 1; selector < selectorCount; selector++ {
		running.AddMixed(baseAffine)
		multiplesJacobian[selector] = running
	}

	affineMultiples := starkcurve.BatchJacobianToAffineG1(multiplesJacobian)
	for selector := range selectorCount {
		multiples[selector] = affineMultiples[selector]
	}
	return multiples
}

func mulBy256(point *starkcurve.G1Jac) {
	for range 8 {
		point.DoubleAssign()
	}
}

// PedersenArray implements [Pedersen array hashing].
//
// [Pedersen array hashing]: https://docs.starknet.io/learn/protocol/cryptography#array-hashing
func PedersenArray(elems ...*felt.Felt) felt.Felt {
	var digest PedersenDigest
	return digest.Update(elems...).Finish()
}

// Pedersen implements the [Pedersen hash].
//
// [Pedersen hash]: https://docs.starknet.io/learn/protocol/cryptography#pedersen-hash
func Pedersen(a, b *felt.Felt) felt.Felt {
	return felt.Felt(pedersen(a.Impl(), b.Impl()))
}

var _ Digest = (*PedersenDigest)(nil)

type PedersenDigest struct {
	digest fp.Element
	count  uint64
}

func (d *PedersenDigest) Update(elems ...*felt.Felt) Digest {
	for idx := range elems {
		d.digest = pedersen(&d.digest, elems[idx].Impl())
	}
	d.count += uint64(len(elems))
	return d
}

func (d *PedersenDigest) Finish() felt.Felt {
	var count fp.Element
	count.SetUint64(d.count)
	d.digest = pedersen(&d.digest, &count)
	return felt.Felt(d.digest)
}

func pedersen(a, b *fp.Element) fp.Element {
	// Hot path: unlike gnark-crypto's nibble tables, the lower 248 bits are
	// handled as bytes, so each low window needs one lookup and one mixed add
	// instead of two nibble lookups/adds. Affine tables let the Jacobian
	// accumulator use cheaper mixed adds.
	acc := pedersenShiftPoint

	accumulateLow := func(
		bytes *[fp.Bytes]byte,
		pointIndexed *[pedersenLowByteCount][pedersenLowWindowSelectors]starkcurve.G1Affine,
	) {
		for byteIndex := range pedersenLowByteCount {
			if value := bytes[fp.Bytes-1-byteIndex]; value > 0 {
				acc.AddMixed(&pointIndexed[byteIndex][value])
			}
		}
	}

	aBytes := a.Bytes()
	accumulateLow(&aBytes, &pedersenLowIndexed[0])
	if high := aBytes[0] & 0x0F; high > 0 {
		acc.AddMixed(&pedersenHighIndexed[0][high])
	}

	bBytes := b.Bytes()
	accumulateLow(&bBytes, &pedersenLowIndexed[1])
	if high := bBytes[0] & 0x0F; high > 0 {
		acc.AddMixed(&pedersenHighIndexed[1][high])
	}

	// Recover the affine x-coordinate from the Jacobian accumulator.
	var x fp.Element
	x.Inverse(&acc.Z).
		Square(&x)
	x.Mul(&acc.X, &x)

	return x
}
