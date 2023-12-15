package p2p

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto/pb"
	"google.golang.org/protobuf/proto"

	"github.com/NethermindEth/juno/p2p/starknet"

	"github.com/NethermindEth/juno/blockchain"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

const (
	keyLength                 = 2048
	routingTableRefreshPeriod = 10 * time.Second
)

type Service struct {
	host host.Host

	network utils.Network
	handler *starknet.Handler
	log     utils.SimpleLogger

	dht        *dht.IpfsDHT
	pubsub     *pubsub.PubSub
	topics     map[string]*pubsub.Topic
	topicsLock sync.RWMutex

	synchroniser *syncService

	bootNode bool

	runCtx  context.Context
	runLock sync.RWMutex
}

func New(addr, userAgent, bootPeers, privKeyStr string, bootNode bool, bc *blockchain.Blockchain, snNetwork utils.Network,
	log utils.SimpleLogger,
) (*Service, error) {
	if addr == "" {
		// 0.0.0.0/tcp/0 will listen on any interface device and assing a free port.
		addr = "/ip4/0.0.0.0/tcp/0"
	}
	sourceMultiAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	prvKey, err := privateKey(privKeyStr)
	if err != nil {
		return nil, err
	}

	p2pHost, err := libp2p.New(libp2p.ListenAddrs(sourceMultiAddr), libp2p.Identity(prvKey), libp2p.UserAgent(userAgent))
	if err != nil {
		return nil, err
	}
	return NewWithHost(p2pHost, bootPeers, bootNode, bc, snNetwork, log)
}

func NewWithHost(p2phost host.Host, bootNodes string, bootNode bool, bc *blockchain.Blockchain, snNetwork utils.Network,
	log utils.SimpleLogger,
) (*Service, error) {
	bootPeers := []peer.AddrInfo{}
	if bootNodes != "" {
		splitted := strings.Split(bootNodes, ",")
		for _, peerStr := range splitted {
			bootAddr, err := peer.AddrInfoFromString(peerStr)
			if err != nil {
				return nil, fmt.Errorf("addr info from %q: %w", peerStr, err)
			}

			bootPeers = append(bootPeers, *bootAddr)
		}
	}

	p2pdht, err := makeDHT(p2phost, snNetwork, bootPeers)
	if err != nil {
		return nil, err
	}

	// todo: reconsider initialising synchroniser here because if node is a bootnode we shouldn't not create an instance of it.
	var peerId peer.ID
	if len(bootPeers) > 0 {
		peerId = bootPeers[0].ID
	}
	synchroniser := newSyncService(bc, p2phost, peerId, snNetwork, log)
	s := &Service{
		synchroniser: synchroniser,
		log:          log,
		host:         p2phost,
		network:      snNetwork,
		dht:          p2pdht,
		bootNode:     bootNode,
		topics:       make(map[string]*pubsub.Topic),
		handler:      starknet.NewHandler(bc, log),
	}
	s.runLock.Lock()
	return s, nil
}

func makeDHT(p2phost host.Host, snNetwork utils.Network, bootPeers []peer.AddrInfo) (*dht.IpfsDHT, error) {
	return dht.New(context.Background(), p2phost,
		dht.ProtocolPrefix(snNetwork.ProtocolID()),
		dht.BootstrapPeers(bootPeers...),
		dht.RoutingTableRefreshPeriod(routingTableRefreshPeriod),
		dht.Mode(dht.ModeServer),
	)
}

func privateKey(privKeyStr string) (crypto.PrivKey, error) {
	fmt.Println("Inside privateKey()", privKeyStr)
	if privKeyStr == "" {
		// Creates a new key pair for this host.
		prvKey, _, _, err := GenKeyPair()
		if err != nil {
			return nil, err
		}
		return prvKey, nil
	}

	privKeyBytes, err := hex.DecodeString(privKeyStr)
	if err != nil {
		return nil, err
	}

	privKeyBytesPB, err := proto.Marshal(&pb.PrivateKey{
		Type: utils.Ptr(pb.KeyType_Ed25519),
		Data: privKeyBytes,
	})
	if err != nil {
		return nil, err
	}

	prvKey, err := crypto.UnmarshalPrivateKey(privKeyBytesPB)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarslah private key: %w", err)
	}

	return prvKey, nil
}

func (s *Service) SubscribePeerConnectednessChanged(ctx context.Context) (<-chan event.EvtPeerConnectednessChanged, error) {
	ch := make(chan event.EvtPeerConnectednessChanged)
	sub, err := s.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				if err = sub.Close(); err != nil {
					s.log.Warnw("Failed to close subscription", "err", err)
				}
				close(ch)
				return
			case evnt := <-sub.Out():
				typedEvnt := evnt.(event.EvtPeerConnectednessChanged)
				if typedEvnt.Connectedness == network.Connected {
					ch <- typedEvnt
				}
			}
		}
	}()

	return ch, nil
}

// Run starts the p2p service. Calling any other function before run is undefined behaviour
func (s *Service) Run(ctx context.Context) error {
	defer s.host.Close()

	err := func() error {
		defer s.runLock.Unlock()

		err := s.dht.Bootstrap(ctx)
		if err != nil {
			return err
		}

		s.pubsub, err = pubsub.NewGossipSub(ctx, s.host)
		if err != nil {
			return err
		}

		s.runCtx = ctx
		return nil
	}()
	if err != nil {
		return err
	}
	defer s.callAndLogErr(s.dht.Close, "Failed stopping DHT")

	listenAddrs, err := s.ListenAddrs()
	if err != nil {
		return err
	}
	for _, addr := range listenAddrs {
		s.log.Infow("Listening on", "addr", addr)
	}

	// Start Synchronisation only if bootnode is false

	// 1. First get all the peers from dht
	// 2. Ask for missing blocks
	// 3. Synchronisation handles peers connecting and disconnecting
	// 4. Synchroniser has snapshot of peers to send requests to
	if !s.bootNode {
		// s.synchroniser.start(ctx)
		// s.synchroniser.startSerial(ctx)
		s.synchroniser.startPipeline(ctx)
	}
	s.setProtocolHandlers()

	<-s.runCtx.Done()
	if err := s.dht.Close(); err != nil {
		s.log.Warnw("Failed stopping DHT", "err", err.Error())
	}
	return s.host.Close()
}

func (s *Service) setProtocolHandlers() {
	s.SetProtocolHandler(starknet.BlockHeadersPID(s.network), s.handler.BlockHeadersHandler)
	s.SetProtocolHandler(starknet.CurrentBlockHeaderPID(s.network), s.handler.CurrentBlockHeaderHandler)
	s.SetProtocolHandler(starknet.ReceiptsPID(s.network), s.handler.ReceiptsHandler)
	// todo discuss protocol id (should it be included in BlockHeadersPID)
	s.SetProtocolHandler(starknet.BlockBodiesPID(s.network), s.handler.BlockBodiesHandler)
	s.SetProtocolHandler(starknet.EventsPID(s.network), s.handler.EventsHandler)
	s.SetProtocolHandler(starknet.TransactionsPID(s.network), s.handler.TransactionsHandler)
}

func (s *Service) callAndLogErr(f func() error, msg string) {
	err := f()
	if err != nil {
		s.log.Warnw(msg, "err", err.Error())
	}
}

func (s *Service) ListenAddrs() ([]multiaddr.Multiaddr, error) {
	pidmhash, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", s.host.ID()))
	if err != nil {
		return nil, err
	}

	var listenAddrs []multiaddr.Multiaddr
	for _, addr := range s.host.Addrs() {
		listenAddrs = append(listenAddrs, addr.Encapsulate(pidmhash))
	}

	return listenAddrs, nil
}

// NewStream creates a bidirectional connection to a random peer that implements a set of protocol ids
func (s *Service) NewStream(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
	peers := s.host.Peerstore().Peers()
	peersCount := peers.Len()
	if peersCount <= 0 {
		return nil, errors.New("no peers")
	}

	randomPeerIdx := rand.Intn(peersCount) //nolint: gosec
	for peerIdx := (randomPeerIdx + 1) % peersCount; ; peerIdx = (peerIdx + 1) % peersCount {
		peerID := peers[peerIdx]
		if peerID != s.host.ID() {
			stream, err := s.host.NewStream(ctx, peerID, pids...)
			if err == nil {
				return stream, nil
			} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
		}

		if peerIdx == randomPeerIdx {
			return nil, fmt.Errorf("no reachable peers supporting %s", protocol.ConvertToStrings(pids))
		}
	}
}

func (s *Service) joinTopic(topic string) (*pubsub.Topic, error) {
	existingTopic := func() *pubsub.Topic {
		s.topicsLock.RLock()
		defer s.topicsLock.RUnlock()
		if t, found := s.topics[topic]; found {
			return t
		}
		return nil
	}()

	if existingTopic != nil {
		return existingTopic, nil
	}

	s.runLock.RLock()
	defer s.runLock.RUnlock()
	if s.runCtx == nil {
		return nil, errors.New("uninitialized p2p service")
	}

	newTopic, err := s.pubsub.Join(topic)
	if err != nil {
		return nil, err
	}

	s.topicsLock.Lock()
	defer s.topicsLock.Unlock()
	s.topics[topic] = newTopic
	return newTopic, nil
}

func (s *Service) SubscribeToTopic(topic string) (chan []byte, func(), error) {
	t, joinErr := s.joinTopic(topic)
	if joinErr != nil {
		return nil, nil, joinErr
	}

	sub, subErr := t.Subscribe()
	if subErr != nil {
		return nil, nil, subErr
	}

	const bufferSize = 16
	ch := make(chan []byte, bufferSize)
	go func() {
		for {
			msg, err := sub.Next(s.runCtx)
			if err != nil {
				close(ch)
				return
			}
			// only forward messages delivered by others
			if msg.ReceivedFrom == s.host.ID() {
				continue
			}

			select {
			case ch <- msg.GetData():
			case <-s.runCtx.Done():
			}
		}
	}()
	return ch, sub.Cancel, nil
}

func (s *Service) PublishOnTopic(topic string, data []byte) error {
	t, joinErr := s.joinTopic(topic)
	if joinErr != nil {
		return joinErr
	}

	return t.Publish(s.runCtx, data)
}

func (s *Service) SetProtocolHandler(pid protocol.ID, handler func(network.Stream)) {
	s.host.SetStreamHandler(pid, handler)
}
