package propeller

import (
	"bytes"
	"context"
	"io"

	pb "github.com/NethermindEth/juno/consensus/propeller/proto"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// This would represent the propeller service that glues the whole
// thing to p2p. Thing is, I've no clue how to do that.
type Service interface{}

type propellerService struct {
	host   host.Host
	engine *Engine
	cmdCh  chan<- engineCommand
	config Config
	log    utils.Logger
}

func New(
	host host.Host,
	privKey crypto.PrivKey,
	config *Config,
	log utils.Logger,
) Service {
	engine, cmdCh := NewEngine(
		privKey,
		config,
		log,
	)

	return &propellerService{
		host:   host,
		engine: engine,
		cmdCh:  cmdCh,
		config: *config,
		log:    log,
	}
}

func (s *propellerService) receivePropellerUnits(stream network.Stream) {
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

	var batch pb.PropellerUnitBatch
	err = proto.Unmarshal(buf.Bytes(), &batch)
	if err != nil {
		s.log.Debug("error unmarshalling propeller batch",
			zap.Stringer("peer", sender),
			zap.Error(err),
		)
	}

	for _, protoUnit := range batch.GetBatch() {
		unit, err := UnitFromProto(protoUnit)
		if err != nil {
			s.log.Warn("received invalid unit", zap.Error(err))
			// todo(rdr): penalize sender?
			continue
		}
		// send unit to engine
		s.cmdCh <- processUnit{
			&unit,
			sender,
		}
	}
}

func (s *propellerService) Run(ctx context.Context) error {
	go func() {
		err := s.engine.Run(ctx)
		if err != nil {
			s.log.Error("shutting down propeller engine", zap.Error(err))
			return
		}
		s.log.Info("shutting down propeller engine")
	}()

	s.host.SetStreamHandler(s.config.StreamProtocol, s.receivePropellerUnits)
	defer s.host.RemoveStreamHandler(s.config.StreamProtocol)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		}
		// todo(rdr): handle the engines output such as units to broadcast
	}
}

func (s *propellerService) broadcast() {
}
