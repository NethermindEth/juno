package starknet

import (
	"fmt"
	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/iter"
	"google.golang.org/protobuf/proto"
)

type SnapProvider struct {
	SnapServer sync.SnapServer
}

func (h *Handler) onClassHashesRequest(req *spec.ClassHashesRequest) (iter.Seq[proto.Message], error) {
	type yieldFunc = func(proto.Message) bool
	finMsg := &spec.ClassesResponse{
		ClassMessage: &spec.ClassesResponse_Fin{},
	}

	return func(yield yieldFunc) {
		if req == nil || req.ClassHashes == nil || len(req.ClassHashes) == 0 {
			yield(finMsg)
			return
		}

		// Since we return iterator we may split given keys into smaller chunks if necessary
		classHashes := make([]*felt.Felt, len(req.ClassHashes))
		for _, hash := range req.ClassHashes {
			classHashes = append(classHashes, p2p2core.AdaptHash(hash))
		}

		classes, err := h.snapProvider.SnapServer.GetClasses(h.ctx, classHashes)
		if err != nil {
			h.log.Errorw("failed to get classes", "err", err)
			return
		}

		for _, cls := range classes {
			msg := &spec.ClassesResponse{
				ClassMessage: &spec.ClassesResponse_Class{
					Class: cls,
				},
			}
			if !yield(msg) {
				// if caller is not interested in remaining data (example: connection to a peer is closed) exit
				// note that in this case we won't send finMsg
				return
			}
		}

		yield(finMsg)
	}, nil
}

func (h *Handler) onClassRangeRequest(req *spec.ClassRangeRequest) (iter.Seq[proto.Message], error) {
	if h.snapProvider == nil {
		return nil, fmt.Errorf("snapsyncing not supported")
	}

	finMsg := &spec.ClassRangeResponse{
		Responses: &spec.ClassRangeResponse_Fin{},
	}

	return func(yield func(message proto.Message) bool) {
		// port snap_server here
		yield(finMsg)
	}, nil
}

func (h *Handler) onContractRangeRequest(req *spec.ContractRangeRequest) (iter.Seq[proto.Message], error) {
	if h.snapProvider == nil {
		return nil, fmt.Errorf("snapsyncing not supported")
	}

	finMsg := &spec.ContractRangeResponse{
		Responses: &spec.ContractRangeResponse_Fin{},
	}

	return func(yield func(message proto.Message) bool) {
		// port snap_server here
		yield(finMsg)
	}, nil
}

func (h *Handler) onContractStorageRequest(req *spec.ContractStorageRequest) (iter.Seq[proto.Message], error) {
	if h.snapProvider == nil {
		return nil, fmt.Errorf("snapsyncing not supported")
	}

	finMsg := &spec.ContractStorageResponse{
		Responses: &spec.ContractStorageResponse_Fin{},
	}
	return func(yield func(message proto.Message) bool) {
		// TODO: adapter method? Do we need separate req/res structs?
		srr := &sync.StorageRangeRequest{
			StateRoot:     p2p2core.AdaptHash(req.StateRoot),
			ChunkPerProof: uint64(req.ChunksPerProof),
			Queries:       req.Query,
		}
		stoIter, err := h.snapProvider.SnapServer.GetStorageRange(h.ctx, srr)
		if err != nil {
			h.log.Errorw("failed to get storage range", "err", err)
			return
		}

		stoIter(func(result *sync.StorageRangeStreamingResult) bool {
			// TODO: again adapter or just return spec types?
			res := &spec.ContractStorageResponse{
				StateRoot:       req.StateRoot,
				ContractAddress: core2p2p.AdaptFelt(result.StorageAddr),
				Responses: &spec.ContractStorageResponse_Storage{
					Storage: &spec.ContractStorage{
						KeyValue: result.Range,
					},
				},
			}

			if !yield(res) {
				return false
			}

			return true
		})

		yield(finMsg)
	}, nil
}
