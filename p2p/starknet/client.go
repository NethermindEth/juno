package starknet

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
)

type NewStreamFunc func(ctx context.Context, pids ...protocol.ID) (network.Stream, error)

type Client struct {
	newStream NewStreamFunc
	network   utils.Network
	log       utils.SimpleLogger
}

func NewClient(newStream NewStreamFunc, snNetwork utils.Network, log utils.SimpleLogger) *Client {
	return &Client{
		newStream: newStream,
		network:   snNetwork,
		log:       log,
	}
}

func sendAndCloseWrite(stream network.Stream, req proto.Message) error {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	if _, err = stream.Write(reqBytes); err != nil {
		return err
	}
	return stream.CloseWrite()
}

func receiveInto(stream network.Stream, res proto.Message) error {
	return protodelim.UnmarshalFrom(&byteReader{stream}, res)
}

func requestAndReceiveStream[ReqT proto.Message, ResT proto.Message](ctx context.Context,
	newStream NewStreamFunc, protocolID protocol.ID, req ReqT,
) (Stream[ResT], error) {
	stream, err := newStream(ctx, protocolID)
	if err != nil {
		return nil, err
	}

	id := stream.ID()
	if err := sendAndCloseWrite(stream, req); err != nil {
		fmt.Printf("sendAndCloseWrite (stream is not closed), error: %v (streamID is %v)\n", err, id)
		return nil, err
	}

	return func() (ResT, bool) {
		fmt.Printf("iterator function is called (streamID is %v)\n", id)

		var zero ResT
		res := zero.ProtoReflect().New().Interface()
		if err := receiveInto(stream, res); err != nil {
			// todo: check for a specific error otherwise log the error if it doesn't match
			closeErr := stream.Close() // todo: dont ignore close errors
			if closeErr != nil {
				fmt.Printf("-----------Close stream error-------------: %v\n", closeErr)
			} else {
				fmt.Printf("stream %v is closed for protocol %v\n", id, stream.Protocol())
			}
			return zero, false
		}
		return res.(ResT), true
	}, nil
}

func (c *Client) RequestCurrentBlockHeader(ctx context.Context, req *spec.CurrentBlockHeaderRequest) (Stream[*spec.BlockHeadersResponse], error) {
	return requestAndReceiveStream[*spec.CurrentBlockHeaderRequest, *spec.BlockHeadersResponse](ctx, c.newStream,
		CurrentBlockHeaderPID(c.network), req)
}

func (c *Client) RequestBlockHeaders(ctx context.Context, req *spec.BlockHeadersRequest) (Stream[*spec.BlockHeadersResponse], error) {
	return requestAndReceiveStream[*spec.BlockHeadersRequest, *spec.BlockHeadersResponse](ctx, c.newStream, BlockHeadersPID(c.network), req)
}

func (c *Client) RequestBlockBodies(ctx context.Context, req *spec.BlockBodiesRequest) (Stream[*spec.BlockBodiesResponse], error) {
	return requestAndReceiveStream[*spec.BlockBodiesRequest, *spec.BlockBodiesResponse](ctx, c.newStream, BlockBodiesPID(c.network), req)
}

func (c *Client) RequestEvents(ctx context.Context, req *spec.EventsRequest) (Stream[*spec.EventsResponse], error) {
	return requestAndReceiveStream[*spec.EventsRequest, *spec.EventsResponse](ctx, c.newStream, EventsPID(c.network), req)
}

func (c *Client) RequestReceipts(ctx context.Context, req *spec.ReceiptsRequest) (Stream[*spec.ReceiptsResponse], error) {
	return requestAndReceiveStream[*spec.ReceiptsRequest, *spec.ReceiptsResponse](ctx, c.newStream, ReceiptsPID(c.network), req)
}

func (c *Client) RequestTransactions(ctx context.Context, req *spec.TransactionsRequest) (Stream[*spec.TransactionsResponse], error) {
	return requestAndReceiveStream[*spec.TransactionsRequest, *spec.TransactionsResponse](ctx, c.newStream, TransactionsPID(c.network), req)
}
