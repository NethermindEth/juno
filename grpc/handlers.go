//go:generate protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative kv.proto
package grpc

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Handler struct {
	gen.UnimplementedKVServer
	db      db.KeyValueStore
	version string
}

func New(database db.KeyValueStore, version string) *Handler {
	return &Handler{
		db:      database,
		version: version,
	}
}

func (h Handler) Version(ctx context.Context, _ *emptypb.Empty) (*gen.VersionReply, error) {
	ver, err := semver.NewVersion(h.version)
	if err != nil {
		return nil, err
	}

	return &gen.VersionReply{
		Major: uint32(ver.Major()),
		Minor: uint32(ver.Minor()),
		Patch: uint32(ver.Patch()),
	}, nil
}

func (h Handler) Tx(server gen.KV_TxServer) error {
	dbTx := h.db.NewIndexedBatch()
	tx := newTx(dbTx)

	for {
		var (
			cursor *gen.Cursor
			err    error
		)
		if cursor, err = server.Recv(); err == nil {
			if err = h.handleTxCursor(cursor, tx, server); err == nil {
				continue
			}
		}
		return utils.RunAndWrapOnError(tx.cleanup, err)
	}
}

//nolint:gocyclo
func (h Handler) handleTxCursor(
	cur *gen.Cursor,
	tx *tx,
	server gen.KV_TxServer,
) error {
	responsePair := &gen.Pair{}

	// open is special case: it's the only way to receive cursor id
	if cur.Op == gen.Op_OPEN {
		cursorID, err := tx.newCursor()
		if err != nil {
			return err
		}
		responsePair.CursorId = cursorID
		return server.Send(responsePair)
	} else if cur.Op == gen.Op_GET {
		err := tx.dbTx.Get(cur.K, func(data []byte) error {
			if data != nil {
				responsePair.V = data
				responsePair.K = cur.K
			}
			return nil
		})
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}
		return server.Send(responsePair)
	}

	it, err := tx.iterator(cur.Cursor)
	if err != nil {
		return err
	}
	responsePair.CursorId = cur.Cursor

	switch cur.Op {
	case gen.Op_SEEK:
		key := slices.Concat(cur.BucketName, cur.K)
		if it.Seek(key) {
			responsePair.K = it.Key()
			responsePair.V, err = it.Value()
		}
	case gen.Op_SEEK_EXACT:
		key := slices.Concat(cur.BucketName, cur.K)
		if it.Seek(key) && bytes.Equal(it.Key(), key) {
			responsePair.K = it.Key()
			responsePair.V, err = it.Value()
		}
	case gen.Op_NEXT:
		if it.Next() {
			responsePair.K = it.Key()
			responsePair.V, err = it.Value()
		}
	case gen.Op_CURRENT:
		if it.Valid() {
			responsePair.K = it.Key()
			responsePair.V, err = it.Value()
		}
	case gen.Op_CLOSE:
		err = tx.closeCursor(cur.Cursor)
	default:
		err = fmt.Errorf("unknown operation %q", cur.Op)
	}

	if err != nil {
		return errors.Wrapf(err, "cursor %d operation %q", cur.Cursor, cur.Op)
	}

	return server.Send(responsePair)
}
