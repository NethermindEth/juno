package p2p

import (
	"math/big"

	"github.com/NethermindEth/juno/utils"
	"google.golang.org/protobuf/proto"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils/iter"
	"github.com/ethereum/go-ethereum/log"
)

type ContractRangeStreamingResult struct {
	ContractsRoot *felt.Felt
	ClassesRoot   *felt.Felt
	Range         []*spec.ContractState
	RangeProof    *spec.PatriciaRangeProof
}

type StorageRangeRequest struct {
	StateRoot     *felt.Felt
	ChunkPerProof uint64 // Missing in spec
	Queries       []*spec.StorageRangeQuery
}

type StorageRangeStreamingResult struct {
	ContractsRoot *felt.Felt
	ClassesRoot   *felt.Felt
	StorageAddr   *felt.Felt
	Range         []*spec.ContractStoredValue
	RangeProof    *spec.PatriciaRangeProof
}

type ClassRangeStreamingResult struct {
	ContractsRoot *felt.Felt
	ClassesRoot   *felt.Felt
	Range         *spec.Classes
	RangeProof    *spec.PatriciaRangeProof
}

type SnapServerBlockchain interface {
	GetStateForStateRoot(stateRoot *felt.Felt) (*core.State, error)
	GetClasses(felts []*felt.Felt) ([]core.Class, error)
}

type yieldFunc = func(proto.Message) bool

var _ SnapServerBlockchain = (*blockchain.Blockchain)(nil)

func NewSnapServer(blockchain SnapServerBlockchain, log utils.SimpleLogger) *snapServer {
	return &snapServer{
		log:        log,
		blockchain: blockchain,
	}
}

type snapServer struct {
	log        utils.SimpleLogger
	blockchain SnapServerBlockchain
}

func determineMaxNodes(specifiedMaxNodes uint32) uint32 {
	const (
		defaultMaxNodes   = 1024 * 16
		maxNodePerRequest = 1024 * 1024 // I just want it to process faster
	)

	if specifiedMaxNodes == 0 {
		return defaultMaxNodes
	}

	if specifiedMaxNodes < maxNodePerRequest {
		return specifiedMaxNodes
	}

	return maxNodePerRequest
}

func (b *snapServer) GetClassRange(request *spec.ClassRangeRequest) (iter.Seq[proto.Message], error) {
	var finMsg proto.Message = &spec.ClassRangeResponse{
		Responses: &spec.ClassRangeResponse_Fin{},
	}

	stateRoot := p2p2core.AdaptHash(request.Root)
	startAddr := p2p2core.AdaptHash(request.Start)
	b.log.Debugw("GetClassRange", "start", startAddr, "chunks", request.ChunksPerProof)

	return func(yield yieldFunc) {
		s, err := b.blockchain.GetStateForStateRoot(stateRoot)
		if err != nil {
			log.Error("error getting state for state root", "err", err)
			return
		}

		contractRoot, classRoot, err := s.StateAndClassRoot()
		if err != nil {
			log.Error("error getting state and class root", "err", err)
			return
		}

		ctrie, classCloser, err := s.ClassTrie()
		if err != nil {
			log.Error("error getting class trie", "err", err)
			return
		}
		defer func() { _ = classCloser() }()

		startAddr := p2p2core.AdaptHash(request.Start)
		limitAddr := p2p2core.AdaptHash(request.End)
		if limitAddr != nil && limitAddr.IsZero() {
			limitAddr = nil
		}

		for {
			response := &spec.Classes{
				Classes: make([]*spec.Class, 0),
			}

			classkeys := []*felt.Felt{}
			proofs, finished, err := ctrie.IterateWithLimit(startAddr, limitAddr, determineMaxNodes(request.ChunksPerProof), b.log,
				func(key, value *felt.Felt) error {
					classkeys = append(classkeys, key)
					return nil
				})
			if err != nil {
				log.Error("error iterating class trie", "err", err)
				return
			}

			coreClasses, err := b.blockchain.GetClasses(classkeys)
			if err != nil {
				log.Error("error getting classes", "err", err)
				return
			}

			for _, coreclass := range coreClasses {
				if coreclass == nil {
					log.Error("nil class in the returned array of core classes")
					return
				}
				response.Classes = append(response.Classes, core2p2p.AdaptClass(coreclass))
			}

			clsMsg := &spec.ClassRangeResponse{
				ContractsRoot: core2p2p.AdaptHash(contractRoot),
				ClassesRoot:   core2p2p.AdaptHash(classRoot),
				Responses: &spec.ClassRangeResponse_Classes{
					Classes: response,
				},
				RangeProof: Core2P2pProof(proofs),
			}

			first := classkeys[0]
			last := classkeys[len(classkeys)-1]
			b.log.Infow("sending class range response", "len(classes)", len(classkeys), "first", first, "last", last)
			if !yield(clsMsg) {
				// we should not send `FinMsg` when the client explicitly asks to stop
				return
			}
			if finished {
				break
			}
			startAddr = classkeys[len(classkeys)-1]
		}

		yield(finMsg)
		b.log.Infow("GetClassRange iteration completed")
	}, nil
}

func (b *snapServer) GetContractRange(request *spec.ContractRangeRequest) (iter.Seq[proto.Message], error) {
	var finMsg proto.Message = &spec.ContractRangeResponse{
		Responses: &spec.ContractRangeResponse_Fin{},
	}
	stateRoot := p2p2core.AdaptHash(request.StateRoot)
	startAddr := p2p2core.AdaptAddress(request.Start)
	b.log.Debugw("GetContractRange", "root", stateRoot, "start", startAddr, "chunks", request.ChunksPerProof)

	return func(yield yieldFunc) {
		s, err := b.blockchain.GetStateForStateRoot(stateRoot)
		if err != nil {
			log.Error("error getting state for state root", "err", err)
			return
		}

		contractRoot, classRoot, err := s.StateAndClassRoot()
		if err != nil {
			log.Error("error getting state and class root", "err", err)
			return
		}

		strie, scloser, err := s.StorageTrie()
		if err != nil {
			log.Error("error getting storage trie", "err", err)
			return
		}
		defer func() { _ = scloser() }()

		startAddr := p2p2core.AdaptAddress(request.Start)
		limitAddr := p2p2core.AdaptAddress(request.End)
		states := []*spec.ContractState{}

		for {
			proofs, finished, err := strie.IterateWithLimit(startAddr, limitAddr, determineMaxNodes(request.ChunksPerProof), b.log,
				func(key, value *felt.Felt) error {
					classHash, err := s.ContractClassHash(key)
					if err != nil {
						return err
					}

					nonce, err := s.ContractNonce(key)
					if err != nil {
						return err
					}

					ctr, err := s.StorageTrieForAddr(key)
					if err != nil {
						return err
					}

					croot, err := ctr.Root()
					if err != nil {
						return err
					}

					startAddr = key
					states = append(states, &spec.ContractState{
						Address: core2p2p.AdaptAddress(key),
						Class:   core2p2p.AdaptHash(classHash),
						Storage: core2p2p.AdaptHash(croot),
						Nonce:   nonce.Uint64(),
					})
					return nil
				})
			if err != nil {
				log.Error("error iterating storage trie", "err", err)
				return
			}

			cntrMsg := &spec.ContractRangeResponse{
				Root:          request.StateRoot,
				ContractsRoot: core2p2p.AdaptHash(contractRoot),
				ClassesRoot:   core2p2p.AdaptHash(classRoot),
				RangeProof:    Core2P2pProof(proofs),
				Responses: &spec.ContractRangeResponse_Range{
					Range: &spec.ContractRange{
						State: states,
					},
				},
			}

			var first, last *felt.Felt
			if len(states) > 0 {
				first = p2p2core.AdaptAddress(states[0].Address)
				last = p2p2core.AdaptAddress(states[len(states)-1].Address)
			}
			b.log.Infow("sending contract range response", "len(states)", len(states), "first", first, "last", last)
			if !yield(cntrMsg) {
				// we should not send `FinMsg` when the client explicitly asks to stop
				return
			}
			if finished {
				break
			}

			states = states[:0]
		}

		yield(finMsg)
		b.log.Infow("contract range iteration completed")
	}, nil
}

func (b *snapServer) GetStorageRange(request *spec.ContractStorageRequest) (iter.Seq[proto.Message], error) {
	var finMsg proto.Message = &spec.ContractStorageResponse{
		Responses: &spec.ContractStorageResponse_Fin{},
	}
	startKey := p2p2core.AdaptAddress(request.Query[0].Address)
	last := len(request.Query) - 1
	endKey := p2p2core.AdaptAddress(request.Query[last].Address)
	b.log.Debugw("GetStorageRange", "query[0]", startKey, "query[", last, "]", endKey)

	return func(yield yieldFunc) {
		stateRoot := p2p2core.AdaptHash(request.StateRoot)

		s, err := b.blockchain.GetStateForStateRoot(stateRoot)
		if err != nil {
			log.Error("error getting state for state root", "err", err)
			return
		}

		var curNodeLimit uint32 = 1000000

		// shouldContinue is a return value from the yield function which specify whether the iteration should continue
		var shouldContinue bool = true
		for _, query := range request.Query {
			contractLimit := curNodeLimit

			strie, err := s.StorageTrieForAddr(p2p2core.AdaptAddress(query.Address))
			if err != nil {
				addr := p2p2core.AdaptAddress(query.Address)
				log.Error("error getting storage trie for address", "addr", addr, "err", err)
				return
			}

			handled, err := b.handleStorageRangeRequest(strie, query, request.ChunksPerProof, contractLimit, b.log,
				func(values []*spec.ContractStoredValue, proofs []trie.ProofNode) bool {
					stoMsg := &spec.ContractStorageResponse{
						StateRoot:       request.StateRoot,
						ContractAddress: query.Address,
						RangeProof:      Core2P2pProof(proofs),
						Responses: &spec.ContractStorageResponse_Storage{
							Storage: &spec.ContractStorage{
								KeyValue: values,
							},
						},
					}
					if shouldContinue = yield(stoMsg); !shouldContinue {
						return false
					}
					return true
				})
			if err != nil {
				log.Error("error handling storage range request", "err", err)
				return
			}

			curNodeLimit -= handled

			if curNodeLimit <= 0 {
				break
			}
		}
		if shouldContinue {
			// we should `Fin` only when client expects iteration to continue
			yield(finMsg)
		}
	}, nil
}

func (b *snapServer) GetClasses(request *spec.ClassHashesRequest) (iter.Seq[proto.Message], error) {
	var finMsg proto.Message = &spec.ClassesResponse{
		ClassMessage: &spec.ClassesResponse_Fin{},
	}
	b.log.Debugw("GetClasses", "len(hashes)", len(request.ClassHashes))

	return func(yield yieldFunc) {
		felts := make([]*felt.Felt, len(request.ClassHashes))
		for i, hash := range request.ClassHashes {
			felts[i] = p2p2core.AdaptHash(hash)
		}

		coreClasses, err := b.blockchain.GetClasses(felts)
		if err != nil {
			log.Error("error getting classes", "err", err)
			return
		}

		for _, cls := range coreClasses {
			clsMsg := &spec.ClassesResponse{
				ClassMessage: &spec.ClassesResponse_Class{
					Class: core2p2p.AdaptClass(cls),
				},
			}
			if !yield(clsMsg) {
				// we should not send `FinMsg` when the client explicitly asks to stop
				return
			}
		}

		yield(finMsg)
	}, nil
}

func (b *snapServer) handleStorageRangeRequest(
	stTrie *trie.Trie,
	request *spec.StorageRangeQuery,
	maxChunkPerProof, nodeLimit uint32,
	logger utils.SimpleLogger,
	yield func([]*spec.ContractStoredValue, []trie.ProofNode) bool,
) (uint32, error) {
	totalSent := 0
	finished := false
	startAddr := p2p2core.AdaptFelt(request.Start.Key)
	var endAddr *felt.Felt = nil
	if request.End != nil {
		endAddr = p2p2core.AdaptFelt(request.End.Key)
	}

	for !finished {
		response := []*spec.ContractStoredValue{}

		limit := maxChunkPerProof
		if nodeLimit < limit {
			limit = nodeLimit
		}

		proofs, finish, err := stTrie.IterateWithLimit(startAddr, endAddr, limit, logger,
			func(key, value *felt.Felt) error {
				response = append(response, &spec.ContractStoredValue{
					Key:   core2p2p.AdaptFelt(key),
					Value: core2p2p.AdaptFelt(value),
				})

				startAddr = key
				return nil
			})
		finished = finish

		if err != nil {
			return 0, err
		}

		if len(response) == 0 {
			finished = true
		}

		if !yield(response, proofs) {
			finished = true
		}

		totalSent += len(response)
		nodeLimit -= limit

		asBint := startAddr.BigInt(big.NewInt(0))
		asBint = asBint.Add(asBint, big.NewInt(1))
		startAddr = startAddr.SetBigInt(asBint)
	}

	return uint32(totalSent), nil
}

func Core2P2pProof(proofs []trie.ProofNode) *spec.PatriciaRangeProof {
	nodes := make([]*spec.PatriciaNode, len(proofs))

	for i := range proofs {
		if proofs[i].Binary != nil {
			binary := proofs[i].Binary
			nodes[i] = &spec.PatriciaNode{
				Node: &spec.PatriciaNode_Binary_{
					Binary: &spec.PatriciaNode_Binary{
						Left:  core2p2p.AdaptFelt(binary.LeftHash),
						Right: core2p2p.AdaptFelt(binary.RightHash),
					},
				},
			}
		}
		if proofs[i].Edge != nil {
			edge := proofs[i].Edge
			pathfeld := edge.Path.Felt()
			nodes[i] = &spec.PatriciaNode{
				Node: &spec.PatriciaNode_Edge_{
					Edge: &spec.PatriciaNode_Edge{
						Length: uint32(edge.Path.Len()),
						Path:   core2p2p.AdaptFelt(&pathfeld),
						Value:  core2p2p.AdaptFelt(edge.Child),
					},
				},
			}
		}
	}

	return &spec.PatriciaRangeProof{
		Nodes: nodes,
	}
}
