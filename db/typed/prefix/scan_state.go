package prefix

import (
	"iter"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
)

type scanState[V any] struct {
	prefix  []byte
	scanner scanner[V]
}

func (s scanState[V]) Scan(database db.KeyValueReader) iter.Seq2[Entry[V], error] {
	return s.scanner.scan(database, s.prefix)
}

func (s scanState[V]) DeletePrefix(database db.KeyValueRangeDeleter) error {
	return database.DeleteRange(s.prefix, dbutils.UpperBound(s.prefix))
}
