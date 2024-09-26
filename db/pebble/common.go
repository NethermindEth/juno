package pebble

import (
	"errors"
	"io"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
)

type getter interface {
	Get([]byte) ([]byte, io.Closer, error)
}

func get(g getter, key []byte, cb func([]byte) error, listener db.EventListener) error {
	start := time.Now()
	var val []byte
	var closer io.Closer

	val, closer, err := g.Get(key)

	// We need it evaluated immediately so the duration doesn't include the runtime of the user callback that we call below.
	defer listener.OnIO(false, time.Since(start)) //nolint:govet
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return db.ErrKeyNotFound
		}
		return err
	}

	return utils.RunAndWrapOnError(closer.Close, cb(val))
}
