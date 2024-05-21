package blockchain

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type SnapServer interface {
	GetClasses(classes []*felt.Felt) ([]core.Class, error)
}

var _ SnapServer = &Blockchain{}

func (b *Blockchain) GetClasses(classes []*felt.Felt) ([]core.Class, error) {
	s, closer, err := b.HeadState()
	if errors.Is(err, db.ErrKeyNotFound) {
		return make([]core.Class, len(classes)), nil
	}
	if err != nil {
		return nil, err
	}

	defer func() {
		err := closer()
		if err != nil {
			b.log.Errorw("error closing state", "error", err)
		}
	}()

	response := make([]core.Class, 0)
	for _, classKey := range classes {
		class, err := s.Class(classKey)
		if errors.Is(err, db.ErrKeyNotFound) {
			response = append(response, nil)
			continue
		}
		if err != nil {
			return nil, err
		}

		response = append(response, class.Class)
	}

	return response, nil
}
