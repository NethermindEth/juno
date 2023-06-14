package p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/pkg/errors"
)

type Verifier interface {
	VerifyBlock(block *core.Block)
	VerifyClass(class core.Class, hash *felt.Felt) error
}

var _ Verifier = &verifier{}

type verifier struct {
}

func (h *verifier) VerifyBlock(block *core.Block) {
	//TODO implement me
	panic("implement me")
}

func (h *verifier) VerifyClass(class core.Class, hash *felt.Felt) error {
	switch v := class.(type) {
	case *core.Cairo1Class:
		err := h.verifyCairo1Hash(v, hash)
		if err != nil {
			return err
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
