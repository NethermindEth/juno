package core2sn_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/core2sn"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
)

func TestAdaptDeprecatedEntryPoint(t *testing.T) {
	selector := new(felt.Felt).SetUint64(0xdeadbeef)
	offset := new(felt.Felt).SetUint64(161)

	ep := &core.DeprecatedEntryPoint{
		Selector: selector,
		Offset:   offset,
	}

	got := core2sn.AdaptDeprecatedEntryPoint(ep)

	assert.Equal(t, selector, got.Selector)
	assert.Equal(t, starknet.EntryPointOffset(*offset), got.Offset)
}
