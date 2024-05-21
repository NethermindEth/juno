package snap

//go:generate protoc --go_out=proto --proto_path=proto ./proto/common.proto ./proto/snap.proto

import (
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/snapsync/p2pproto"
	"github.com/NethermindEth/juno/utils"
)

const Proto = "/juno/starknet/snap-sync/1"

type SnapSyncServer struct {
	snapServer     blockchain.SnapServer
	log            utils.SimpleLogger
	pivotBlockHash *felt.Felt
}

func NewSnapSyncServer(server blockchain.SnapServer, log utils.SimpleLogger, pivotBlockHash *felt.Felt) *SnapSyncServer {
	return &SnapSyncServer{
		snapServer:     server,
		log:            log,
		pivotBlockHash: pivotBlockHash,
	}
}

func (s *SnapSyncServer) HandleSnapSyncRequest(request *p2pproto.SnapRequest) (*p2pproto.SnapResponse, error) {
	switch v := request.Request.(type) {
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

func (s *SnapSyncServer) HandleGetClasses(classes *p2pproto.GetClasses) (*p2pproto.Classes, error) {
	keys := fieldElementsToFelts(classes.Hashes)
	response, err := s.snapServer.GetClasses(s.pivotBlockHash, keys)
	if err != nil {
		return nil, err
	}

	protoclasses := make([]*p2pproto.ContractClass, 0)
	for i, class := range response {
		protoclass, err := coreClassToProto(keys[i], class)
		if err != nil {
			return nil, err
		}

		protoclasses = append(protoclasses, protoclass)
	}

	return &p2pproto.Classes{Classes: protoclasses}, nil
}
