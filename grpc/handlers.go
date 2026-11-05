//go:generate protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative kv.proto
package grpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
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
		return errors.Join(err, tx.cleanup())
	}
}

func (h Handler) handleTxCursor(
	cur *gen.Cursor,
	tx *tx,
	server gen.KV_TxServer,
) error {
	responsePair := &gen.Pair{}

	// open is special case: it's the only way to receive cursor id
	if cur.Op == gen.Op_OPEN {
		cursorID, err := tx.newCursor(cur.BucketName, len(cur.V) > 0)
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
	} else if cur.Op == gen.Op_CLOSE {
		if err := tx.closeCursor(cur.Cursor); err != nil {
			return fmt.Errorf("cursor %d operation %q: %w", cur.Cursor, cur.Op, err)
		}
		return server.Send(responsePair)
	}

	it, err := tx.iterator(cur.Cursor)
	if err != nil {
		return err
	}
	responsePair.CursorId = cur.Cursor

	valid, err := moveCursor(it, cur)
	if valid {
		responsePair.K = it.Key()
		responsePair.V, err = it.Value()
	}

	if err != nil {
		return fmt.Errorf("cursor %d operation %q: %w", cur.Cursor, cur.Op, err)
	}

	return server.Send(responsePair)
}

// moveCursor applies a positioning op to it and reports whether it landed on an entry.
func moveCursor(it db.Iterator, cur *gen.Cursor) (bool, error) {
	switch cur.Op {
	case gen.Op_FIRST:
		return it.First(), nil
	case gen.Op_LAST:
		return it.Last(), nil
	case gen.Op_SEEK_GE:
		return it.SeekGE(slices.Concat(cur.BucketName, cur.K)), nil
	case gen.Op_SEEK_LT:
		return it.SeekLT(slices.Concat(cur.BucketName, cur.K)), nil
	case gen.Op_SEEK_EXACT:
		key := slices.Concat(cur.BucketName, cur.K)
		return it.SeekGE(key) && bytes.Equal(it.Key(), key), nil
	case gen.Op_NEXT:
		return it.Next(), nil
	case gen.Op_PREV:
		return it.Prev(), nil
	case gen.Op_CURRENT:
		return it.Valid(), nil
	default:
		return false, fmt.Errorf("unknown operation %q", cur.Op)
	}
}
