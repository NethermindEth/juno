// I don't remember how this is usually done. This will do for now...
//go:generate protoc --go_out=proto --proto_path=proto ./proto/common.proto ./proto/propagation.proto ./proto/sync.proto

package p2p

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	p2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

const (
	blockSyncProto            = "/core/blocks-sync/1"
	defaultSourcePort         = 30301
	keyLength                 = 2048
	routingTableRefreshPeriod = 10 * time.Second
)

type Service struct {
	host       host.Host
	bootPeers  string
	network    utils.Network
	blockchain *blockchain.Blockchain

	dht *dht.IpfsDHT

	log utils.SimpleLogger
}

func New(
	addr,
	userAgent,
	bootPeers,
	privKeyStr string,
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

	p2pdht, err := makeDHT(p2phost, network, bootPeers)
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

	p2phost.SetStreamHandler(blockSyncProto, syncServer.handleBlockSyncStream)

	return &Service{
		bootPeers:  bootPeers,
		log:        log,
		host:       p2phost,
		network:    network,
		dht:        p2pdht,
		blockchain: bc,
	}, nil
}

func makeDHT(p2phost host.Host, network utils.Network, cfgBootPeers string) (*dht.IpfsDHT, error) {
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
				if typedEvnt.Connectedness == p2pnet.Connected {
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

	<-ctx.Done()
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
