// I don't remember how this is usually done. This will do for now...
//go:generate protoc --go_out=proto --proto_path=proto ./proto/common.proto ./proto/propagation.proto ./proto/sync.proto

package p2p

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/pkg/errors"
	"os"
	"strings"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
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

	dht *dht.IpfsDHT

	log utils.SimpleLogger
}

var _ p2pServer = &Service{}

func New(
	addr,
	userAgent,
	bootPeers string,
	bc *blockchain.Blockchain,
	network utils.Network,
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

	prvKey, err := privateKey()
	if err != nil {
		return nil, err
	}

	host, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
		libp2p.UserAgent(userAgent),
	)
	if err != nil {
		return nil, err
	}

	dht, err := makeDHT(host, network, bootPeers)
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

	host.SetStreamHandler(blockSyncProto, syncServer.handleBlockSyncStream)

	return &Service{
		bootPeers:  bootPeers,
		log:        log,
		host:       host,
		network:    network,
		blockchain: bc,
		dht:        dht,
	}, nil
}

func makeDHT(host host.Host, network utils.Network, cfgBootPeers string) (*dht.IpfsDHT, error) {
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

	protocolPrefix := protocol.ID(fmt.Sprintf("/starknet/%s", network.String()))
	return dht.New(context.Background(), host,
		dht.ProtocolPrefix(protocolPrefix),
		dht.BootstrapPeers(bootPeers...),
		dht.RoutingTableRefreshPeriod(routingTableRefreshPeriod),
		dht.Mode(dht.ModeServer),
	)
}

func privateKey() (crypto.PrivKey, error) {
	privKeyStr, ok := os.LookupEnv("P2P_PRIVATE_KEY")
	if !ok {
		// Creates a new key pair for this host.
		prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, keyLength, rand.Reader)
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
	err := s.dht.Bootstrap(ctx)
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
		return errors.Wrap(err, "failed to setup identity protocol")
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

	return nil

	<-ctx.Done()
	if err := s.dht.Close(); err != nil {
		s.log.Warnw("Failed stopping DHT", "err", err.Error())
	}
	return s.host.Close()
}

func (ip *Service) setupIdentity() error {
	idservice, err := identify.NewIDService(ip.host)
	if err != nil {
		return err
	}

	go idservice.Start()

	return nil
}

func (ip *Service) SubscribeToNewPeer(ctx context.Context) (<-chan peerInfo, error) {
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
				ip.log.Infow("error getting peer protocol", "error", err)
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

func (ip *Service) SubscribeToPeerDisconnected(ctx context.Context) (<-chan peer.ID, error) {
	// TODO: implement this
	ch := make(chan peer.ID)
	return ch, nil
}

func (ip *Service) NewStream(ctx context.Context, id peer.ID, pcol protocol.ID) (network.Stream, error) {
	return ip.host.NewStream(ctx, id, pcol)
}

func (s *Service) ListenAddrs() ([]multiaddr.Multiaddr, error) {
	pidmhash, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", s.host.ID().String()))
	if err != nil {
		return nil, err
	}

	var listenAddrs []multiaddr.Multiaddr
	for _, addr := range s.host.Addrs() {
		listenAddrs = append(listenAddrs, addr.Encapsulate(pidmhash))
	}

	return listenAddrs, nil
}

func (ip *Service) CreateBlockSyncProvider() (BlockSyncPeerManager, service.Service, error) {
	peerManager, err := NewP2PPeerPoolManager(ip, blockSyncProto, ip.log)
	if err != nil {
		return nil, nil, err
	}

	blockSyncPeerManager, err := NewBlockSyncPeerManager(peerManager.OpenStream, ip.blockchain, ip.log)
	if err != nil {
		return nil, nil, err
	}

	return blockSyncPeerManager, peerManager, nil
}
