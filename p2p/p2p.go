// I don't remember how this is usually done. This will do for now...
//go:generate protoc --go_out=proto --proto_path=proto ./proto/common.proto ./proto/propagation.proto ./proto/sync.proto

package p2p

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

const (
	defaultSourcePort         = 30301
	keyLength                 = 2048
	routingTableRefreshPeriod = 10 * time.Second
)

type P2PImpl struct {
	host       host.Host
	syncServer blockSyncServer
	blockchain *blockchain.Blockchain
	bootPeers  string

	logger utils.SimpleLogger
}

var _ p2pServer = &P2PImpl{}

func (ip *P2PImpl) setupGossipSub(ctx context.Context) error {
	// TODO: this should depend on network
	topic := "blocks/Görli"
	gossip, err := pubsub.NewGossipSub(ctx, ip.host)
	if err != nil {
		return err
	}

	topicObj, err := gossip.Join(topic)
	if err != nil {
		return err
	}

	topicSub, err := topicObj.Subscribe()
	if err != nil {
		return err
	}

	go func() {
		// TODO: this should be a loop and a place to ingest block propagation
		next, err := topicSub.Next(ctx)
		for err == nil {
			ip.logger.Infow("Got pubsub event", "event", next)
		}
		if err != nil {
			ip.logger.Infow("Got pubsub error", "error", err)
		}
	}()

	return nil
}

func (ip *P2PImpl) setupIdentity() error {
	idservice, err := identify.NewIDService(ip.host)
	if err != nil {
		return err
	}

	go idservice.Start()

	return nil
}

func (ip *P2PImpl) setupKademlia(ctx context.Context, bootPeersStr string) error {
	splitted := strings.Split(bootPeersStr, ",")
	bootPeers := make([]peer.AddrInfo, len(splitted))

	if len(splitted) == 1 && splitted[0] == "" {
		bootPeers = make([]peer.AddrInfo, 0)
	} else {
		for i, peerStr := range splitted {
			ip.logger.Infow("Boot peers", "peer", peerStr)

			bootAddr, err := peer.AddrInfoFromString(peerStr)
			if err != nil {
				return err
			}

			bootPeers[i] = *bootAddr
		}
	}

	dhtinstance, err := dht.New(ctx, ip.host,
		dht.ProtocolPrefix("/pathfinder/kad/1.0.0"),
		dht.BootstrapPeers(bootPeers...),
		dht.RoutingTableRefreshPeriod(routingTableRefreshPeriod),
		dht.Mode(dht.ModeServer),
	)
	if err != nil {
		return err
	}

	ctx, events := dht.RegisterForLookupEvents(ctx)

	go func() {
		ip.logger.Infow("Listening for kad events")

		for event := range events {
			ip.logger.Infow("Got event", "event", event)
		}
	}()

	err = dhtinstance.Bootstrap(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (ip *P2PImpl) determineKey() (crypto.PrivKey, error) {
	privKeyStr, ok := os.LookupEnv("P2P_PRIVATE_KEY")
	if !ok {
		// Creates a new key pair for this host.
		prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, keyLength, rand.Reader)
		if err != nil {
			return nil, err
		}

		prvKeyBytes, err := crypto.MarshalPrivateKey(prvKey)
		if err != nil {
			return nil, err
		}

		privKeyStr = hex.EncodeToString(prvKeyBytes)
		ip.logger.Infow(fmt.Sprintf("Generated a new key. P2P_PRIVATE_KEY=%s", privKeyStr))

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

func (ip *P2PImpl) SubscribeToNewPeer(ctx context.Context) (<-chan peerInfo, error) {
	ch := make(chan peerInfo)

	sub, err := ip.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		return nil, err
	}

	go func() {
		for evt := range sub.Out() {
			evt := evt.(event.EvtPeerIdentificationCompleted)

			protocols, err := ip.host.Peerstore().GetProtocols(evt.Peer)
			if err != nil {
				ip.logger.Infow("error getting peer protocol", "error", err)
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

func (ip *P2PImpl) SubscribeToPeerDisconnected(ctx context.Context) (<-chan peer.ID, error) {
	// TODO: implement this
	ch := make(chan peer.ID)
	return ch, nil
}

func (ip *P2PImpl) NewStream(ctx context.Context, id peer.ID, pcol protocol.ID) (network.Stream, error) {
	return ip.host.NewStream(ctx, id, pcol)
}

func New(
	bc *blockchain.Blockchain,
	addr,
	bootPeers string,
	log utils.SimpleLogger,
) (*P2PImpl, error) {
	impl := P2PImpl{
		blockchain: bc,
		bootPeers:  bootPeers,
		logger:     log,
	}

	prvKey, err := impl.determineKey()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to determine key")
	}

	var sourceMultiAddr multiaddr.Multiaddr
	// 0.0.0.0 will listen on any interface device.
	if addr != "" {
		sourceMultiAddr, err = multiaddr.NewMultiaddr(addr)
	} else {
		sourceMultiAddr, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", defaultSourcePort))
	}
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse address")
	}

	pubKey := prvKey.GetPublic()
	pid, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return nil, err
	}

	pidmhash, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", pid.String()))
	if err != nil {
		return nil, err
	}

	impl.logger.Infow(fmt.Sprintf("Id is %s\n", sourceMultiAddr.Encapsulate(pidmhash).String()))

	p2pHost, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
	)
	if err != nil {
		return nil, err
	}
	impl.host = p2pHost

	// Sync handler
	impl.syncServer = blockSyncServer{
		blockchain: bc,
		converter:  NewConverter(&blockchainClassProvider{blockchain: bc}),
		log:        impl.logger,
	}

	p2pHost.SetStreamHandler(blockSyncProto, impl.syncServer.handleBlockSyncStream)
	return &impl, nil
}

func (ip *P2PImpl) Run(ctx context.Context) error {
	err := ip.setupKademlia(ctx, ip.bootPeers)
	if err != nil {
		return errors.Wrap(err, "failed to setup kademlia")
	}

	err = ip.setupIdentity()
	if err != nil {
		return errors.Wrap(err, "failed to setup identity protocol")
	}

	err = ip.setupGossipSub(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup gossibsub")
	}

	// And some debug stuff
	go func() {
		sub, err2 := ip.host.EventBus().Subscribe(event.WildcardSubscription)
		if err2 != nil {
			panic(err2)
		}
		for event := range sub.Out() {
			ip.logger.Infow("got event via bus", "event", event)
		}
	}()

	_, ok := os.LookupEnv("P2P_RUN_REENCODING_TEST")
	if ok {
		err = runBlockEncodingTests(ip.blockchain)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ip *P2PImpl) CreateBlockSyncProvider() (BlockSyncPeerManager, service.Service, error) {
	peerManager, err := NewP2PPeerPoolManager(ip, blockSyncProto, ip.logger)
	if err != nil {
		return nil, nil, err
	}

	blockSyncPeerManager, err := NewBlockSyncPeerManager(peerManager.OpenStream, ip.blockchain, ip.logger)
	if err != nil {
		return nil, nil, err
	}

	return blockSyncPeerManager, peerManager, nil
}
