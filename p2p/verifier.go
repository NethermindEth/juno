package p2p

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/pkg/errors"
)

type Verifier interface {
	VerifyBlock(block *core.Block) error
	VerifyClass(class core.Class, hash *felt.Felt) error
}

var (
	_             Verifier       = &verifier{}
	mismatchLock  *sync.Mutex    = &sync.Mutex{}
	mismatchCount map[string]int = map[string]int{}
)

type verifier struct {
	network utils.Network
}

func (h *verifier) VerifyBlock(block *core.Block) error {
	err := core.VerifyBlockHash(block, h.network)
	if err != nil {
		return err
	}

	for _, transaction := range block.Transactions {
		recalculatedHash, err := core.StrictTransactionHash(transaction, h.network)
		if err != nil {
			return err
		}

		if !recalculatedHash.Equal(transaction.Hash()) {
			// This is definitely bad but still happen
			// return errors.Errorf("mismatched transaction hash %s != %s %T", transaction.Hash().String(), recalculatedHash.String(), transaction)
			h.trackMismatch(fmt.Sprintf("%T", transaction), recalculatedHash, transaction.Hash())
			continue
		}

		recalculatedHash, err = core.StrictTransactionHash(transaction, h.network)
		if err != nil {
			return err
		}

		if !recalculatedHash.Equal(transaction.Hash()) {
			// This is probably bad
			txcategory := reflect.TypeOf(transaction).String()

			if tx, ok := transaction.(*core.InvokeTransaction); ok {
				if tx.Version.IsOne() {
					txcategory += "v1"
				}
				if tx.Version.IsZero() {
					txcategory += "v0"
				}
			}

			h.trackMismatch(txcategory, recalculatedHash, transaction.Hash())
		}
	}

	return nil
}

func (h *verifier) VerifyClass(class core.Class, expectedHash *felt.Felt) error {
	switch v := class.(type) {
	case *core.Cairo1Class:
		err := h.verifyCairo1Hash(v, expectedHash)
		if err != nil {
			return err
		}
	case *core.Cairo0Class:
		hash, err := calculateClas0Hash(v)
		if err != nil {
			return err
		}

		if !hash.Equal(expectedHash) {
			h.trackMismatch("Cairo0Class", hash, expectedHash)
		}
	}

	return nil
}

func (h *verifier) verifyCairo1Hash(coreClass *core.Cairo1Class, expectedHash *felt.Felt) error {
	hash := coreClass.Hash()

	if expectedHash != nil && !expectedHash.Equal(hash) {
		return errors.Errorf("unable to recalculate hash for class %s", expectedHash.String())
	}

	return nil
}

func (h *verifier) trackMismatch(category string, _, _ *felt.Felt) {
	// Just tracking right now...
	mismatchLock.Lock()
	_, ok := mismatchCount[category]
	if !ok {
		mismatchCount[category] = 0
	}

	mismatchCount[category] += 1
	mismatchLock.Unlock()
}

func calculateClas0Hash(c *core.Cairo0Class) (*felt.Felt, error) {
	abistr, err := c.Abi.MarshalJSON()
	if err != nil {
		return nil, err
	}

	abihash, err := crypto.StarknetKeccak(abistr)
	if err != nil {
		return nil, err
	}

	programHash, err := crypto.StarknetKeccak([]byte(c.Program))
	if err != nil {
		return nil, err
	}

	return crypto.PoseidonArray(
		crypto.PoseidonArray(flattenEntryPoints(c.Externals)...),
		crypto.PoseidonArray(flattenEntryPoints(c.L1Handlers)...),
		crypto.PoseidonArray(flattenEntryPoints(c.Constructors)...),
		abihash,
		programHash,
	), nil
}

func flattenEntryPoints(entryPoints []core.EntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*2)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first because the order
		// influences the class hash.
		result[2*i] = entryPoint.Selector
		result[2*i+1] = entryPoint.Offset
	}
	return result
}
