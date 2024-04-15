package starknet

import (
	"context"
	"errors"
	"io"

	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/utils/iter"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
)

type NewStreamFunc func(ctx context.Context, pids ...protocol.ID) (network.Stream, error)

type Client struct {
	newStream NewStreamFunc
	network   *utils.Network
	log       utils.SimpleLogger
}

func NewClient(newStream NewStreamFunc, snNetwork *utils.Network, log utils.SimpleLogger) *Client {
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
	unmarshaller := protodelim.UnmarshalOptions{
		MaxSize: 10 * utils.Megabyte,
	}
	return unmarshaller.UnmarshalFrom(&byteReader{stream}, res)
}

func requestAndReceiveStream[ReqT proto.Message, ResT proto.Message](ctx context.Context,
	newStream NewStreamFunc, protocolID protocol.ID, req ReqT, log utils.SimpleLogger,
) (iter.Seq[ResT], error) {
	stream, err := newStream(ctx, protocolID)
	if err != nil {
		return nil, err
	}

	id := stream.ID()
	if err := sendAndCloseWrite(stream, req); err != nil {
		log.Debugw("sendAndCloseWrite (stream is not closed)", "err", err, "streamID", id)
		return nil, err
	}

	return func(yield func(ResT) bool) {
		defer func() {
			closeErr := stream.Close()
			if closeErr != nil {
				log.Debugw("Error while closing stream", "err", closeErr)
			}
		}()

		for {
			var zero ResT
			res := zero.ProtoReflect().New().Interface()
			if err := receiveInto(stream, res); err != nil {
				if !errors.Is(err, io.EOF) {
					log.Debugw("Error while reading from stream", "err", err)
				}

				break
			}

			if !yield(res.(ResT)) {
				break
			}
		}
	}, nil
}

func (c *Client) RequestCurrentBlockHeader(
	ctx context.Context, req *spec.CurrentBlockHeaderRequest,
) (iter.Seq[*spec.BlockHeadersResponse], error) {
	return requestAndReceiveStream[*spec.CurrentBlockHeaderRequest, *spec.BlockHeadersResponse](ctx, c.newStream,
		CurrentBlockHeaderPID(c.network), req, c.log)
}

func (c *Client) RequestBlockHeaders(
	ctx context.Context, req *spec.BlockHeadersRequest,
) (iter.Seq[*spec.BlockHeadersResponse], error) {
	return requestAndReceiveStream[*spec.BlockHeadersRequest, *spec.BlockHeadersResponse](
		ctx, c.newStream, BlockHeadersPID(c.network), req, c.log)
}

func (c *Client) RequestBlockBodies(ctx context.Context, req *spec.BlockBodiesRequest) (iter.Seq[*spec.BlockBodiesResponse], error) {
	return requestAndReceiveStream[*spec.BlockBodiesRequest, *spec.BlockBodiesResponse](
		ctx, c.newStream, BlockBodiesPID(c.network), req, c.log)
}

func (c *Client) RequestEvents(ctx context.Context, req *spec.EventsRequest) (iter.Seq[*spec.EventsResponse], error) {
	return requestAndReceiveStream[*spec.EventsRequest, *spec.EventsResponse](ctx, c.newStream, EventsPID(c.network), req, c.log)
}

func (c *Client) RequestReceipts(ctx context.Context, req *spec.ReceiptsRequest) (iter.Seq[*spec.ReceiptsResponse], error) {
	return requestAndReceiveStream[*spec.ReceiptsRequest, *spec.ReceiptsResponse](ctx, c.newStream, ReceiptsPID(c.network), req, c.log)
}

func (c *Client) RequestTransactions(ctx context.Context, req *spec.TransactionsRequest) (iter.Seq[*spec.TransactionsResponse], error) {
	return requestAndReceiveStream[*spec.TransactionsRequest, *spec.TransactionsResponse](
		ctx, c.newStream, TransactionsPID(c.network), req, c.log)
}
