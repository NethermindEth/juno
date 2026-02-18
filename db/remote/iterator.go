package remote

import (
	"slices"

	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

type iterator struct {
	client   gen.KV_TxClient
	cursorID uint32
	log      utils.StructuredLogger
	currentK []byte
	currentV []byte
}

func (i *iterator) doOpAndUpdate(op gen.Op, k []byte) error {
	i.currentK = nil
	i.currentV = nil

	if err := i.client.Send(&gen.Cursor{
		Op:     op,
		Cursor: i.cursorID,
		K:      k,
	}); err != nil {
		return err
	}

	pair, err := i.client.Recv()
	if err != nil {
		return err
	}

	i.currentK = pair.K
	i.currentV = pair.V
	return nil
}

func (i *iterator) Valid() bool {
	if len(i.currentK) == 0 && len(i.currentV) == 0 {
		if err := i.doOpAndUpdate(gen.Op_CURRENT, nil); err != nil {
			i.log.Debug("Error", zap.Stringer("op", gen.Op_CURRENT), zap.Error(err))
		}
	}
	return len(i.currentK) > 0 || len(i.currentV) > 0
}

func (i *iterator) Key() []byte {
	return i.currentK
}

func (i *iterator) Value() ([]byte, error) {
	return slices.Clone(i.currentV), nil
}

// DO NOT USE this if you don't unmarshal the value immediately.
// See [db.Iterator] for more details.
func (i *iterator) UncopiedValue() ([]byte, error) {
	return i.currentV, nil
}

func (i *iterator) First() bool {
	if err := i.doOpAndUpdate(gen.Op_FIRST, nil); err != nil {
		i.log.Debug("Error", zap.Stringer("op", gen.Op_FIRST), zap.Error(err))
	}
	return len(i.currentK) > 0 || len(i.currentV) > 0
}

func (i *iterator) Prev() bool {
	panic("not implemented")
}

func (i *iterator) Next() bool {
	if err := i.doOpAndUpdate(gen.Op_NEXT, nil); err != nil {
		i.log.Debug("Error", zap.Stringer("op", gen.Op_NEXT), zap.Error(err))
	}
	return len(i.currentK) > 0 || len(i.currentV) > 0
}

func (i *iterator) Seek(key []byte) bool {
	if err := i.doOpAndUpdate(gen.Op_SEEK, key); err != nil {
		i.log.Debug("Error", zap.Stringer("op", gen.Op_SEEK), zap.Error(err))
	}
	return len(i.currentK) > 0 || len(i.currentV) > 0
}

func (i *iterator) Close() error {
	return i.doOpAndUpdate(gen.Op_CLOSE, nil)
}
