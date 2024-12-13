package starknet

import (
	"context"
	"errors"
	"io"
	"iter"
	"time"

	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/bufbuild/protovalidate-go"
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

	validator, err := protovalidate.New(
		protovalidate.WithDisableLazy(true),
		protovalidate.WithMessages(&spec.Hash{}),
	)
	if err != nil {
		return nil, err
	}

	return func(yield func(ResT) bool) {
		defer func() {
			closeErr := stream.Close()
			if closeErr != nil {
				log.Errorw("Error while closing stream", "err", closeErr)
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

			if err := validator.Validate(res); err != nil {
				log.Errorw("Error while validating response", "err", err)
				break
			}

			if !yield(res.(ResT)) {
				break
			}
		}
	}, nil
}

func (c *Client) RequestBlockHeaders(
	ctx context.Context, req *spec.BlockHeadersRequest,
) (iter.Seq[*spec.BlockHeadersResponse], error) {
	return requestAndReceiveStream[*spec.BlockHeadersRequest, *spec.BlockHeadersResponse](
		ctx, c.newStream, HeadersPID(), req, c.log)
}

func (c *Client) RequestEvents(ctx context.Context, req *spec.EventsRequest) (iter.Seq[*spec.EventsResponse], error) {
	return requestAndReceiveStream[*spec.EventsRequest, *spec.EventsResponse](ctx, c.newStream, EventsPID(), req, c.log)
}

func (c *Client) RequestClasses(ctx context.Context, req *spec.ClassesRequest) (iter.Seq[*spec.ClassesResponse], error) {
	return requestAndReceiveStream[*spec.ClassesRequest, *spec.ClassesResponse](ctx, c.newStream, ClassesPID(), req, c.log)
}

func (c *Client) RequestStateDiffs(ctx context.Context, req *spec.StateDiffsRequest) (iter.Seq[*spec.StateDiffsResponse], error) {
	return requestAndReceiveStream[*spec.StateDiffsRequest, *spec.StateDiffsResponse](ctx, c.newStream, StateDiffPID(), req, c.log)
}

func (c *Client) RequestTransactions(ctx context.Context, req *spec.TransactionsRequest) (iter.Seq[*spec.TransactionsResponse], error) {
	return requestAndReceiveStream[*spec.TransactionsRequest, *spec.TransactionsResponse](
		ctx, c.newStream, TransactionsPID(), req, c.log)
}
