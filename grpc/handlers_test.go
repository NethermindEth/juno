package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func makeGrpcStreamMock(t *testing.T) *grpcStreamMock {
	return &grpcStreamMock{
		ctx:            t.Context(),
		recvToServer:   make(chan *gen.Cursor, 10),
		sentFromServer: make(chan *gen.Pair, 10),
	}
}

type grpcStreamMock struct {
	grpc.ServerStream
	ctx            context.Context
	recvToServer   chan *gen.Cursor
	sentFromServer chan *gen.Pair
}

func (m *grpcStreamMock) Context() context.Context {
	return m.ctx
}

func (m *grpcStreamMock) Send(resp *gen.Pair) error {
	m.sentFromServer <- resp
	return nil
}

func (m *grpcStreamMock) Recv() (*gen.Cursor, error) {
	req, more := <-m.recvToServer
	if !more {
		return nil, errors.New("empty")
	}
	return req, nil
}

func (m *grpcStreamMock) SendFromClient(req *gen.Cursor) {
	m.recvToServer <- req
}

func (m *grpcStreamMock) RecvToClient() (*gen.Pair, error) {
	response, more := <-m.sentFromServer
	if !more {
		return nil, errors.New("empty")
	}
	return response, nil
}

func (m *grpcStreamMock) Close() {
	close(m.recvToServer) // signal server that we are done
	<-m.sentFromServer    // wait for server to shutdown
}

func TestHandlers_Version(t *testing.T) {
	expectedVersion := &gen.VersionReply{
		Major: 1,
		Minor: 2,
		Patch: 3,
	}
	h := Handler{version: "1.2.3-rc1"}
	v, err := h.Version(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, expectedVersion, v)
}

func createTxStream(t *testing.T, h Handler) *grpcStreamMock {
	stream := makeGrpcStreamMock(t)
	go func() {
		err := h.Tx(stream)
		if err != nil {
			assert.ErrorContains(t, err, "empty")
		}
		close(stream.sentFromServer)
	}()

	return stream
}

func TestHandlers_Tx(t *testing.T) {
	memDB := memory.New()
	h := Handler{db: memDB}
	stream := createTxStream(t, h)

	prefix := db.ChainHeight
	stream.SendFromClient(&gen.Cursor{
		Op: gen.Op_OPEN,
	})
	cur, err := stream.RecvToClient()
	require.NoError(t, err)
	cID := cur.CursorId

	ops := []gen.Op{
		gen.Op_SEEK,
		gen.Op_SEEK_EXACT,
		gen.Op_NEXT,
		gen.Op_CURRENT,
		gen.Op_GET,
		gen.Op_CLOSE,
	}

	for _, op := range ops {
		stream.SendFromClient(&gen.Cursor{
			Op:         op,
			BucketName: prefix.Key(),
			Cursor:     cID,
		})
		cur, err = stream.RecvToClient()
		require.NoError(t, err)
		assert.Empty(t, cur.K)
		assert.Empty(t, cur.V)
	}
	stream.Close()
}
