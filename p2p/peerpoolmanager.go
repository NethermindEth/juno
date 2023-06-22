package p2p

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"golang.org/x/exp/slices"
	"sync"
	"time"
)

// Simple peer pool manager
type p2pPeerPoolManager struct {
	p2p      *P2PImpl
	protocol protocol.ID

	syncPeerMtx          *sync.Mutex
	peerTurn             int
	blockSyncPeers       []peer.ID
	syncPeerUpdateChan   chan int
	pickedBlockSyncPeers map[peer.ID]int
}

func (p *p2pPeerPoolManager) Start(ctx context.Context) error {
	sub, err := p.p2p.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		return err
	}

	for evt := range sub.Out() {
		evt := evt.(event.EvtPeerIdentificationCompleted)

		protocols, err := p.p2p.host.Peerstore().GetProtocols(evt.Peer)
		if err != nil {
			fmt.Printf("Error %v\n", err)
			continue
		}

		if slices.Contains(protocols, p.protocol) {
			p.AddPeer(evt.Peer)
		}
	}

	return nil
}

func (p *p2pPeerPoolManager) AddPeer(id peer.ID) {
	if !slices.Contains(p.blockSyncPeers, id) {
		p.blockSyncPeers = append(p.blockSyncPeers, id)
		select {
		case p.syncPeerUpdateChan <- 0:
		default:
		}
	}
}

func (p *p2pPeerPoolManager) OpenStream(ctx context.Context) (network.Stream, func(), error) {
	peer, err := p.pickBlockSyncPeer(ctx)
	if err != nil {
		return nil, nil, err
	}

	stream, err := p.p2p.host.NewStream(ctx, *peer, p.protocol)
	return stream, func() {
		p.releaseBlockSyncPeer(peer)
	}, err
}

func (p *p2pPeerPoolManager) pickBlockSyncPeer(ctx context.Context) (*peer.ID, error) {
	for {
		peer := p.pickBlockSyncPeerNoWait()
		if peer != nil {
			return peer, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		case <-p.syncPeerUpdateChan:
		}
	}
}

func (p *p2pPeerPoolManager) pickBlockSyncPeerNoWait() *peer.ID {
	maxConcurrentRequestPerPeer := 32
	p.syncPeerMtx.Lock()
	defer p.syncPeerMtx.Unlock()

	// Simple mechanism to round robin the peers
	p.peerTurn = p.peerTurn + 1

	for i := 0; i < len(p.blockSyncPeers); i++ {
		peer := p.blockSyncPeers[(i+p.peerTurn)%len(p.blockSyncPeers)]
		if p.pickedBlockSyncPeers[peer] < maxConcurrentRequestPerPeer {
			p.pickedBlockSyncPeers[peer] += 1
			return &peer
		}
	}
	return nil
}

func (p *p2pPeerPoolManager) releaseBlockSyncPeer(id *peer.ID) {
	p.syncPeerMtx.Lock()
	defer p.syncPeerMtx.Unlock()

	p.pickedBlockSyncPeers[*id] -= 1

	select {
	case p.syncPeerUpdateChan <- 0:
	default:
	}
}
