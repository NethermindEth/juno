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

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/starknet"
	junoSync "github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/crypto/pb"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"
)

const (
	keyLength                 = 2048
	routingTableRefreshPeriod = 1 * time.Second
	clientName                = "juno"
)

type Service struct {
	host host.Host

	network *utils.Network
	handler *starknet.Handler
	log     utils.SimpleLogger

	dht        *dht.IpfsDHT
	pubsub     *pubsub.PubSub
	topics     map[string]*pubsub.Topic
	topicsLock sync.RWMutex

	synchroniser *syncService
	gossipTracer *gossipTracer

	feederNode bool
	database   db.DB
}

func New(addr, publicAddr, version, peers, privKeyStr string, feederNode bool, bc *blockchain.Blockchain, snNetwork *utils.Network,
	log utils.SimpleLogger, database db.DB,
) (*Service, error) {
	if addr == "" {
		// 0.0.0.0/tcp/0 will listen on any interface device and assing a free port.
		addr = "/ip4/0.0.0.0/tcp/0"
	}
	sourceMultiAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	var publicMultiAddr multiaddr.Multiaddr
	if publicAddr != "" {
		publicMultiAddr, err = multiaddr.NewMultiaddr(publicAddr)
		if err != nil {
			return nil, err
		}
	}

	prvKey, err := privateKey(privKeyStr)
	if err != nil {
		return nil, err
	}

	// The address Factory is used when the public ip is passed to the node.
	// In this case the node will NOT try to listen to the public IP because
	// it is not possible to listen to a public IP. Instead, the node will
	// listen to the private one (or 0.0.0.0) and will add the public IP to
	// the list of addresses that it will advertise to the network.
	addressFactory := func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
		if publicMultiAddr != nil {
			addrs = append(addrs, publicMultiAddr)
		}
		return addrs
	}

	p2pHost, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
		libp2p.UserAgent(makeAgentName(version)),
		// Use address factory to add the public address to the list of
		// addresses that the node will advertise.
		libp2p.AddrsFactory(addressFactory),
		// If we know the public ip, enable the relay service.
		libp2p.EnableRelayService(),
		// When listening behind NAT, enable peers to try to poke thought the
		// NAT in order to reach the node.
		libp2p.EnableHolePunching(),
		// Try to open a port in the NAT router to accept incoming connections.
		libp2p.NATPortMap(),
	)
	if err != nil {
		return nil, err
	}
	// Todo: try to understand what will happen if user passes a multiaddr with p2p public and a private key which doesn't match.
	// For example, a user passes the following multiaddr: --p2p-addr=/ip4/0.0.0.0/tcp/7778/p2p/(SomePublicKey) and also passes a
	// --p2p-private-key="SomePrivateKey". However, the private public key pair don't match, in this case what will happen?
	return NewWithHost(p2pHost, peers, feederNode, bc, snNetwork, log, database)
}

func NewWithHost(p2phost host.Host, peers string, feederNode bool, bc *blockchain.Blockchain, snNetwork *utils.Network,
	log utils.SimpleLogger, database db.DB,
) (*Service, error) {
	var (
		peersAddrInfoS []peer.AddrInfo
		err            error
	)

	peersAddrInfoS, err = loadPeers(database)
	if err != nil {
		log.Warnw("Failed to load peers", "err", err)
	}

	if peers != "" {
		splitted := strings.Split(peers, ",")
		for _, peerStr := range splitted {
			var peerAddr *peer.AddrInfo
			peerAddr, err = peer.AddrInfoFromString(peerStr)
			if err != nil {
				return nil, fmt.Errorf("addr info from %q: %w", peerStr, err)
			}

			peersAddrInfoS = append(peersAddrInfoS, *peerAddr)
		}
	}

	p2pdht, err := makeDHT(p2phost, peersAddrInfoS)
	if err != nil {
		return nil, err
	}

	// todo: reconsider initialising synchroniser here because if node is a feedernode we shouldn't not create an instance of it.

	synchroniser := newSyncService(bc, p2phost, snNetwork, log)
	s := &Service{
		synchroniser: synchroniser,
		log:          log,
		host:         p2phost,
		network:      snNetwork,
		dht:          p2pdht,
		feederNode:   feederNode,
		topics:       make(map[string]*pubsub.Topic),
		handler:      starknet.NewHandler(bc, log),
		database:     database,
	}
	return s, nil
}

func makeDHT(p2phost host.Host, addrInfos []peer.AddrInfo) (*dht.IpfsDHT, error) {
	return dht.New(context.Background(), p2phost,
		dht.ProtocolPrefix(starknet.Prefix),
		dht.BootstrapPeers(addrInfos...),
		dht.RoutingTableRefreshPeriod(routingTableRefreshPeriod),
		dht.Mode(dht.ModeServer),
	)
}

func privateKey(privKeyStr string) (crypto.PrivKey, error) {
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
	defer func() {
		if err := s.host.Close(); err != nil {
			s.log.Warnw("Failed to close host", "err", err)
		}
	}()

	err := s.dht.Bootstrap(ctx)
	if err != nil {
		return err
	}

	var options []pubsub.Option
	if s.gossipTracer != nil {
		options = append(options, pubsub.WithRawTracer(s.gossipTracer))
	}

	s.pubsub, err = pubsub.NewGossipSub(ctx, s.host, options...)
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

	s.setProtocolHandlers()

	if !s.feederNode {
		s.synchroniser.start(ctx)
	}

	<-ctx.Done()
	if err := s.persistPeers(); err != nil {
		s.log.Warnw("Failed to persist peers", "err", err)
	}
	if err := s.dht.Close(); err != nil {
		s.log.Warnw("Failed stopping DHT", "err", err.Error())
	}
	return nil
}

func (s *Service) setProtocolHandlers() {
	s.SetProtocolHandler(starknet.HeadersPID(), s.handler.HeadersHandler)
	s.SetProtocolHandler(starknet.EventsPID(), s.handler.EventsHandler)
	s.SetProtocolHandler(starknet.TransactionsPID(), s.handler.TransactionsHandler)
	s.SetProtocolHandler(starknet.ClassesPID(), s.handler.ClassesHandler)
	s.SetProtocolHandler(starknet.StateDiffPID(), s.handler.StateDiffHandler)
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

	listenAddrs := make([]multiaddr.Multiaddr, 0, len(s.host.Addrs()))
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
	// go func() {
	//	for {
	//		msg, err := sub.Next(s.runCtx)
	//		if err != nil {
	//			close(ch)
	//			return
	//		}
	//		only forward messages delivered by others
	//		if msg.ReceivedFrom == s.host.ID() {
	//			continue
	//		}
	//
	//		select {
	//		case ch <- msg.GetData():
	//		case <-s.runCtx.Done():
	//		}
	//	}
	// }()
	return ch, sub.Cancel, nil
}

func (s *Service) PublishOnTopic(topic string) error {
	_, err := s.joinTopic(topic)
	return err
}

func (s *Service) SetProtocolHandler(pid protocol.ID, handler func(network.Stream)) {
	s.host.SetStreamHandler(pid, handler)
}

func (s *Service) WithListener(l junoSync.EventListener) {
	s.synchroniser.WithListener(l)
}

func (s *Service) WithGossipTracer() {
	s.gossipTracer = NewGossipTracer(s.host)
}

// persistPeers stores the given peers in the peers database
func (s *Service) persistPeers() error {
	txn, err := s.database.NewTransaction(true)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	store := s.host.Peerstore()
	peers := utils.Filter(store.Peers(), func(peerID peer.ID) bool {
		return peerID != s.host.ID()
	})
	for _, peerID := range peers {
		peerInfo := store.PeerInfo(peerID)

		encodedAddrs, err := EncodeAddrs(peerInfo.Addrs)
		if err != nil {
			return fmt.Errorf("encode addresses for peer %s: %w", peerID, err)
		}

		if err := txn.Set(db.Peer.Key([]byte(peerID)), encodedAddrs); err != nil {
			return fmt.Errorf("set data for peer %s: %w", peerID, err)
		}
	}

	if err := txn.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	s.log.Infow("Stored peers", "num", len(peers))

	return nil
}

// loadPeers loads the previously stored peers from the database
func loadPeers(database db.DB) ([]peer.AddrInfo, error) {
	var peers []peer.AddrInfo

	err := database.View(func(txn db.Transaction) error {
		it, err := txn.NewIterator()
		if err != nil {
			return fmt.Errorf("create iterator: %w", err)
		}
		defer it.Close()

		prefix := db.Peer.Key()
		for it.Seek(prefix); it.Valid(); it.Next() {
			peerIDBytes := it.Key()[len(prefix):]
			peerID, err := peer.IDFromBytes(peerIDBytes)
			if err != nil {
				return fmt.Errorf("decode peer ID: %w", err)
			}

			val, err := it.Value()
			if err != nil {
				return fmt.Errorf("get value: %w", err)
			}

			addrs, err := decodeAddrs(val)
			if err != nil {
				return fmt.Errorf("decode addresses for peer %s: %w", peerID, err)
			}

			peers = append(peers, peer.AddrInfo{ID: peerID, Addrs: addrs})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load peers: %w", err)
	}

	return peers, nil
}

func makeAgentName(version string) string {
	modVer := "0.0.0"
	semVer, err := semver.NewVersion(version)
	if err == nil {
		modVer = fmt.Sprintf("%d.%d.%d", semVer.Major(), semVer.Minor(), semVer.Patch())
	}

	return fmt.Sprintf("%s/%s", clientName, modVer)
}
