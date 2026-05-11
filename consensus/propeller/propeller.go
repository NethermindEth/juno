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
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// This would represent the propeller service that glues the whole
// thing to p2p. Thing is, I've no clue how to do that.
type Service any

type propellerService struct {
	// P2P config
	host host.Host
	// Internal config
	config Config
	log    utils.Logger
	// Propeller communication
	engine   *Engine
	cmdCh    chan<- engineCommand
	eventsCh <-chan Event
	// External communication
	messageRecv chan []byte
}

func New(
	host host.Host,
	privKey crypto.PrivKey,
	config *Config,
	log utils.Logger,
) Service {
	engine, cmdCh, eventsCh := NewEngine(
		privKey,
		config,
		log,
	)

	return &propellerService{
		host:     host,
		engine:   engine,
		cmdCh:    cmdCh,
		eventsCh: eventsCh,
		config:   *config,
		log:      log,
	}
}

func (s *propellerService) receiveUnits(stream network.Stream) {
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
			// If we do it here then it means it shouldn't be handled at
			// subP or Processor level, and all should be handled here,
			// which means, every invalid unit should be handled at Service
			// level. To be determined yet.
			continue
		}
		// send unit to engine
		s.cmdCh <- processUnit{
			&unit,
			sender,
		}
	}
}

func (s *propellerService) broadcastUnit(ctx context.Context, unit *Unit, peers []peer.ID) {
	batch := &pb.PropellerUnitBatch{
		Batch: []*pb.PropellerUnit{unit.ToProto()},
	}
	data, err := proto.Marshal(batch)
	if err != nil {
		// todo(rdr): log the error? What if this cannot get it done?
		// Our batch is correct unless there is an internal bug
		panic(err)
	}

	for _, p := range peers {
		err = s.sendToPeer(ctx, p, data)
		if err != nil {
			// Why would there be any error
			// Based on the error type, what should we do
			panic(err)
		}
	}
}

func (s *propellerService) sendToPeer(ctx context.Context, p peer.ID, data []byte) error {
	stream, err := s.host.NewStream(ctx, p, s.config.StreamProtocol)
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = stream.Write(data)
	return err
}

func (s *propellerService) broadcastMessage(ctx context.Context, msg []byte) {
}

func (s *propellerService) handleEvent(ctx context.Context, event Event) {
	switch event := event.(type) {
	case *messageFinalized:
		// if the message is finalized it should have a receive
		s.messageRecv <- event.message
	case *broadcastUnit:
		s.broadcastUnit(ctx, event.unit, event.peers)
	case *broadcastMessage:
		s.broadcastMessage(ctx, event.unit)
	}
}

func (s *propellerService) Run(ctx context.Context) error {
	// Start engine service in the background
	go func() {
		err := s.engine.Run(ctx)
		if err != nil {
			s.log.Error("shutting down propeller engine", zap.Error(err))
			return
		}
		s.log.Info("shutting down propeller engine")
	}()

	// Subscribe to receiving certain topics
	s.host.SetStreamHandler(s.config.StreamProtocol, s.receiveUnits)
	defer s.host.RemoveStreamHandler(s.config.StreamProtocol)

	// Handle Engine outputs
	for {
		// todo(rdr): handle the engines output such as units to broadcast
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-s.eventsCh:
			s.handleEvent(ctx, event)
		}
	}
}

// todo(rdr): I am not sure of the Propeller <-> Engine separation...
// TBD how it looks like or if there should be any in the future

func (s *propellerService) Broadcast(committeeID *CommitteeID, msg []byte) error {
	return s.engine.Broadcast(committeeID, msg)
}

func (s *propellerService) Recv() <-chan []byte {
	return s.messageRecv
}

func (s *propellerService) RegisterCommittee(
	committeeID *CommitteeID,
	peers []PeerCommittee,
	// todo(rdr): peersKeys is something I don't know how to set correctly yet
	peersKeys []*StakerID,
) error {
	return s.engine.RegisterCommittee(committeeID, peers, peersKeys)
}

func (s *propellerService) UnregisterCommittee(comitteeID *CommitteeID) {
	s.engine.UnregisterCommittee(comitteeID)
}
