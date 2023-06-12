// I don't remember how this is usually done. This will do for now...
//go:generate protoc --go_out=proto --go-grpc_out=proto --proto_path=proto ./proto/common.proto ./proto/propagation.proto ./proto/sync.proto

package p2p

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/grpcclient"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

const blockSyncProto = "/core/blocks-sync/1"

type P2P interface {
	GetBlockHeaderByNumber(ctx context.Context, number uint64) (*core.Header, error)
	GetBlockByHash(ctx context.Context, hash *felt.Felt) (*core.Block, error)
}

type P2PImpl struct {
	host       host.Host
	syncServer blockSyncServer

	blockSyncPeers       []peer.ID
	mtx                  *sync.Mutex
	pickedBlockSyncPeers map[peer.ID]bool
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

func (ip *P2PImpl) setupIdentity(ctx context.Context) error {
	idservice, err := identify.NewIDService(ip.host)
	if err != nil {
		return err
	}

	go idservice.Start()

	sub, err := ip.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		panic(err)
	}
	go func() {
		for evt := range sub.Out() {
			evt := evt.(event.EvtPeerIdentificationCompleted)

			protocols, err := ip.host.Peerstore().GetProtocols(evt.Peer)
			if err != nil {
				fmt.Printf("Error %v\n", err)
				continue
			}

			fmt.Printf("The protocols for %v is %+v\n", evt.Peer, protocols)

			if slices.Contains(protocols, blockSyncProto) && !slices.Contains(ip.blockSyncPeers, evt.Peer) {
				ip.blockSyncPeers = append(ip.blockSyncPeers, evt.Peer)
			}
		}
	}()

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
	)

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

func (ip *P2PImpl) GetBlockHeaderByNumber(ctx context.Context, number uint64) (*core.Header, error) {
	request := grpcclient.GetBlockHeaders{
		StartBlock: &grpcclient.GetBlockHeaders_BlockNumber{
			BlockNumber: number,
		},
		Count: 1,
	}

	response, err := ip.sendBlockSyncRequest(ctx,
		&grpcclient.Request{
			Request: &grpcclient.Request_GetBlockHeaders{
				GetBlockHeaders: &request,
			},
		})

	if err != nil {
		return nil, err
	}

	blockHeaders := response.GetBlockHeaders().GetHeaders()
	if blockHeaders == nil {
		return nil, fmt.Errorf("Block headers is nil")
	}

	if len(blockHeaders) != 1 {
		return nil, fmt.Errorf("Unexpected number of block headers. Expected: 1, Actual: %d", len(blockHeaders))
	}

	header := blockHeaders[0]

	// Need to make a custom hasher from the grpc header. The core.Header missed a few hashes.
	coreheader := &core.Header{
		Hash:             fieldElementToFelt(header.Hash),
		ParentHash:       fieldElementToFelt(header.ParentBlockHash),
		Number:           header.BlockNumber,
		GlobalStateRoot:  fieldElementToFelt(header.GlobalStateRoot),
		SequencerAddress: fieldElementToFelt(header.SequencerAddress),
		TransactionCount: uint64(header.TransactionCount),
		EventCount:       uint64(header.EventCount),
		Timestamp:        header.BlockTimestamp,
		ProtocolVersion:  fmt.Sprintf("%d", header.ProtocolVersion),
		// ExtraData:        header.ExtraData,
		// EventsBloom:      header.EventsBloom,
	}

	return coreheader, nil
}

func (ip *P2PImpl) GetBlockByHash(ctx context.Context, hash *felt.Felt) (*core.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (ip *P2PImpl) sendBlockSyncRequest(ctx context.Context, request *grpcclient.Request) (*grpcclient.Response, error) {
	p, err := ip.pickBlockSyncPeer(ctx)
	if err != nil {
		return nil, err
	}
	defer ip.releaseBlockSyncPeer(p)

	stream, err := ip.host.NewStream(ctx, *p, blockSyncProto)
	if err != nil {
		return nil, err
	}
	defer func(stream network.Stream) {
		err := stream.Close()
		if err != nil {
			fmt.Printf("Error closing stream %s", err)
		}
	}(stream)

	err = writeCompressedProtobuf(stream, request)
	if err != nil {
		return nil, err
	}
	err = stream.CloseWrite()
	if err != nil {
		return nil, err
	}

	resp := &grpcclient.Response{}
	err = readCompressedProtobuf(stream, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (ip *P2PImpl) pickBlockSyncPeer(ctx context.Context) (*peer.ID, error) {
	for {
		p := ip.pickBlockSyncPeerNoWait()
		if p != nil {
			return p, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (ip *P2PImpl) pickBlockSyncPeerNoWait() *peer.ID {
	ip.mtx.Lock()
	defer ip.mtx.Unlock()
	for _, p := range ip.blockSyncPeers {
		if !ip.pickedBlockSyncPeers[p] {
			ip.pickedBlockSyncPeers[p] = true
			return &p
		}
	}

	return nil
}

func (ip *P2PImpl) releaseBlockSyncPeer(id *peer.ID) {
	ip.mtx.Lock()
	defer ip.mtx.Unlock()
	delete(ip.pickedBlockSyncPeers, *id)
}

func Start(blockchain *blockchain.Blockchain, addr string, bootPeers string) (P2P, error) {
	ctx := context.Background()
	impl := P2PImpl{
		syncServer: blockSyncServer{
			blockchain: blockchain,
		},

		mtx:                  &sync.Mutex{},
		pickedBlockSyncPeers: map[peer.ID]bool{},
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
		sourceMultiAddr, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", 30301))
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
		sub, err := p2pHost.EventBus().Subscribe(event.WildcardSubscription)
		if err != nil {
			panic(err)
		}
		for eent := range sub.Out() {
			fmt.Printf("Got event via bus %s %+v\n", reflect.TypeOf(eent), eent)
		}
	}()

	head, err := blockchain.Head()
	if err != nil {
		return nil, err
	}

	blocknumchan := make(chan int)

	threadcount := 32
	wg := sync.WaitGroup{}
	wg.Add(threadcount)
	for i := 0; i < threadcount; i++ {
		go func() {
			defer wg.Done()
			for i := range blocknumchan {
				fmt.Printf("Running on block %d\n", i)

				theblock, err := blockchain.BlockByNumber(uint64(i))
				if err != nil {
					panic(err)
				}

				err = testBlockEncoding(theblock, blockchain)
				if err != nil {
					panic(err)
				}
			}
		}()
	}

	startblock := 4900
	for i := startblock; i < int(head.Number); i++ {
		blocknumchan <- i
	}
	close(blocknumchan)
	wg.Wait()

	return &impl, nil
}

func testBlockEncoding(originalBlock *core.Block, blockchain *blockchain.Blockchain) error {
	c := converter{
		blockchain: blockchain,
	}
	originalBlock.ProtocolVersion = ""

	protoheader, err := c.coreBlockToProtobufHeader(originalBlock)
	if err != nil {
		return err
	}

	protoBody, err := c.coreBlockToProtobufBody(originalBlock)
	if err != nil {
		return err
	}

	newCoreBlock, err := protobufHeaderAndBodyToCoreBlock(protoheader, protoBody, blockchain.Network())
	if err != nil {
		return err
	}

	newCoreBlock.ProtocolVersion = ""

	gatewayjson, err := json.MarshalIndent(originalBlock, "", "    ")
	if err != nil {
		return err
	}

	reencodedblockjson, err := json.MarshalIndent(newCoreBlock, "", "    ")
	if err != nil {
		return err
	}

	if string(gatewayjson) != string(reencodedblockjson) {
		for i, receipt := range originalBlock.Receipts {
			tx := originalBlock.Transactions[i]

			tx2 := newCoreBlock.Transactions[i]
			receipt2 := newCoreBlock.Receipts[i]

			if !compareAndPrintDiff(tx, tx2) {
				thegrpctx := protoBody.Transactions[i]
				felttx := fieldElementToFelt(thegrpctx.GetL1Handler().GetHash())
				return fmt.Errorf("tx mismatch. %s %s", thegrpctx, felttx)
			}

			if !compareAndPrintDiff(receipt, receipt2) {
				return errors.New("receipt mismatch")
			}

			if len(receipt.Events) > 0 {
				txjson, _ := json.Marshal(tx)
				treceipt, _ := json.Marshal(receipt)
				fmt.Println(txjson)
				fmt.Println(treceipt)
			}

		}

		compareAndPrintDiff(originalBlock, newCoreBlock)
		return errors.New("Mismatch")
	}

	return nil
}

func determineKey() (crypto.PrivKey, error) {
	var prvKey crypto.PrivKey
	var err error

	if err != nil {
		return nil, err
	}

	privKeyStr, ok := os.LookupEnv("P2P_PRIVATE_KEY")
	if !ok {
		// Creates a new key pair for this host.
		prvKey, _, err = crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
		if err != nil {
			return nil, err
		}

		prvKeyBytes, err := crypto.MarshalPrivateKey(prvKey)
		if err != nil {
			return nil, err
		}

		privKeyStr = hex.EncodeToString(prvKeyBytes)
		fmt.Printf("Generated a new key. P2P_PRIVATE_KEY=%s\n", privKeyStr)
	} else {
		privKeyBytes, err := hex.DecodeString(privKeyStr)
		if err != nil {
			return nil, err
		}

		prvKey, err = crypto.UnmarshalPrivateKey(privKeyBytes)
		if err != nil {
			return nil, err
		}
	}

	return prvKey, err
}
