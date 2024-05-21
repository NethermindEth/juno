package blockchain

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type SnapServer interface {
	GetClasses(pivotBlockHash *felt.Felt, classes []*felt.Felt) ([]core.Class, error)
}

var _ SnapServer = &Blockchain{}

func (b *Blockchain) GetClasses(pivotBlockHash *felt.Felt, classes []*felt.Felt) ([]core.Class, error) {
	s, closer, err := b.StateAtBlockHash(pivotBlockHash)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := closer(); err != nil {
			b.log.Errorw("error closing state", "error", err)
		}
	}()

	response := make([]core.Class, len(classes))
	for _, classKey := range classes {
		class, err := s.Class(classKey)
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				response = append(response, nil)
				continue
			}
			return nil, err
		}
		response = append(response, class.Class)
	}

	return response, nil
}
