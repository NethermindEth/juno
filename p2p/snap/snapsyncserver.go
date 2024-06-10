package snap

//go:generate protoc --go_out=proto --proto_path=proto ./proto/common.proto ./proto/snap.proto

import (
	"fmt"
	"io"
	"reflect"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/p2p/snap/p2pproto"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
)

const Proto = "/juno/starknet/snap-sync/1"

type SnapSyncServer struct {
	snapServer blockchain.SnapServer
	log        utils.SimpleLogger
}

func NewSnapSyncServer(server blockchain.SnapServer, log utils.SimpleLogger) *SnapSyncServer {
	return &SnapSyncServer{
		snapServer: server,
		log:        log,
	}
}

func (s *SnapSyncServer) HandleSnapSyncRequest(request *p2pproto.SnapRequest) (*p2pproto.SnapResponse, error) {
	switch v := request.Request.(type) {

	case *p2pproto.SnapRequest_GetAddressRange:
		response, err := s.HandleAddressRange(v.GetAddressRange)
		return &p2pproto.SnapResponse{
			Response: &p2pproto.SnapResponse_AddressRange{
				AddressRange: response,
			},
		}, err
	default:
		return nil, fmt.Errorf("unexpected request type %t", v)
	}
}

func (s *SnapSyncServer) HandleStream(stream network.Stream) {
	err := s.DoHandleStream(stream)
	if err != nil {
		s.log.Errorw("error handling block sync", err)
	}
	err = stream.Close()
	if err != nil {
		s.log.Errorw("error closing stream", err)
	}
}

func (s *SnapSyncServer) DoHandleStream(stream io.ReadWriteCloser) error {
	msg := p2pproto.SnapRequest{}
	err := readCompressedProtobuf(stream, &msg)
	if err != nil {
		return err
	}

	s.log.Infow("Handling snap sync", "type", reflect.TypeOf(msg.Request))
	resp, err := s.HandleSnapSyncRequest(&msg)
	if err != nil {
		return err
	}

	err = writeCompressedProtobuf(stream, resp)
	if err != nil {
		return err
	}

	return nil
}

func (s *SnapSyncServer) HandleAddressRange(addressRange *p2pproto.GetAddressRange) (*p2pproto.AddressRange, error) {
	response, err := s.snapServer.GetAddressRange(
		fieldElementToFelt(addressRange.Root),
		fieldElementToFelt(addressRange.StartAddr),
		fieldElementToFelt(addressRange.LimitAddr),
		addressRange.MaxNodes,
	)
	if err == blockchain.ErrMissingSnapshot {
		return nil, nil // Hmm....
	}
	if err != nil {
		return nil, err
	}

	return MapValueViaReflect[*p2pproto.AddressRange](response), nil
}
