// I don't remember how this is usually done. This will do for now...
//go:generate protoc --go_out=proto --proto_path=proto ./proto/common.proto ./proto/propagation.proto ./proto/sync.proto

package p2p

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/p2p/snap"
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
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/multiformats/go-multiaddr"
)

const (
	defaultSourcePort         = 30301
	keyLength                 = 2048
	routingTableRefreshPeriod = 10 * time.Second
)

type Service struct {
	host       host.Host
	userAgent  string
	bootPeers  string
	network    utils.Network
	syncServer blockSyncServer
	blockchain *blockchain.Blockchain
	log        utils.SimpleLogger

	dht        *dht.IpfsDHT
	pubsub     *pubsub.PubSub
	topics     map[string]*pubsub.Topic
	topicsLock sync.RWMutex

	runCtx context.Context
}

func New(
	addr,
	userAgent,
	bootPeers string,
	privKeyStr string,
	bc *blockchain.Blockchain,
	snNetwork utils.Network,
	log utils.SimpleLogger,
) (*Service, error) {
	if addr == "" {
		// 0.0.0.0 will listen on any interface device.
		addr = fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", defaultSourcePort)
	}
	sourceMultiAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	prvKey, err := privateKey(privKeyStr)
	if err != nil {
		return nil, err
	}

	p2phost, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
		libp2p.UserAgent(userAgent),
	)
	if err != nil {
		return nil, err
	}

	p2pdht, err := makeDHT(p2phost, snNetwork, bootPeers)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPublicKey(prvKey.GetPublic())
	if err != nil {
		return nil, err
	}

	pidmhash, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", pid.String()))
	if err != nil {
		return nil, err
	}

	log.Infow(fmt.Sprintf("Id is %s\n", sourceMultiAddr.Encapsulate(pidmhash).String()))

	// Sync handler
	syncServer := blockSyncServer{
		blockchain: bc,
		converter:  NewConverter(&blockchainClassProvider{blockchain: bc}),
		log:        log,
	}

	snapSyncServer := snap.NewSnapSyncServer(bc, log)
	p2phost.SetStreamHandler(blockSyncProto, syncServer.handleBlockSyncStream)
	p2phost.SetStreamHandler(snap.Proto, snapSyncServer.HandleStream)

	return &Service{
		bootPeers:  bootPeers,
		log:        log,
		blockchain: bc,
		host:       p2phost,
		network:    snNetwork,
		dht:        p2pdht,
		topics:     make(map[string]*pubsub.Topic),
	}, nil
}

func makeDHT(p2phost host.Host, snNetwork utils.Network, cfgBootPeers string) (*dht.IpfsDHT, error) {
	bootPeers := []peer.AddrInfo{}
	if cfgBootPeers != "" {
		splitted := strings.Split(cfgBootPeers, ",")
		for _, peerStr := range splitted {
			bootAddr, err := peer.AddrInfoFromString(peerStr)
			if err != nil {
				return nil, err
			}

			bootPeers = append(bootPeers, *bootAddr)
		}
	}

	protocolPrefix := protocol.ID(fmt.Sprintf("/starknet/%s", snNetwork))
	return dht.New(context.Background(), p2phost,
		dht.ProtocolPrefix(protocolPrefix),
		dht.BootstrapPeers(bootPeers...),
		dht.RoutingTableRefreshPeriod(routingTableRefreshPeriod),
		dht.Mode(dht.ModeServer),
	)
}

func privateKey(privKeyStr string) (crypto.PrivKey, error) {
	if privKeyStr == "" {
		// Creates a new key pair for this host.
		prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, keyLength, cryptorand.Reader)
		if err != nil {
			return nil, err
		}
		return prvKey, nil
	}
	privKeyBytes, err := hex.DecodeString(privKeyStr)
	if err != nil {
		return nil, err
	}

	prvKey, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		return nil, err
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

func (s *Service) Run(ctx context.Context) error {
	var err error

	s.runCtx = ctx
	s.pubsub, err = pubsub.NewGossipSub(s.runCtx, s.host)
	if err != nil {
		return err
	}

	err = s.dht.Bootstrap(s.runCtx)
	if err != nil {
		return err
	}

	listenAddrs, err := s.ListenAddrs()
	if err != nil {
		return err
	}
	for _, addr := range listenAddrs {
		s.log.Infow("Listening on", "addr", addr)
	}

	err = s.setupIdentity()
	if err != nil {
		return errors.Join(err, errors.New("failed to setup identity protocol"))
	}

	// And some debug stuff
	go func() {
		sub, err2 := s.host.EventBus().Subscribe(event.WildcardSubscription)
		if err2 != nil {
			panic(err2)
		}
		for event := range sub.Out() {
			s.log.Infow("got event via bus", "event", event)
		}
	}()

	_, ok := os.LookupEnv("P2P_RUN_REENCODING_TEST")
	if ok {
		err = runBlockEncodingTests(s.blockchain)
		if err != nil {
			return err
		}
	}

	<-s.runCtx.Done()
	if err := s.dht.Close(); err != nil {
		s.log.Warnw("Failed stopping DHT", "err", err.Error())
	}
	return s.host.Close()
}

func (s *Service) setupIdentity() error {
	idservice, err := identify.NewIDService(s.host)
	if err != nil {
		return err
	}

	go idservice.Start()

	return nil
}

func (s *Service) SubscribeToNewPeer(ctx context.Context) (<-chan peerInfo, error) {
	ch := make(chan peerInfo)

	sub, err := s.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		return nil, err
	}

	go func() {
		for evt := range sub.Out() {
			evt := evt.(event.EvtPeerIdentificationCompleted)

			protocols, err := s.host.Peerstore().GetProtocols(evt.Peer)
			if err != nil {
				s.log.Infow("error getting peer protocol", "error", err)
				continue
			}

			ch <- peerInfo{
				id:        evt.Peer,
				protocols: protocols,
			}
		}
	}()

	return ch, nil
}

func (s *Service) SubscribeToPeerDisconnected(ctx context.Context) (<-chan peer.ID, error) {
	// TODO: implement this
	ch := make(chan peer.ID)
	return ch, nil
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

func (s *Service) CreateBlockSyncProvider() (*BlockSyncProvider, error) {
	blockSyncPeerManager, err := NewBlockSyncPeerManager(func(ctx context.Context) (network.Stream, func(), error) {
		str, err := s.NewStream(ctx, snap.Proto)
		if err != nil {
			return nil, nil, err
		}

		return str, func() {}, nil
	}, s.blockchain, s.log)

	if err != nil {
		return nil, err
	}

	return blockSyncPeerManager, nil
}

func (s *Service) CreateSnapProvider() (*snap.SnapProvider, error) {
	provider, err := snap.NewSnapProvider(func(ctx context.Context) (network.Stream, func(), error) {
		str, err := s.NewStream(ctx, snap.Proto)
		if err != nil {
			return nil, nil, err
		}

		return str, func() {}, nil
	}, s.log)

	if err != nil {
		return nil, err
	}

	return provider, nil
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
