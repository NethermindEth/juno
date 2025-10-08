package p2p2core_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
	transaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
	"github.com/stretchr/testify/assert"
)

func TestAdaptTransactionInBlockDeclareV0(t *testing.T) {
	t.Run("SuccessDeclareV0", func(t *testing.T) {
		txn := synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_DeclareV0{
				DeclareV0: &synctransaction.TransactionInBlock_DeclareV0WithoutClass{
					ClassHash: &common.Hash{
						Elements: []byte("0xtestelements"),
					},
					Signature: &transaction.AccountSignature{
						Parts: []*common.Felt252{
							{
								Elements: []byte("0xtestelements"),
							},
						},
					},
				},
			},
		}
		response, err := p2p2core.AdaptTransaction(&txn, &utils.Sepolia)
		assert.Nil(t, err)
		assert.Implements(t, (*core.Transaction)(nil), response)
	})
}
