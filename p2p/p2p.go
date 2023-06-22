// I don't remember how this is usually done. This will do for now...
//go:generate protoc --go_out=proto --proto_path=proto ./proto/common.proto ./proto/propagation.proto ./proto/sync.proto

package p2p

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

const defaultSourcePort = 30301
const keyLength = 2048
const routingTableRefreshPeriod = 10 * time.Second

type P2PImpl struct {
	host       host.Host
	syncServer blockSyncServer

	blockchain *blockchain.Blockchain
}

func (ip *P2PImpl) setupGossipSub(ctx context.Context) error {
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
		next, err := topicSub.Next(ctx)
		for err == nil {
			fmt.Printf("Got pubsub event %+v\n", next)
		}
		if err != nil {
			fmt.Printf("Pubsub err %+v\n", err)
		}
	}()

	return nil
}
func (ip *P2PImpl) setupIdentity(context.Context) error {
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
			fmt.Printf("Boot peer: %s\n", peerStr)

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
		fmt.Println("Listening for kad events")

		for lookup := range events {
			fmt.Printf("Got event %v", lookup)
		}
	}()

	err = dhtinstance.Bootstrap(ctx)
	if err != nil {
		return err
	}

	return nil
}

func determineKey() (crypto.PrivKey, error) {
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
		fmt.Printf("Generated a new key. P2P_PRIVATE_KEY=%s\n", privKeyStr)

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
func Start(blockchain *blockchain.Blockchain, addr string, bootPeers string, log utils.SimpleLogger) (*P2PImpl, error) {
	ctx := context.Background()
	converter := NewConverter(&blockchainClassProvider{
		blockchain: blockchain,
	})
	impl := P2PImpl{
		blockchain: blockchain,
		syncServer: blockSyncServer{
			blockchain: blockchain,
			converter:  converter,
			log:        log,
		},
	}

	prvKey, err := determineKey()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to determine key")
	}

	var sourceMultiAddr multiaddr.Multiaddr
	// 0.0.0.0 will listen on any interface device.
	if len(addr) != 0 {
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

	fmt.Printf("Id is %s\n", sourceMultiAddr.Encapsulate(pidmhash).String())

	p2pHost, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
	)
	if err != nil {
		return nil, err
	}
	impl.host = p2pHost

	err = impl.setupKademlia(ctx, bootPeers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup kademlia")
	}

	err = impl.setupIdentity(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup identity protocol")
	}

	err = impl.setupGossipSub(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup gossibsub")
	}

	// Sync handler
	p2pHost.SetStreamHandler(blockSyncProto, impl.syncServer.handleBlockSyncStream)

	// And some other stuff
	go func() {
		sub, err2 := p2pHost.EventBus().Subscribe(event.WildcardSubscription)
		if err2 != nil {
			panic(err2)
		}
		for eent := range sub.Out() {
			fmt.Printf("Got event via bus %s %+v\n", reflect.TypeOf(eent), eent)
		}
	}()

	_, ok := os.LookupEnv("P2P_RUN_REENCODING_TEST")
	if ok {
		err = runBlockEncodingTests(blockchain)
		if err != nil {
			return nil, err
		}
	}

	return &impl, nil
}

func (ip *P2PImpl) CreateBlockSyncProvider(ctx context.Context) (BlockSyncPeerManager, error) {
	peerManager := p2pPeerPoolManager{
		p2p:                  ip,
		protocol:             blockSyncProto,
		blockSyncPeers:       make([]peer.ID, 0),
		syncPeerMtx:          &sync.Mutex{},
		syncPeerUpdateChan:   make(chan int),
		pickedBlockSyncPeers: map[peer.ID]int{},
	}

	go peerManager.Start(ctx)
	return NewBlockSyncPeerManager(ctx, peerManager.OpenStream, ip.blockchain)
}
