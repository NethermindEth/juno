package remote

import (
	"slices"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type iterator struct {
	client   gen.KV_TxClient
	cursorID uint32
	logger   log.StructuredLogger
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
			i.logger.Debug("Error", zap.Stringer("op", gen.Op_CURRENT), zap.Error(err))
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
		i.logger.Debug("Error", zap.Stringer("op", gen.Op_FIRST), zap.Error(err))
	}
	return len(i.currentK) > 0 || len(i.currentV) > 0
}

func (i *iterator) Prev() bool {
	panic("not implemented")
}

func (i *iterator) Next() bool {
	if err := i.doOpAndUpdate(gen.Op_NEXT, nil); err != nil {
		i.logger.Debug("Error", zap.Stringer("op", gen.Op_NEXT), zap.Error(err))
	}
	return len(i.currentK) > 0 || len(i.currentV) > 0
}

func (i *iterator) Seek(key []byte) bool {
	if err := i.doOpAndUpdate(gen.Op_SEEK, key); err != nil {
		i.logger.Debug("Error", zap.Stringer("op", gen.Op_SEEK), zap.Error(err))
	}
	return len(i.currentK) > 0 || len(i.currentV) > 0
}

func (i *iterator) Close() error {
	return i.doOpAndUpdate(gen.Op_CLOSE, nil)
}

// ownedIterator holds the only reference to its transaction, so closing it has
// to release the stream. An iterator taken from a batch or a snapshot shares
// that stream with its owner and must leave it alone.
type ownedIterator struct {
	db.Iterator
	txn *transaction
}

// Close discards the transaction, which drops the iterator on the server too.
// It skips [gen.Op_CLOSE]: the round trip is redundant and its error would
// surface as a failure of the scan that has already finished.
func (i *ownedIterator) Close() error {
	return i.txn.Discard()
}
