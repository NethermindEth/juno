package crypto

import (
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/stark-curve"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

var (
	shiftPoint  *starkcurve.G1Affine
	p0          *starkcurve.G1Affine
	p1          *starkcurve.G1Affine
	p2          *starkcurve.G1Affine
	p3          *starkcurve.G1Affine
	lowPartMask [4]uint64
)

func init() {
	// The curve points come from the [reference implementation].
	//
	// [reference implementation]: https://github.com/starkware-libs/cairo-lang/blob/de741b92657f245a50caab99cfaef093152fd8be/src/starkware/crypto/signature/fast_pedersen_hash.py

	x := new(fp.Element)
	y := new(fp.Element)

	x.SetString("2089986280348253421170679821480865132823066470938446095505822317253594081284")
	y.SetString("1713931329540660377023406109199410414810705867260802078187082345529207694986")
	shiftPoint = &starkcurve.G1Affine{X: *x, Y: *y}

	x.SetString("996781205833008774514500082376783249102396023663454813447423147977397232763")
	y.SetString("1668503676786377725805489344771023921079126552019160156920634619255970485781")
	p0 = &starkcurve.G1Affine{X: *x, Y: *y}

	x.SetString("2251563274489750535117886426533222435294046428347329203627021249169616184184")
	y.SetString("1798716007562728905295480679789526322175868328062420237419143593021674992973")
	p1 = &starkcurve.G1Affine{X: *x, Y: *y}

	x.SetString("2138414695194151160943305727036575959195309218611738193261179310511854807447")
	y.SetString("113410276730064486255102093846540133784865286929052426931474106396135072156")
	p2 = &starkcurve.G1Affine{X: *x, Y: *y}

	x.SetString("2379962749567351885752724891227938183011949129833673362440656643086021394946")
	y.SetString("776496453633298175483985398648758586525933812536653089401905292063708816422")
	p3 = &starkcurve.G1Affine{X: *x, Y: *y}
}

// PedersenArray implements [Pedersen array hashing].
//
// [Pedersen array hashing]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
func PedersenArray(elems ...*fp.Element) *fp.Element {
	d := new(fp.Element)
	for _, e := range elems {
		d = Pedersen(d, e)
	}
	return Pedersen(d, new(fp.Element).SetUint64(uint64(len(elems))))
}

// Pedersen implements the [Pedersen hash] based on the [reference implementation].
//
// [Pedersen hash]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
// [reference implementation]: https://github.com/starkware-libs/cairo-lang/blob/de741b92657f245a50caab99cfaef093152fd8be/src/starkware/crypto/signature/fast_pedersen_hash.py
func Pedersen(a *fp.Element, b *fp.Element) *fp.Element {
	result := new(starkcurve.G1Affine).Add(shiftPoint, processElement(a, p0, p1))
	return &result.Add(result, processElement(b, p2, p3)).X
}

func processElement(a *fp.Element, p1 *starkcurve.G1Affine, p2 *starkcurve.G1Affine) *starkcurve.G1Affine {
	lowPart := a.ToRegular()
	lowPart[3] = lowPart[3] & 0xffffffffffffff // Zero-out the top nibble (bits 249-252)
	m := new(starkcurve.G1Affine).ScalarMultiplication(p1, lowPart.ToBigInt(new(big.Int)))

	highPart := new(big.Int).SetUint64(a.ToRegular()[3] >> 56 & 0xf) // The top nibble (bits 249-252)
	n := new(starkcurve.G1Affine).ScalarMultiplication(p2, highPart)
	return m.Add(m, n)
}
