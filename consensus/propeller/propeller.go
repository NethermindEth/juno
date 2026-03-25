package propeller

import (
	"bytes"
	"context"
	"io"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"go.uber.org/zap"
)

const propellerProtocolID = "/propeller/0.0.1"

// This would represent the propeller service that glues the whole
// thing to p2p. Thing is, I've no clue how to do that.
type Service interface{}

type propellerService struct {
	host   host.Host
	engine *Engine
	config Config
	log    utils.Logger
}

func New(
	host host.Host,
	privKey crypto.PrivKey,
	config *Config,
	log utils.Logger,
) Service {
	engine := NewEngine(
		privKey,
		config,
		nil,
		log,
	)

	return &propellerService{
		host:   host,
		engine: engine,
		config: *config,
		log:    log,
	}
}

func (s *propellerService) Run(ctx context.Context) {
}

func (s *propellerService) handleInboudStream(stream network.Stream) {
	defer stream.Close()

	sender := stream.Conn().RemotePeer()

	reader := io.LimitReader(stream, int64(s.config.MaxWireMessageSize))

	var buf bytes.Buffer
	_, err := buf.ReadFrom(reader)
	if err != nil {
		s.log.Debug("error reading inbound propeller stream",
			zap.Stringer("peer", sender),
			zap.Error(err),
		)
	}
}
