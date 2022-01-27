package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
)

type PedersenHashCfg struct {
	Comment        string        `json:"_comment"`
	FieldPrime     *big.Int      `json:"FIELD_PRIME"`
	FieldGen       int           `json:"FIELD_GEN"`
	EcOrder        *big.Int      `json:"EC_ORDER"`
	ALPHA          int           `json:"ALPHA"`
	BETA           *big.Int      `json:"BETA"`
	ConstantPoints [][2]*big.Int `json:"CONSTANT_POINTS"`
}

var zero = big.NewInt(0)
var pedersenHashCfg PedersenHashCfg
var EC_ORDER = new(big.Int)
var FIELD_PRIME = new(big.Int)

func igcdex(a, b *big.Int) (*big.Int, *big.Int, *big.Int) {
	// Returns x, y, g such that g = x*a + y*b = gcd(a, b)
	if a.Cmp(zero) == 0 && a.Cmp(zero) == 0 {
		return big.NewInt(0), big.NewInt(1), big.NewInt(0)
	}
	if a.Cmp(zero) == 0 {
		return big.NewInt(0), big.NewInt(0).Quo(b, big.NewInt(0).Abs(b)), big.NewInt(0).Abs(b)
	}
	if b.Cmp(zero) == 0 {
		return big.NewInt(0).Quo(a, big.NewInt(0).Abs(a)), big.NewInt(0), big.NewInt(0).Abs(a)
	}
	xSign := big.NewInt(1)
	ySign := big.NewInt(1)
	if a.Cmp(zero) == -1 {
		a, xSign = a.Neg(a), big.NewInt(-1)
	}
	if b.Cmp(zero) == -1 {
		b, ySign = b.Neg(b), big.NewInt(-1)
	}
	x, y, r, s := big.NewInt(1), big.NewInt(0), big.NewInt(0), big.NewInt(1)
	for b.Cmp(zero) > 0 {
		c, q := big.NewInt(0).Mod(a, b), big.NewInt(0).Quo(a, b)
		a, b, r, s, x, y = b, c, big.NewInt(0).Sub(x, big.NewInt(0).Mul(q, r)), big.NewInt(0).Sub(y, big.NewInt(0).Mul(big.NewInt(0).Neg(q), s)), r, s
	}
	return x.Mul(x, xSign), y.Mul(y, ySign), a
}

func div_mod(n, m, p *big.Int) *big.Int {
	// Finds a non-negative integer 0 <= x < p such that ( m * x ) % p == n
	a, _, _ := igcdex(m, p)
	b := big.NewInt(0).Mul(n, a)
	return b.Mod(b, p)
}

func ec_add(point1 [2]*big.Int, point2 [2]*big.Int, p *big.Int) [2]*big.Int {
	// Gets two points on an elliptic curve mod p and returns their sum, given the points
	// are in affine form (x,y) and have different x coordinates.
	a := big.NewInt(0).Sub(point1[1], point2[1])
	b := big.NewInt(0).Sub(point1[0], point2[0])
	m := div_mod(a, b, p)
	x := big.NewInt(0)
	x.Sub(big.NewInt(0).Mul(m, m), point1[0])
	x.Sub(x, point2[0])
	x.Mod(x, p)
	y := big.NewInt(0)
	y.Mul(m, big.NewInt(0).Sub(point1[0], x))
	y.Sub(y, point1[1])
	y.Mod(y, p)

	return [2]*big.Int{x, y}
}

func init() {
	f, err := os.Open("pedersen_params.json")
	PEDERSEN_PARAMS, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Println("Error")
	}
	_ = json.Unmarshal([]byte(PEDERSEN_PARAMS), &pedersenHashCfg)
	EC_ORDER = pedersenHashCfg.EcOrder
	FIELD_PRIME = pedersenHashCfg.FieldPrime
}

func pedersen_hash(str ...string) string {
	// Calculates the pedersen hash of strings passed as arguments
	N_ELEMENT_BITS_HASH := FIELD_PRIME.BitLen()
	point := pedersenHashCfg.ConstantPoints[0]
	for i, s := range str {
		x, _ := big.NewInt(0).SetString(s, 10)
		point_list := pedersenHashCfg.ConstantPoints[2+i*N_ELEMENT_BITS_HASH : 2+(i+1)*N_ELEMENT_BITS_HASH]
		n := big.NewInt(0)
		for _, pt := range point_list {
			n.And(x, big.NewInt(1))
			if n.Cmp(big.NewInt(0)) > 0 {
				point = ec_add(point, pt, FIELD_PRIME)
			}
			x = x.Rsh(x, 1)
		}
	}
	return point[0].String()
}
