package p2p2core_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/receipt"
	"github.com/stretchr/testify/assert"
)

func TestAdaptReceipt(t *testing.T) {
	t.Run("successful", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt]("0xCAFEBABE")
		receipt := &receipt.Receipt{
			Type: &receipt.Receipt_L1Handler_{
				L1Handler: &receipt.Receipt_L1Handler{
					Common: &receipt.Receipt_Common{
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
			hash := felt.NewUnsafeFromString[felt.Felt]("0xCAFEDEAD")
			receipt := &receipt.Receipt{
				Type: &receipt.Receipt_L1Handler_{
					L1Handler: &receipt.Receipt_L1Handler{
						Common: &receipt.Receipt_Common{
							RevertReason: &reason,
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
