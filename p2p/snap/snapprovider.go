package snap

import (
	"context"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/snap/p2pproto"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/miolini/datacounter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type SnapProvider struct {
	streamProvider streamProvider
	logger         utils.SimpleLogger
}

type streamProvider = func(ctx context.Context) (network.Stream, func(), error)

var _ blockchain.SnapServer = &SnapProvider{}

var snapDataTotals = promauto.NewCounter(prometheus.CounterOpts{
	Name: "juno_snap_data_totals",
	Help: "Time in address get",
})

func NewSnapProvider(
	streamProvider streamProvider,
	logger utils.SimpleLogger,
) (*SnapProvider, error) {
	peerManager := &SnapProvider{
		streamProvider: streamProvider,
		logger:         logger,
	}

	return peerManager, nil
}

func (ip *SnapProvider) GetAddressRange(
	rootHash, startAddr, limitAddr *felt.Felt,
	maxNodes uint64,
) (*blockchain.AddressRangeResult, error) {
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
	countingReader := datacounter.NewReaderCounter(stream)
	err = readCompressedProtobuf(countingReader, resp)
	snapDataTotals.Add(float64(countingReader.Count()))
	if err != nil {
		return nil, err
	}

	return resp, nil
}
