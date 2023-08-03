package starknet

import (
	"context"

	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/proto"
)

type NewStreamFunc func(ctx context.Context, pids ...protocol.ID) (network.Stream, error)

type Client struct {
	newStream  NewStreamFunc
	protocolID protocol.ID
	log        utils.Logger
}

func NewClient(newStream NewStreamFunc, protocolID protocol.ID, log utils.Logger) *Client {
	return &Client{
		newStream:  newStream,
		protocolID: protocolID,
		log:        log,
	}
}

func (c *Client) sendAndReceiveInto(ctx context.Context, req, res proto.Message) error {
	stream, err := c.newStream(ctx, c.protocolID)
	if err != nil {
		return err
	}
	defer stream.Close() // todo: dont ignore close errors

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	if _, err = stream.Write(reqBytes); err != nil {
		return err
	}

	if err = stream.CloseWrite(); err != nil {
		return err
	}

	buffer := getBuffer()
	defer bufferPool.Put(buffer)

	if _, err = buffer.ReadFrom(stream); err != nil {
		return err
	}

	return proto.Unmarshal(buffer.Bytes(), res)
}

func (c *Client) GetBlocks(ctx context.Context, req *spec.GetBlocks) (*spec.GetBlocksResponse, error) {
	wrappedReq := spec.Request{
		Req: &spec.Request_GetBlocks{
			GetBlocks: req,
		},
	}

	var res spec.GetBlocksResponse
	if err := c.sendAndReceiveInto(ctx, &wrappedReq, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
