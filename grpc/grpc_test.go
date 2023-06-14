package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func makeGrpcStreamMock() *grpcStreamMock {
	return &grpcStreamMock{
		ctx:            context.Background(),
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

func TestServer_Run(t *testing.T) {
	server := NewServer(7777, "", nil, nil)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := server.Run(ctx)
		require.NoError(t, err)
	}()

	cancel()
	wg.Wait()
}

func TestClient(t *testing.T) {
	t.Skip("manual testing")

	conn, err := grpc.Dial(":7777", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := gen.NewKVClient(conn)
	t.Run("Version", func(t *testing.T) {
		_, err := client.Version(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
	})
	t.Run("Tx", func(t *testing.T) {
		stream, err := client.Tx(context.Background())
		require.NoError(t, err)

		err = stream.Send(&gen.Cursor{
			Op: gen.Op_OPEN,
		})
		require.NoError(t, err)

		openPair, err := stream.Recv()
		require.NoError(t, err)

		err = stream.Send(&gen.Cursor{
			Op:         gen.Op_SEEK,
			BucketName: db.ChainHeight.Key(),
			Cursor:     openPair.CursorId,
		})
		require.NoError(t, err)

		pair, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				fmt.Println("disconnected from server")
			} else {
				spew.Dump("error", err)
			}
		}

		spew.Dump(pair)
	})
}
