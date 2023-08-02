package p2p

import (
	"context"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/p2p/p2pproto"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
)

type SnapProvider struct {
	streamProvider streamProvider
	logger         utils.SimpleLogger

	converter *converter
	verifier  *verifier
}

var _ blockchain.SnapServer = &SnapProvider{}

func NewSnapProvider(
	streamProvider streamProvider,
	bc *blockchain.Blockchain,
	logger utils.SimpleLogger,
) (*SnapProvider, error) {
	converter := NewConverter(&blockchainClassProvider{
		blockchain: bc,
	})

	peerManager := &SnapProvider{
		streamProvider: streamProvider,
		converter:      converter,
		verifier: &verifier{
			network: bc.Network(),
		},
		logger: logger,
	}

	return peerManager, nil
}

func (ip *SnapProvider) GetTrieRootAt(blockHash *felt.Felt) (*blockchain.TrieRootInfo, error) {
	ctx := context.Background()
	request := &p2pproto.SnapRequest{
		Request: &p2pproto.SnapRequest_GetTrieRoot{
			GetTrieRoot: &p2pproto.GetRootInfo{
				BlockHash: feltToFieldElement(blockHash),
			},
		},
	}

	response, err := ip.sendSnapRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	return MapValueViaReflect[*blockchain.TrieRootInfo](response.GetRootInfo()), nil
}

func (ip *SnapProvider) GetClassRange(classTrieRootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt, maxNodes uint64) (*blockchain.ClassRangeResult, error) {
	ctx := context.Background()
	request := &p2pproto.SnapRequest{
		Request: &p2pproto.SnapRequest_GetClassRange{
			GetClassRange: &p2pproto.GetClassRange{
				Root:      feltToFieldElement(classTrieRootHash),
				StartAddr: feltToFieldElement(startAddr),
				LimitAddr: feltToFieldElement(limitAddr),
				MaxNodes:  maxNodes,
			},
		},
	}

	response, err := ip.sendSnapRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	protoclassrange := response.GetClassRange()

	return &blockchain.ClassRangeResult{
		Paths:            fieldElementsToFelts(protoclassrange.Paths),
		ClassCommitments: fieldElementsToFelts(protoclassrange.ClassCommitments),
		Proofs:           MapValueViaReflect[[]*trie.ProofNode](protoclassrange.Proofs),
	}, nil
}

func (ip *SnapProvider) GetClasses(classes []*felt.Felt) ([]core.Class, error) {
	ctx := context.Background()
	request := &p2pproto.SnapRequest{
		Request: &p2pproto.SnapRequest_GetClasses{
			GetClasses: &p2pproto.GetClasses{
				Hashes: feltsToFieldElements(classes),
			},
		},
	}

	response, err := ip.sendSnapRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	protoclasses := response.GetClasses()

	coreclasses := make([]core.Class, 0)
	for _, class := range protoclasses.Classes {
		_, cls, err := ip.converter.protobufClassToCoreClass(class)
		if err != nil {
			return nil, err
		}
		coreclasses = append(coreclasses, cls)
	}

	return coreclasses, nil
}

func (ip *SnapProvider) GetAddressRange(rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt, maxNodes uint64) (*blockchain.AddressRangeResult, error) {
	ctx := context.Background()
	request := &p2pproto.SnapRequest{
		Request: &p2pproto.SnapRequest_GetAddressRange{
			GetAddressRange: &p2pproto.GetAddressRange{
				Root:      feltToFieldElement(rootHash),
				StartAddr: feltToFieldElement(startAddr),
				LimitAddr: feltToFieldElement(limitAddr),
				MaxNodes:  maxNodes,
			},
		},
	}

	response, err := ip.sendSnapRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	return MapValueViaReflect[*blockchain.AddressRangeResult](response.GetAddressRange()), nil
}

func (ip *SnapProvider) GetContractRange(storageTrieRootHash *felt.Felt, requests []*blockchain.StorageRangeRequest, maxNodes uint64) ([]*blockchain.StorageRangeResult, error) {
	ctx := context.Background()
	request := &p2pproto.SnapRequest{
		Request: &p2pproto.SnapRequest_GetContractRange{
			GetContractRange: &p2pproto.GetContractRange{
				Root:     feltToFieldElement(storageTrieRootHash),
				Requests: MapValueViaReflect[[]*p2pproto.ContractRangeRequest](requests),
				MaxNodes: maxNodes,
			},
		},
	}

	response, err := ip.sendSnapRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	return MapValueViaReflect[[]*blockchain.StorageRangeResult](response.GetContractRange().Responses), nil
}

func (ip *SnapProvider) sendSnapRequest(ctx context.Context, request *p2pproto.SnapRequest) (*p2pproto.SnapResponse, error) {
	stream, closeFunc, err := ip.streamProvider(ctx)
	if err != nil {
		return nil, err
	}

	defer closeFunc()

	defer func(stream network.Stream) {
		err = stream.Close()
		if err != nil {
			ip.logger.Warnw("error closing stream", "error", err)
		}
	}(stream)

	err = writeCompressedProtobuf(stream, request)
	if err != nil {
		return nil, err
	}
	err = stream.CloseWrite()
	if err != nil {
		return nil, err
	}

	resp := &p2pproto.SnapResponse{}
	err = readCompressedProtobuf(stream, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
