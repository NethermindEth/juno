// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENCE file.

// Package weierstrass provides a standard interface for short-form
// Weierstrass elliptic curves over prime fields.
//
// As a result, it may not be as efficient as the standard library's
// elliptic package to (which it was forked from) but it does allow for
// a uniform interface for a broader set of curves than just standard
// NIST curves with a = -3.
//
// It is based on the following https://github.com/golang/go/pull/26873.
package weierstrass

import (
	"io"
	"math/big"
)

// Curve represents a short-form Weierstrass curve.
//
// The behaviour of Add, Double, and ScalarMult when the input is not a
// point on the curve is undefined.
//
// Note that the conventional point at infinity (0, 0) is not considered
// on the curve, although it can be returned by Add, Double, ScalarMult,
// or ScalarBaseMult (but not the Unmarshal or UnmarshalCompressed
// functions).
type Curve interface {
	// Params returns the parameters for the curve.
	Params() *CurveParams
	// IsOnCurve reports whether the given (x, y) lies on the curve.
	IsOnCurve(x, y *big.Int) bool
	// Add returns the sum of (x1, y1) and (x2, y2).
	Add(x1, y1, x2, y2 *big.Int) (x, y *big.Int)
	// Double returns 2 * (x, y).
	Double(x1, y1 *big.Int) (x, y *big.Int)
	// ScalarMult returns k*(Bx,By) where k is a number in big-endian.
	ScalarMult(x1, y1 *big.Int, k []byte) (x, y *big.Int)
	// ScalarBaseMult returns k * G, where G is the base point of the
	// group and k is an integer in big-endian.
	ScalarBaseMult(k []byte) (x, y *big.Int)
}

// CurveParams contains the parameters of an elliptic curve.
type CurveParams struct {
	A      *big.Int // a coefficient.
	B      *big.Int // b coefficient.
	Gx, Gy *big.Int // Generator.
	N      *big.Int // Order of the generator.
	P      *big.Int // Order of the field.

	BitSize int    // Size of the field.
	Name    string // Canonical name of the curve.
}

// Params returns the CurveParams of the curve.
//
// CurveParams operates, internally, on Jacobian coordinates. For a
// given (x, y) position on the curve, the Jacobian coordinates are
// (x1, y1, z1) where x = x1/z1² and y = y1/z1³. The greatest speedups
// come when the whole calculation can be performed within the transform
// (as in ScalarMult and ScalarBaseMult). But even for Add and Double,
// it's faster to apply and reverse the transform than to operate in
// affine coordinates.
func (curve *CurveParams) Params() *CurveParams { return curve }

// short returns the short Weierstrass form of a curve,
// y² = x³ + ax + b, for a some x.
func (curve *CurveParams) short(x *big.Int) *big.Int {
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	ax := new(big.Int).Mul(curve.A, x)
	ax.Mod(ax, curve.P)

	x3.Add(x3, ax)
	x3.Add(x3, curve.B)
	x3.Mod(x3, curve.P)

	return x3
}

// IsOnCurve reports whether the given (x, y) lies on the curve.
func (curve *CurveParams) IsOnCurve(x, y *big.Int) bool {
	if x.Sign() < 0 || x.Cmp(curve.P) >= 0 ||
		y.Sign() < 0 || y.Cmp(curve.P) >= 0 {
		return false
	}

	// y² = x³ - ax + b (mod p).
	y2 := new(big.Int).Mul(y, y)
	y2.Mod(y2, curve.P)

	return curve.short(x).Cmp(y2) == 0
}

// zForAffine returns a Jacobian Z value for the affine point (x, y). If
// x and y are zero, it assumes that they represent the point at
// infinity.
func zForAffine(x, y *big.Int) *big.Int {
	z := new(big.Int)
	if x.Sign() != 0 || y.Sign() != 0 {
		z.SetInt64(1)
	}
	return z
}

// affineFromJacobian reverses the Jacobian transform. See the comment
// at the top of the file. If the point is ∞ it returns 0, 0.
func (curve *CurveParams) affineFromJacobian(
	x, y, z *big.Int,
) (affX, affY *big.Int) {
	if z.Sign() == 0 {
		return new(big.Int), new(big.Int)
	}

	// (x, y, z) → (x/z², x/z³).

	zInv := new(big.Int).ModInverse(z, curve.P)
	zInvSq := new(big.Int).Mul(zInv, zInv)
	affX = new(big.Int).Mul(x, zInvSq)
	affX.Mod(affX, curve.P)

	zInvSq.Mul(zInvSq, zInv)
	affY = new(big.Int).Mul(y, zInvSq)
	affY.Mod(affY, curve.P)
	return
}

// Add returns the sum of (x1, y1) and (x2, y2).
func (curve *CurveParams) Add(
	x1, y1, x2, y2 *big.Int,
) (*big.Int, *big.Int) {
	z1 := zForAffine(x1, y1)
	z2 := zForAffine(x2, y2)
	return curve.affineFromJacobian(curve.addJacobian(x1, y1, z1, x2, y2, z2))
}

// addJacobian takes two points in Jacobian coordinates, (x1, y1, z1)
// and (x2, y2, z2) and returns their sum, also in Jacobian form.
func (curve *CurveParams) addJacobian(
	x1, y1, z1, x2, y2, z2 *big.Int,
) (*big.Int, *big.Int, *big.Int) {
	// See: https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian.html#addition-add-2007-bl.
	x3, y3, z3 := new(big.Int), new(big.Int), new(big.Int)
	if z1.Sign() == 0 {
		x3.Set(x2)
		y3.Set(y2)
		z3.Set(z2)
		return x3, y3, z3
	}
	if z2.Sign() == 0 {
		x3.Set(x1)
		y3.Set(y1)
		z3.Set(z1)
		return x3, y3, z3
	}

	// z1z1 = z1².
	z1z1 := new(big.Int).Mul(z1, z1)
	z1z1.Mod(z1z1, curve.P)

	// z2z2 = z2².
	z2z2 := new(big.Int).Mul(z2, z2)
	z2z2.Mod(z2z2, curve.P)

	// u1 = z1 * z2z2.
	u1 := new(big.Int).Mul(x1, z2z2)
	u1.Mod(u1, curve.P)

	// u2 = x2 * z1z1.
	u2 := new(big.Int).Mul(x2, z1z1)
	u2.Mod(u2, curve.P)

	// h = u2 - u1.
	h := new(big.Int).Sub(u2, u1)
	xEqual := h.Sign() == 0
	if h.Sign() == -1 {
		h.Add(h, curve.P)
	}

	// i = (2 * h)².
	i := new(big.Int).Lsh(h, 1)
	i.Mul(i, i)

	// j = h * i.
	j := new(big.Int).Mul(h, i)

	// s1 = y1 * z2 * z2z2.
	s1 := new(big.Int).Mul(y1, z2)
	s1.Mul(s1, z2z2)
	s1.Mod(s1, curve.P)

	// s2 = y2 * z1 * z1z1.
	s2 := new(big.Int).Mul(y2, z1)
	s2.Mul(s2, z1z1)
	s2.Mod(s2, curve.P)

	// r = 2 * (s2 - s1).
	r := new(big.Int).Sub(s2, s1)
	if r.Sign() == -1 {
		r.Add(r, curve.P)
	}
	yEqual := r.Sign() == 0
	if xEqual && yEqual {
		// XXX: The following should be remove if more curves are added that
		// can branch out to a more optimised computation at this point.
		// notest
		return curve.doubleJacobian(x1, y1, z1)
	}
	r.Lsh(r, 1)

	// v = u1 * i.
	v := new(big.Int).Mul(u1, i)

	// x3 = r² - j - 2 * v.
	x3.Set(r)
	x3.Mul(x3, x3)
	x3.Sub(x3, j)
	x3.Sub(x3, v)
	x3.Sub(x3, v)
	x3.Mod(x3, curve.P)

	// y3 = r * (v - x3) - 2 * s1 * j.
	y3.Set(r)
	v.Sub(v, x3)
	y3.Mul(y3, v)
	s1.Mul(s1, j)
	s1.Lsh(s1, 1)
	y3.Sub(y3, s1)
	y3.Mod(y3, curve.P)

	// z3 = ((z1 + z2)² - z1z1 - z2z2) * h.
	z3.Add(z1, z2)
	z3.Mul(z3, z3)
	z3.Sub(z3, z1z1)
	z3.Sub(z3, z2z2)
	z3.Mul(z3, h)
	z3.Mod(z3, curve.P)

	return x3, y3, z3
}

// Double returns 2 * (x, y).
func (curve *CurveParams) Double(x1, y1 *big.Int) (*big.Int, *big.Int) {
	z1 := zForAffine(x1, y1)
	return curve.affineFromJacobian(curve.doubleJacobian(x1, y1, z1))
}

// doubleJacobian takes a point in Jacobian coordinates, (x, y, z), and
// returns its double, also in Jacobian form.
func (curve *CurveParams) doubleJacobian(
	x, y, z *big.Int,
) (*big.Int, *big.Int, *big.Int) {
	// See: https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian.html#doubling-dbl-2007-bl.
	// xx = x1².
	xx := new(big.Int).Mul(x, x)
	xx.Mod(xx, curve.P)

	// yy = y1².
	yy := new(big.Int).Mul(y, y)
	yy.Mod(yy, curve.P)

	// yyyy = yy².
	yyyy := new(big.Int).Mul(yy, yy)
	yyyy.Mod(yyyy, curve.P)

	// zz = z1².
	zz := new(big.Int).Mul(z, z)
	zz.Mod(zz, curve.P)

	// s = 2 * ((x1 + yy)² - xx - yyyy).
	s := new(big.Int).Add(x, yy)
	s.Mul(s, s)
	s.Sub(s, xx)
	if s.Sign() == -1 {
		// notest
		s.Add(s, curve.P)
	}
	s.Sub(s, yyyy)
	if s.Sign() == -1 {
		// notest
		s.Add(s, curve.P)
	}
	s.Lsh(s, 1)
	s.Mod(s, curve.P)

	// m = 3 * xx + a * zz².
	m := new(big.Int).Lsh(xx, 1)
	m.Add(m, xx)
	addend := new(big.Int).Mul(zz, zz)
	addend.Mul(curve.A, addend)
	m.Add(m, addend)
	m.Mod(m, curve.P)

	// t = m² - 2 * s.
	t := new(big.Int).Mul(m, m)
	t.Sub(t, new(big.Int).Lsh(s, 1))
	if t.Sign() == -1 {
		// notest
		t.Add(t, curve.P)
	}
	t.Mod(t, curve.P)

	// x3 = t.
	x3 := new(big.Int).Set(t)

	// y3 = m * (s - t) - 8 * yyyy.
	y3 := new(big.Int).Sub(s, t)
	if y3.Sign() == -1 {
		y3.Add(y3, curve.P)
	}
	y3.Mul(y3, m)
	y3.Sub(y3, new(big.Int).Lsh(yyyy, 3))
	if y3.Sign() == -1 {
		// notest
		y3.Add(y3, curve.P)
	}
	y3.Mod(y3, curve.P)

	// z3 = (y1 + z1)² - yy - zz.
	z3 := new(big.Int).Add(y, z)
	z3.Mul(z3, z3)
	z3.Sub(z3, yy)
	if z3.Sign() == -1 {
		// notest
		z3.Add(z3, curve.P)
	}
	z3.Sub(z3, zz)
	if z3.Sign() == -1 {
		// notest
		z3.Add(z3, curve.P)
	}
	z3.Mod(z3, curve.P)

	return x3, y3, z3
}

// ScalarMult returns k * (Bx, By) where k is a number in big-endian.
func (curve *CurveParams) ScalarMult(
	Bx, By *big.Int, k []byte,
) (*big.Int, *big.Int) {
	Bz := new(big.Int).SetInt64(1)
	x, y, z := new(big.Int), new(big.Int), new(big.Int)

	for _, byte := range k {
		for bitNum := 0; bitNum < 8; bitNum++ {
			x, y, z = curve.doubleJacobian(x, y, z)
			if byte&0x80 == 0x80 {
				x, y, z = curve.addJacobian(Bx, By, Bz, x, y, z)
			}
			byte <<= 1
		}
	}

	return curve.affineFromJacobian(x, y, z)
}

// ScalarBaseMult returns k * G, where G is the base point of the group
// and k is an integer in big-endian.
func (curve *CurveParams) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	return curve.ScalarMult(curve.Gx, curve.Gy, k)
}

var mask = []byte{0xff, 0x1, 0x3, 0x7, 0xf, 0x1f, 0x3f, 0x7f}

// GenerateKey returns a public/private key pair. The private key is
// generated using the given reader, which must return random data.
func GenerateKey(
	curve Curve, rand io.Reader,
) (pvt []byte, x, y *big.Int, err error) {
	N := curve.Params().N
	bitSize := N.BitLen()
	byteLen := (bitSize + 7) / 8
	pvt = make([]byte, byteLen)

	for x == nil {
		_, err = io.ReadFull(rand, pvt)
		if err != nil {
			return
		}
		// Mask off any excess bits in the case that the size of the
		// underlying field is not a whole number of bytes.
		pvt[0] &= mask[bitSize%8]
		// This is because, in tests, rand will return all zeros and we
		// don't want to get the point at infinity and loop forever.
		pvt[1] ^= 0x42

		// If the scalar is out of range, sample another random number.
		// notest
		if new(big.Int).SetBytes(pvt).Cmp(N) >= 0 {
			continue
		}

		x, y = curve.ScalarBaseMult(pvt)
	}
	return
}

// Marshal converts a point on the curve into the uncompressed form
// specified in  SEC 1, Version 2.0, Section 2.3.3. If the point is not
// on the curve (or is the conventional point at infinity), the
// behaviour is undefined.
func Marshal(curve Curve, x, y *big.Int) []byte {
	byteLen := (curve.Params().BitSize + 7) / 8

	ret := make([]byte, 1+2*byteLen)
	ret[0] = 4 // uncompressed point

	x.FillBytes(ret[1 : 1+byteLen])
	y.FillBytes(ret[1+byteLen : 1+2*byteLen])

	return ret
}

// MarshalCompressed converts a point on the curve into the compressed
// form specified in SEC 1, Version 2.0, Section 2.3.3. If the point is
// not on the curve (or is the conventional point at infinity), the
// behaviour is undefined.
func MarshalCompressed(curve Curve, x, y *big.Int) []byte {
	byteLen := (curve.Params().BitSize + 7) / 8
	compressed := make([]byte, 1+byteLen)
	compressed[0] = byte(y.Bit(0)) | 2
	x.FillBytes(compressed[1:])
	return compressed
}

// Unmarshal converts a point, serialized by Marshal, into an x, y pair.
// It is an error if the point is not in uncompressed form, is not on
// the curve, or is the point at infinity. On error, x = nil.
func Unmarshal(curve Curve, data []byte) (x, y *big.Int) {
	byteLen := (curve.Params().BitSize + 7) / 8
	if len(data) != 1+2*byteLen {
		return nil, nil
	}
	if data[0] != 4 { // uncompressed form
		// notest
		return nil, nil
	}
	p := curve.Params().P
	x = new(big.Int).SetBytes(data[1 : 1+byteLen])
	y = new(big.Int).SetBytes(data[1+byteLen:])
	if x.Cmp(p) >= 0 || y.Cmp(p) >= 0 {
		return nil, nil
	}
	if !curve.IsOnCurve(x, y) {
		return nil, nil
	}
	return
}

// UnmarshalCompressed converts a point, serialized by
// MarshalCompressed, into an x, y pair. It is an error if the point is
// not in compressed form, is not on the curve, or is the point at
// infinity. On error, x = nil.
func UnmarshalCompressed(curve Curve, data []byte) (x, y *big.Int) {
	byteLen := (curve.Params().BitSize + 7) / 8
	if len(data) != 1+byteLen {
		// notest
		return nil, nil
	}
	if data[0] != 2 && data[0] != 3 { // compressed form
		// notest
		return nil, nil
	}
	p := curve.Params().P
	x = new(big.Int).SetBytes(data[1:])
	if x.Cmp(p) >= 0 {
		// notest
		return nil, nil
	}
	// y² = x³ - 3x + b (mod p).
	y = curve.Params().short(x)
	y = y.ModSqrt(y, p)
	if y == nil {
		// notest
		return nil, nil
	}
	if byte(y.Bit(0)) != data[0]&1 {
		// notest
		y.Neg(y).Mod(y, p)
	}
	if !curve.IsOnCurve(x, y) {
		// notest
		return nil, nil
	}
	return
}
