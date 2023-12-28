package builder

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
)

type Builder struct {
	ownAddress felt.Felt
	privKey    *ecdsa.PrivateKey

	bc *blockchain.Blockchain
	vm vm.VM

	log utils.Logger
}

func New(privKey *ecdsa.PrivateKey, ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM, log utils.Logger) *Builder {
	return &Builder{
		ownAddress: *ownAddr,
		privKey:    privKey,

		bc: bc,
		vm: builderVM,
	}
}

// Sign returns the builder's signature over data.
func (b *Builder) Sign(blockHash, stateDiffCommitment *felt.Felt) ([]*felt.Felt, error) {
	data := crypto.PoseidonArray(blockHash, stateDiffCommitment).Bytes()
	signatureBytes, err := b.privKey.Sign(data[:], nil)
	if err != nil {
		return nil, err
	}
	sig := make([]*felt.Felt, 0)
	for start := 0; start < len(signatureBytes); {
		step := len(signatureBytes[start:])
		if step > felt.Bytes {
			step = felt.Bytes
		}
		sig = append(sig, new(felt.Felt).SetBytes(signatureBytes[start:step]))
		start += step
	}
	return sig, nil
}

// ValidateAgainstPendingState validates a user transaction against the pending state
// only hard-failures result in an error, reverts are not reported back to caller
func (b *Builder) ValidateAgainstPendingState(userTxn *mempool.BroadcastedTransaction) error {
	declaredClasses := []core.Class{}
	if userTxn.DeclaredClass != nil {
		declaredClasses = []core.Class{userTxn.DeclaredClass}
	}

	nextHeight := uint64(0)
	if height, err := b.bc.Height(); err == nil {
		nextHeight = height + 1
	} else if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
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

	_, _, err = b.vm.Execute([]core.Transaction{userTxn.Transaction}, declaredClasses, nextHeight,
		pendingBlock.Block.Timestamp, &b.ownAddress, state, b.bc.Network(), []*felt.Felt{},
		false, false, false, pendingBlock.Block.GasPrice, pendingBlock.Block.GasPriceSTRK, false)
	return err
}
