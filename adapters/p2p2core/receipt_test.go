package p2p2core_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/p2p/gen"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestAdaptReceipt(t *testing.T) {
	t.Run("successful", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0xCAFEBABE")
		receipt := &gen.Receipt{
			Type: &gen.Receipt_L1Handler_{
				L1Handler: &gen.Receipt_L1Handler{
					Common: &gen.Receipt_Common{
						RevertReason: nil,
					},
				},
			},
		}
		r := p2p2core.AdaptReceipt(receipt, hash)
		assert.False(t, r.Reverted)
		assert.Equal(t, "", r.RevertReason)
	})
	t.Run("reverted", func(t *testing.T) {
		reasons := []string{"reason", ""}

		for _, reason := range reasons {
			hash := utils.HexToFelt(t, "0xCAFEDEAD")
			receipt := &gen.Receipt{
				Type: &gen.Receipt_L1Handler_{
					L1Handler: &gen.Receipt_L1Handler{
						Common: &gen.Receipt_Common{
							RevertReason: utils.Ptr(reason),
						},
					},
				},
			}
			r := p2p2core.AdaptReceipt(receipt, hash)
			assert.True(t, r.Reverted)
			assert.Equal(t, receipt.GetL1Handler().GetCommon().GetRevertReason(), r.RevertReason)
		}
	})
}
