package core2p2p

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/stretchr/testify/assert"
)

func TestReceiptCommon(t *testing.T) {
	t.Run("successful", func(t *testing.T) {
		receipt := &core.TransactionReceipt{
			Reverted: false,
		}
		r := receiptCommon(receipt)
		// if RevertReason is nil then receipt considered as non reverted
		assert.Nil(t, r.RevertReason)
	})
	t.Run("reverted", func(t *testing.T) {
		receipts := []*core.TransactionReceipt{
			{
				Reverted:     true,
				RevertReason: "Reason",
			},
			{
				Reverted:     true,
				RevertReason: "",
			},
		}

		for _, receipt := range receipts {
			r := receiptCommon(receipt)
			assert.Equal(t, receipt.Reverted, r)
		}
	})
}
