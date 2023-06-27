package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

type testPeerPool struct {
	newPeerChan          chan peerInfo
	peerDisconnectedChan chan peer.ID
}

func (t *testPeerPool) SubscribeToNewPeer(ctx context.Context) (<-chan peerInfo, error) {
	return t.newPeerChan, nil
}

func (t *testPeerPool) SubscribeToPeerDisconnected(ctx context.Context) (<-chan peer.ID, error) {
	return t.peerDisconnectedChan, nil
}

func (t *testPeerPool) NewStream(ctx context.Context, id peer.ID, pcal protocol.ID) (network.Stream, error) {
	return nil, nil
}

func newTestPeerPool() *testPeerPool {
	return &testPeerPool{
		newPeerChan:          make(chan peerInfo),
		peerDisconnectedChan: make(chan peer.ID),
	}
}

var _ p2pServer = &testPeerPool{}

func TestPeerPoolManager_OpenStreamBlocksUntilPeerIsDiscovered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testp2p := newTestPeerPool()
	testprotocol := protocol.ID("testprotocol")
	otherprotocol := protocol.ID("other")

	poolManager, err := NewP2PPeerPoolManager(ctx, testp2p, testprotocol, utils.NewNopZapLogger())
	if err != nil {
		t.Fatalf("error creating peer pool manager %s", err)
	}

	go func() {
		_ = poolManager.Start(ctx)
	}()

	opened := initiateStreamOpen(ctx, poolManager)

	select {
	case <-opened:
		t.Fatalf("stream opened when not expected")
	case <-time.After(time.Millisecond * 100):
	}

	testp2p.newPeerChan <- peerInfo{
		id:        "a",
		protocols: []protocol.ID{otherprotocol},
	}

	select {
	case <-opened:
		t.Fatalf("stream opened when not expected")
	case <-time.After(time.Millisecond * 100):
	}

	testp2p.newPeerChan <- peerInfo{
		id:        "a",
		protocols: []protocol.ID{testprotocol},
	}

	select {
	case <-opened:
	case <-time.After(time.Millisecond * 100):
		t.Fatalf("stream did not open when expected")
	}
}

func TestPeerPoolManager_OpenStreamBlocksIfNoPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testp2p := newTestPeerPool()
	testprotocol := protocol.ID("testprotocol")

	poolManager, err := NewP2PPeerPoolManager(ctx, testp2p, testprotocol, utils.NewNopZapLogger())
	if err != nil {
		t.Fatalf("error creating peer pool manager %s", err)
	}

	go func() {
		_ = poolManager.Start(ctx)
	}()

	testp2p.newPeerChan <- peerInfo{
		id:        "a",
		protocols: []protocol.ID{testprotocol},
	}
	testp2p.peerDisconnectedChan <- "a"

	<-time.After(time.Millisecond * 100)

	opened := initiateStreamOpen(ctx, poolManager)

	select {
	case <-opened:
		t.Fatalf("stream opened when not expected")
	case <-time.After(time.Millisecond * 100):
	}
}

func TestPeerPoolManager_OpenStreamCancelOnContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testp2p := newTestPeerPool()
	testprotocol := protocol.ID("testprotocol")

	poolManager, err := NewP2PPeerPoolManager(ctx, testp2p, testprotocol, utils.NewNopZapLogger())
	if err != nil {
		t.Fatalf("error creating peer pool manager %s", err)
	}

	go func() {
		_ = poolManager.Start(ctx)
	}()

	streamCtx, streamCtxCancel := context.WithCancel(context.Background())
	opened := initiateStreamOpen(streamCtx, poolManager)

	select {
	case <-opened:
		t.Fatalf("stream opened when not expected")
	case <-time.After(time.Millisecond * 100):
	}

	streamCtxCancel()

	select {
	case <-opened:
	case <-time.After(time.Millisecond * 100):
		t.Fatalf("stream not closed when expected")
	}
}

func initiateStreamOpen(ctx context.Context, poolManager *p2pPeerPoolManager) chan interface{} {
	opened := make(chan interface{})
	go func() {
		_, _, _ = poolManager.OpenStream(ctx)
		close(opened)
	}()
	return opened
}
