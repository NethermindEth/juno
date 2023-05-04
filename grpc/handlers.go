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

type handlers struct {
	db          db.DB
	junoVersion string
}

func (h handlers) Version(ctx context.Context, _ *emptypb.Empty) (*gen.VersionReply, error) {
	v, err := semver.NewVersion(h.junoVersion)
	if err != nil {
		return nil, err
	}

	return &gen.VersionReply{
		Major: uint32(v.Major()),
		Minor: uint32(v.Minor()),
		Patch: uint32(v.Patch()),
	}, nil
}

func (h handlers) Tx(server gen.KV_TxServer) error {
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

func (h handlers) handleTxCursor(
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
