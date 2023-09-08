package grpc

import (
	"bytes"
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Handler struct {
	db      db.DB
	version string
}

func New(database db.DB, version string) *Handler {
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
	dbTx := h.db.NewTransaction(false)
	tx := newTx(dbTx)

	for {
		cursor, err := server.Recv()
		if err != nil {
			return db.CloseAndWrapOnError(tx.cleanup, err)
		}

		err = h.handleTxCursor(cursor, tx, server)
		if err != nil {
			return db.CloseAndWrapOnError(tx.cleanup, err)
		}
	}
}

func (h Handler) handleTxCursor(
	cur *gen.Cursor,
	tx *tx,
	server gen.KV_TxServer,
) error {
	// open is special case: it's the only way to receive cursor id
	if cur.Op == gen.Op_OPEN {
		cursorID, err := tx.newCursor()
		if err != nil {
			return err
		}

		return server.Send(&gen.Pair{
			CursorId: cursorID,
		})
	}

	it, err := tx.iterator(cur.Cursor)
	if err != nil {
		return err
	}
	responsePair := &gen.Pair{
		CursorId: cur.Cursor,
	}

	switch cur.Op {
	case gen.Op_SEEK:
		key := utils.Flatten(cur.BucketName, cur.K)
		if it.Seek(key) {
			responsePair.K = it.Key()
			responsePair.V, err = it.Value()
		}
	case gen.Op_SEEK_EXACT:
		key := utils.Flatten(cur.BucketName, cur.K)
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
	default:
		err = fmt.Errorf("unknown operation %q", cur.Op)
	}

	if err != nil {
		return errors.Wrapf(err, "cursor %d operation %q", cur.Cursor, cur.Op)
	}

	return server.Send(responsePair)
}
