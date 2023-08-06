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
	case *p2pproto.SnapRequest_GetTrieRoot:
		response, err := s.HandleTrieRootRequest(v.GetTrieRoot)
		return &p2pproto.SnapResponse{
			Response: &p2pproto.SnapResponse_RootInfo{
				RootInfo: response,
			},
		}, err
	case *p2pproto.SnapRequest_GetClassRange:
		response, err := s.HandleClassRangeRequest(v.GetClassRange)
		return &p2pproto.SnapResponse{
			Response: &p2pproto.SnapResponse_ClassRange{
				ClassRange: response,
			},
		}, err
	case *p2pproto.SnapRequest_GetContractRange:
		response, err := s.HandleContractRange(v.GetContractRange)
		return &p2pproto.SnapResponse{
			Response: &p2pproto.SnapResponse_ContractRange{
				ContractRange: response,
			},
		}, err
	case *p2pproto.SnapRequest_GetAddressRange:
		response, err := s.HandleAddressRange(v.GetAddressRange)
		return &p2pproto.SnapResponse{
			Response: &p2pproto.SnapResponse_AddressRange{
				AddressRange: response,
			},
		}, err
	case *p2pproto.SnapRequest_GetClasses:
		response, err := s.HandleGetClasses(v.GetClasses)
		return &p2pproto.SnapResponse{
			Response: &p2pproto.SnapResponse_Classes{
				Classes: response,
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

func (s *SnapSyncServer) HandleTrieRootRequest(root *p2pproto.GetRootInfo) (*p2pproto.RootInfo, error) {
	info, err := s.snapServer.GetTrieRootAt(fieldElementToFelt(root.BlockHash))
	if err == blockchain.ErrMissingSnapshot {
		return nil, nil // Hmm....
	}
	if err != nil {
		return nil, err
	}

	return MapValueViaReflect[*p2pproto.RootInfo](info), nil
}

func (s *SnapSyncServer) HandleClassRangeRequest(classRange *p2pproto.GetClassRange) (*p2pproto.ClassRange, error) {
	response, err := s.snapServer.GetClassRange(fieldElementToFelt(classRange.Root), fieldElementToFelt(classRange.StartAddr), fieldElementToFelt(classRange.LimitAddr), classRange.MaxNodes)
	if err == blockchain.ErrMissingSnapshot {
		return nil, nil // Hmm....
	}
	if err != nil {
		return nil, err
	}

	return &p2pproto.ClassRange{
		Paths:            feltsToFieldElements(response.Paths),
		ClassCommitments: feltsToFieldElements(response.ClassCommitments),
		Proofs:           MapValueViaReflect[[]*p2pproto.ProofNode](response.Proofs),
	}, nil
}

func (s *SnapSyncServer) HandleContractRange(contractRangeRequest *p2pproto.GetContractRange) (*p2pproto.ContractRange, error) {
	root := fieldElementToFelt(contractRangeRequest.Root)
	requests := MapValueViaReflect[[]*blockchain.StorageRangeRequest](contractRangeRequest.Requests)

	response, err := s.snapServer.GetContractRange(root, requests, contractRangeRequest.MaxNodes, contractRangeRequest.MaxNodesPerContract)
	if err == blockchain.ErrMissingSnapshot {
		return nil, nil // Hmm....
	}
	if err != nil {
		return nil, err
	}

	return &p2pproto.ContractRange{
		Responses: MapValueViaReflect[[]*p2pproto.ContractRangeResponse](response),
	}, nil
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

func (s *SnapSyncServer) HandleGetClasses(classes *p2pproto.GetClasses) (*p2pproto.Classes, error) {
	keys := fieldElementsToFelts(classes.Hashes)
	response, err := s.snapServer.GetClasses(keys)
	if err != nil {
		return nil, err
	}

	protoclasses := make([]*p2pproto.ContractClass, 0)
	for i, class := range response {
		protoclass, err := coreUndeclaredClassToProtobufClass(keys[i], class)
		if err != nil {
			return nil, err
		}

		protoclasses = append(protoclasses, protoclass)
	}

	return &p2pproto.Classes{Classes: protoclasses}, nil
}
