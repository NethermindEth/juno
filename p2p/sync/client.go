package sync

import (
	"context"
	"errors"
	"io"
	"iter"
	"time"

	"github.com/NethermindEth/juno/p2p/gen"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
)

const (
	unmarshalMaxSize = 15 * utils.Megabyte
	readTimeout      = 10 * time.Second
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
		MaxSize: unmarshalMaxSize,
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

	err = stream.SetReadDeadline(time.Now().Add(readTimeout))
	if err != nil {
		return nil, err
	}

	id := stream.ID()
	if err := sendAndCloseWrite(stream, req); err != nil {
		log.Errorw("sendAndCloseWrite (stream is not closed)", "err", err, "streamID", id)
		return nil, err
	}

	return func(yield func(ResT) bool) {
		defer func() {
			closeErr := stream.Close()
			if closeErr != nil {
				log.Errorw("Error while closing stream", "err", closeErr)
			}
		}()

		var zero ResT
		res := zero.ProtoReflect().New().Interface()
		for {
			if err := receiveInto(stream, res); err != nil {
				if !errors.Is(err, io.EOF) {
					log.Debugw("Error while reading from stream", "err", err)
				}

				break
			}

			if !yield(res.(ResT)) {
				break
			}

			proto.Reset(res)
		}
	}, nil
}

func (c *Client) RequestBlockHeaders(
	ctx context.Context, req *gen.BlockHeadersRequest,
) (iter.Seq[*gen.BlockHeadersResponse], error) {
	return requestAndReceiveStream[*gen.BlockHeadersRequest, *gen.BlockHeadersResponse](
		ctx, c.newStream, HeadersPID(c.network), req, c.log)
}

func (c *Client) RequestEvents(ctx context.Context, req *gen.EventsRequest) (iter.Seq[*gen.EventsResponse], error) {
	return requestAndReceiveStream[*gen.EventsRequest, *gen.EventsResponse](ctx, c.newStream, EventsPID(c.network), req, c.log)
}

func (c *Client) RequestClasses(ctx context.Context, req *gen.ClassesRequest) (iter.Seq[*gen.ClassesResponse], error) {
	return requestAndReceiveStream[*gen.ClassesRequest, *gen.ClassesResponse](ctx, c.newStream, ClassesPID(c.network), req, c.log)
}

func (c *Client) RequestStateDiffs(ctx context.Context, req *gen.StateDiffsRequest) (iter.Seq[*gen.StateDiffsResponse], error) {
	return requestAndReceiveStream[*gen.StateDiffsRequest, *gen.StateDiffsResponse](ctx, c.newStream, StateDiffPID(c.network), req, c.log)
}

func (c *Client) RequestTransactions(ctx context.Context, req *gen.TransactionsRequest) (iter.Seq[*gen.TransactionsResponse], error) {
	return requestAndReceiveStream[*gen.TransactionsRequest, *gen.TransactionsResponse](
		ctx, c.newStream, TransactionsPID(c.network), req, c.log)
}
