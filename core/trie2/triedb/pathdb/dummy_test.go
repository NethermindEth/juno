package pathdb

import (
	"testing"

	"github.com/holiman/uint256"
)

var (
	_uint64  = new(uint256.Int).SetUint64(64)
	_uint256 = new(uint256.Int).SetUint64(256)
	_uint0   = new(uint256.Int).SetUint64(0)
	_uint1   = new(uint256.Int).SetUint64(1)
)

func TestDummy(t *testing.T) {
	getGroup := func(slotNum *uint256.Int) *uint256.Int {
		// If slot number <= 64, return group 0
		if slotNum.Cmp(_uint64) <= 0 {
			return _uint0
		}

		group := new(uint256.Int).Sub(slotNum, _uint64)
		quotient := new(uint256.Int).Div(group, _uint256)
		remainder := new(uint256.Int).Mod(group, _uint256)
		// If there's any remainder, add 1 to round up to next group
		if remainder.Cmp(_uint0) > 0 {
			return new(uint256.Int).Add(quotient, _uint1)
		}
		return quotient
	}

	group := getGroup(new(uint256.Int).SetUint64(64))
	t.Log(group)
}
