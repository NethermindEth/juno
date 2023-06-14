package grpc

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestHandlers_Version(t *testing.T) {
	t.Run("version empty", func(t *testing.T) {
		h := handlers{}
		_, err := h.Version(context.Background(), &emptypb.Empty{})
		require.Error(t, err)
	})
	t.Run("version set", func(t *testing.T) {
		expectedVersion := &gen.VersionReply{
			Major: 1,
			Minor: 2,
			Patch: 3,
		}
		h := handlers{junoVersion: "1.2.3-somecommit"}
		v, err := h.Version(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		assert.Equal(t, expectedVersion, v)
	})
}

func createTxStream(t *testing.T, h handlers) *grpcStreamMock {
	stream := makeGrpcStreamMock()
	go func() {
		err := h.Tx(stream)
		if err != nil {
			t.Errorf(err.Error())
		}
		close(stream.sentFromServer)
		close(stream.recvToServer)
	}()

	return stream
}

func TestHandlers_Tx(t *testing.T) {
	memDB := pebble.NewMemTest()
	h := handlers{db: memDB}
	stream := createTxStream(t, h)

	prefix := db.ChainHeight
	stream.SendFromClient(&gen.Cursor{
		Op: gen.Op_OPEN,
	})
	cur, err := stream.RecvToClient()
	require.NoError(t, err)

	ops := []gen.Op{
		gen.Op_SEEK,
		gen.Op_SEEK_EXACT,
		gen.Op_NEXT,
		gen.Op_CURRENT,
	}

	for _, op := range ops {
		stream.SendFromClient(&gen.Cursor{
			Op:         op,
			BucketName: prefix.Key(),
			Cursor:     cur.CursorId,
		})
		cur, err = stream.RecvToClient()
		require.NoError(t, err)
		assert.Empty(t, cur.K)
		assert.Empty(t, cur.V)
	}
}
