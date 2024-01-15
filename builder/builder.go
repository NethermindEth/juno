package builder

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

type Builder struct {
	ownAddress felt.Felt

	bc *blockchain.Blockchain
	vm vm.VM

	log utils.Logger
}

func New(ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM, log utils.Logger) *Builder {
	return &Builder{
		ownAddress: *ownAddr,

		bc: bc,
		vm: builderVM,
	}
}

// ValidateAgainstPendingState validates a user transaction against the pending state
// only hard-failures result in an error, reverts are not reported back to caller
func (b *Builder) ValidateAgainstPendingState(userTxn *mempool.BroadcastedTransaction) error {
	declaredClasses := []core.Class{}
	if userTxn.DeclaredClass != nil {
		declaredClasses = []core.Class{userTxn.DeclaredClass}
	}

	pendingBlock, err := b.bc.Pending()
	if err != nil {
		return err
	}

	state, stateCloser, err := b.bc.PendingState()
	if err != nil {
		return err
	}

	defer func() {
		if err = stateCloser(); err != nil {
			b.log.Errorw("closing state in ValidateAgainstPendingState", "err", err)
		}
	}()

	_, _, err = b.vm.Execute([]core.Transaction{userTxn.Transaction}, declaredClasses, pendingBlock.Block.Number,
		pendingBlock.Block.Timestamp, &b.ownAddress, state, b.bc.Network(), []*felt.Felt{},
		false, false, false, pendingBlock.Block.GasPrice, pendingBlock.Block.GasPriceSTRK, false)
	return err
}
