package snap

//go:generate protoc --go_out=proto --proto_path=proto ./proto/common.proto ./proto/snap.proto

import (
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/snapsync/p2pproto"
	"github.com/NethermindEth/juno/utils"
	"github.com/pkg/errors"
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
	response, err := s.snapServer.GetClasses(keys)
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

func coreClassToProto(hash *felt.Felt, class core.Class) (*p2pproto.ContractClass, error) {
	if class == nil {
		return nil, nil
	}
	switch class := class.(type) {
	case *core.Cairo0Class:
		abistr, err := class.Abi.MarshalJSON()
		if err != nil {
			return nil, err
		}

		return &p2pproto.ContractClass{
			Class: &p2pproto.ContractClass_Cairo0{
				Cairo0: &p2pproto.Cairo0Class{
					ConstructorEntryPoints: MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.Constructors),
					ExternalEntryPoints:    MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.Externals),
					L1HandlerEntryPoints:   MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.L1Handlers),
					Program:                class.Program,
					Abi:                    string(abistr),
					Hash:                   feltToFieldElement(hash),
				},
			},
		}, nil
	case *core.Cairo1Class:
		return &p2pproto.ContractClass{
			Class: &p2pproto.ContractClass_Cairo1{
				Cairo1: &p2pproto.Cairo1Class{
					ConstructorEntryPoints: MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.Constructor),
					ExternalEntryPoints:    MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.External),
					L1HandlerEntryPoints:   MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.L1Handler),
					Program:                feltsToFieldElements(class.Program),
					ProgramHash:            feltToFieldElement(class.ProgramHash),
					Abi:                    class.Abi,
					Hash:                   feltToFieldElement(hash),
					SemanticVersioning:     class.SemanticVersion,
				},
			},
		}, nil
	default:
		return nil, errors.Errorf("unsupported class type %T", class)
	}
}
