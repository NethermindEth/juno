package p2p2core_test

import (
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAdaptReceipt(t *testing.T) {
	t.Run("reverted", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0xCAFEBABE")
		receipt := &spec.Receipt{
			Type: &spec.Receipt_L1Handler_{
				L1Handler: &spec.Receipt_L1Handler{
					Common: &spec.Receipt_Common{
						RevertReason: nil,
					},
				},
			},
		}
		r := p2p2core.AdaptReceipt(receipt, hash)
		assert.True(t, r.Reverted)
		assert.Equal(t, "", r.RevertReason)
	})
}
