package sync

import (
	"context"
	"errors"
	"io"
	"iter"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	syncclass "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/class"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/event"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/header"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/state"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
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
	log       utils.StructuredLogger
}

func NewClient(newStream NewStreamFunc, snNetwork *utils.Network, log utils.StructuredLogger) *Client {
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
	newStream NewStreamFunc, protocolID protocol.ID, req ReqT, log utils.StructuredLogger,
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
		log.Error("sendAndCloseWrite (stream is not closed)", utils.SugaredFields("err", err, "streamID", id)...)
		return nil, err
	}

	return func(yield func(ResT) bool) {
		defer func() {
			closeErr := stream.Close()
			if closeErr != nil {
				log.Error("Error while closing stream", utils.SugaredFields("err", closeErr)...)
			}
		}()

		var zero ResT
		res := zero.ProtoReflect().New().Interface()
		for {
			if err := receiveInto(stream, res); err != nil {
				if !errors.Is(err, io.EOF) {
					log.Debug("Error while reading from stream", utils.SugaredFields("err", err)...)
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
	ctx context.Context, req *header.BlockHeadersRequest,
) (iter.Seq[*header.BlockHeadersResponse], error) {
	return requestAndReceiveStream[*header.BlockHeadersRequest, *header.BlockHeadersResponse](
		ctx, c.newStream, HeadersPID(), req, c.log)
}

func (c *Client) RequestEvents(ctx context.Context, req *event.EventsRequest) (iter.Seq[*event.EventsResponse], error) {
	return requestAndReceiveStream[*event.EventsRequest, *event.EventsResponse](ctx, c.newStream, EventsPID(), req, c.log)
}

func (c *Client) RequestClasses(ctx context.Context, req *syncclass.ClassesRequest) (iter.Seq[*syncclass.ClassesResponse], error) {
	return requestAndReceiveStream[*syncclass.ClassesRequest, *syncclass.ClassesResponse](ctx, c.newStream, ClassesPID(), req, c.log)
}

func (c *Client) RequestStateDiffs(ctx context.Context, req *state.StateDiffsRequest) (iter.Seq[*state.StateDiffsResponse], error) {
	return requestAndReceiveStream[*state.StateDiffsRequest, *state.StateDiffsResponse](ctx, c.newStream, StateDiffPID(), req, c.log)
}

func (c *Client) RequestTransactions(
	ctx context.Context,
	req *synctransaction.TransactionsRequest,
) (iter.Seq[*synctransaction.TransactionsResponse], error) {
	return requestAndReceiveStream[*synctransaction.TransactionsRequest, *synctransaction.TransactionsResponse](
		ctx, c.newStream, TransactionsPID(), req, c.log)
}
