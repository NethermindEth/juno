// I don't remember how this is usually done. This will do for now...
//go:generate protoc --go_out=proto --go-grpc_out=proto --proto_path=proto ./proto/common.proto ./proto/propagation.proto ./proto/sync.proto

package p2p

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/grpcclient"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/klauspost/compress/zstd"
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
	"github.com/multiformats/go-varint"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"reflect"
	"sync"
)

const blockSyncProto = "/core/blocks-sync/1"

type P2P interface {
	GetBlockHeaderByNumber(ctx context.Context, number uint64) (*core.Header, error)
}

type P2PImpl struct {
	host host.Host

	blockSyncPeers       []peer.ID
	mtx                  *sync.Mutex
	pickedBlockSyncPeers map[peer.ID]bool
}

func (ip *P2PImpl) handleBlockSyncRequest(request *grpcclient.Request) (*grpcclient.Response, error) {
	if request.GetStatus() != nil {
		return &grpcclient.Response{
			Response: &grpcclient.Response_Status{
				Status: &grpcclient.Status{
					Height: 0,
					Hash: &grpcclient.FieldElement{Elements: []byte{
						0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
					}},
				},
			},
		}, nil
	}

	return nil, fmt.Errorf("Unsupported request %v", request)
}

func (ip *P2PImpl) readCompressedProtobuf(stream network.Stream, request proto.Message) error {
	reader := bufio.NewReader(stream)
	_, err := varint.ReadUvarint(reader)
	if err != nil {
		return err
	}

	decoder, err := zstd.NewReader(reader)
	if err != nil {
		return err
	}

	msgBuff, err := io.ReadAll(decoder)
	if err != nil {
		return err
	}

	err = proto.Unmarshal(msgBuff, request)
	if err != nil {
		return err
	}

	return nil
}

func (ip *P2PImpl) writeCompressedProtobuf(stream network.Stream, resp proto.Message) error {
	buff, err := proto.Marshal(resp)
	if err != nil {
		return err
	}

	buffer := bytes.NewBuffer(make([]byte, 0))
	compressed, err := zstd.NewWriter(buffer)
	if err != nil {
		return err
	}

	_, err = compressed.Write(buff)
	if err != nil {
		return err
	}
	err = compressed.Close()
	if err != nil {
		return err
	}

	uvariantBuffer := varint.ToUvarint(uint64(buffer.Len()))
	_, err = stream.Write(uvariantBuffer)
	if err != nil {
		return err
	}

	_, err = io.Copy(stream, buffer)
	if err != nil {
		return err
	}

	return nil
}

func (ip *P2PImpl) handleBlockSyncStream(stream network.Stream) {
	msg := grpcclient.Request{}
	err := ip.readCompressedProtobuf(stream, &msg)
	if err != nil {
		panic(err)
	}

	resp, err := ip.handleBlockSyncRequest(&msg)
	if err != nil {
		panic(err)
	}

	err = ip.writeCompressedProtobuf(stream, resp)
	if err != nil {
		panic(err)
	}
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

func (ip *P2PImpl) setupKademlia(ctx context.Context) error {
	boot1, err := peer.AddrInfoFromString("/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWQGLGtoCnZ5y9En3ZHmcwKMwajZ7T2dUJYLky9zEVsaU3")
	if err != nil {
		return err
	}

	dhtinstance, err := dht.New(ctx, ip.host,
		dht.ProtocolPrefix("/pathfinder/kad/1.0.0"),
		dht.BootstrapPeers(*boot1),
	)

	ctx, events := dht.RegisterForLookupEvents(ctx)

	go func() {
		fmt.Println("Listening for kad events")

		for lookup := range events {
			fmt.Printf("Got event %s", lookup)
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
		// Hash: header.Hash, // Need to learn how to
		ParentHash:       fieldElementToFelt(header.ParentBlockHash),
		Number:           header.BlockNumber,
		GlobalStateRoot:  fieldElementToFelt(header.GlobalStateRoot),
		SequencerAddress: fieldElementToFelt(header.SequencerAddress),
		TransactionCount: uint64(header.TransactionCount),
		EventCount:       uint64(header.EventCount),
		Timestamp:        header.BlockTimestamp,
		ProtocolVersion:  string(header.ProtocolVersion),
		// ExtraData:        header.ExtraData,
		// EventsBloom:      header.EventsBloom,
	}

	return coreheader, nil
}

func fieldElementToFelt(field *grpcclient.FieldElement) *felt.Felt {
	var thebyte [4]uint64

	for i := 0; i < 4; i++ {
		thebyte[i] = binary.BigEndian.Uint64(field.Elements[i*8 : i*8+8]) // I don't know if this is right
	}

	return felt.NewFelt((*fp.Element)(&thebyte))
}

func (ip *P2PImpl) sendBlockSyncRequest(ctx context.Context, request *grpcclient.Request) (*grpcclient.Response, error) {
	p := ip.pickBlockSyncPeer()
	if p == nil {
		return nil, fmt.Errorf("no block sync peer available")
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

	err = ip.writeCompressedProtobuf(stream, request)
	if err != nil {
		return nil, err
	}
	err = stream.CloseWrite()
	if err != nil {
		return nil, err
	}

	resp := &grpcclient.Response{}
	err = ip.readCompressedProtobuf(stream, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (ip *P2PImpl) pickBlockSyncPeer() *peer.ID {
	// Not concurrent safe... just need something working right now...
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

func Start() (P2P, error) {
	ctx := context.Background()
	impl := P2PImpl{
		mtx:                  &sync.Mutex{},
		pickedBlockSyncPeers: map[peer.ID]bool{},
	}

	// Creates a new RSA key pair for this host.
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// 0.0.0.0 will listen on any interface device.
	sourceMultiAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", 30301))
	if err != nil {
		return nil, err
	}

	p2pHost, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
	)
	impl.host = p2pHost

	// Identity
	err = impl.setupKademlia(ctx)
	if err != nil {
		return nil, err
	}

	err = impl.setupIdentity(ctx)
	if err != nil {
		return nil, err
	}

	err = impl.setupGossipSub(ctx)
	if err != nil {
		return nil, err
	}

	// Sync handler
	p2pHost.SetStreamHandler(blockSyncProto, impl.handleBlockSyncStream)

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

	return &impl, nil
}
