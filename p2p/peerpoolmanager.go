package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

type p2pServer interface {
	SubscribeToNewPeer(ctx context.Context) (<-chan peerInfo, error)
	SubscribeToPeerDisconnected(ctx context.Context) (<-chan peer.ID, error)
	NewStream(ctx context.Context, id peer.ID, protocol protocol.ID) (network.Stream, error)
}

type peerInfo struct {
	id        peer.ID
	protocols []protocol.ID
}

// Simple peer pool manager for a particular protocol
type p2pPeerPoolManager struct {
	p2p      p2pServer
	protocol protocol.ID
	logger   utils.SimpleLogger

	syncPeerMtx                 *sync.Mutex
	peerTurn                    int
	blockSyncPeers              []peer.ID
	syncPeerUpdateChan          chan int
	pickedBlockSyncPeers        map[peer.ID]int
	maxConcurrentRequestPerPeer int
}

var _ service.Service = &p2pPeerPoolManager{}

func NewP2PPeerPoolManager(p2p p2pServer, proto protocol.ID, logger utils.SimpleLogger) (*p2pPeerPoolManager, error) {
	peerManager := &p2pPeerPoolManager{
		p2p:      p2p,
		protocol: proto,
		logger:   logger,

		blockSyncPeers:              make([]peer.ID, 0),
		syncPeerMtx:                 &sync.Mutex{},
		syncPeerUpdateChan:          make(chan int),
		pickedBlockSyncPeers:        map[peer.ID]int{},
		maxConcurrentRequestPerPeer: 128,
	}

	return peerManager, nil
}

func (p *p2pPeerPoolManager) Run(ctx context.Context) error {
	newPeerCh, err := p.p2p.SubscribeToNewPeer(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to subscribe to new peer")
	}

	peerDisconnectedCh, err := p.p2p.SubscribeToPeerDisconnected(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to subscribe to new peer")
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		for ch := range newPeerCh {
			if slices.Contains(ch.protocols, p.protocol) {
				p.AddPeer(ch.id)
			}
		}
	}()

	go func() {
		defer wg.Done()

		for peer := range peerDisconnectedCh {
			p.RemovePeer(peer)
		}
	}()

	return nil
}

func (p *p2pPeerPoolManager) AddPeer(id peer.ID) {
	if !slices.Contains(p.blockSyncPeers, id) {
		p.blockSyncPeers = append(p.blockSyncPeers, id)

		p.notifyPeerUpdated()
	}
}

func (p *p2pPeerPoolManager) RemovePeer(id peer.ID) {
	if slices.Contains(p.blockSyncPeers, id) {
		p.blockSyncPeers = append(p.blockSyncPeers, id)

		newlist := make([]peer.ID, 0, len(p.blockSyncPeers))
		for _, peer := range p.blockSyncPeers {
			if peer == id {
				continue
			}
			newlist = append(newlist, peer)
		}
		p.blockSyncPeers = newlist

		p.notifyPeerUpdated()
	}
}

func (p *p2pPeerPoolManager) notifyPeerUpdated() {
	select {
	case p.syncPeerUpdateChan <- 0:
	default:
	}
}

func (p *p2pPeerPoolManager) OpenStream(ctx context.Context) (network.Stream, func(), error) {
	pr, err := p.pickBlockSyncPeer(ctx)
	if err != nil {
		return nil, nil, err
	}

	stream, err := p.p2p.NewStream(ctx, *pr, p.protocol)
	return stream, func() {
		p.releaseBlockSyncPeer(pr)
	}, err
}

func (p *p2pPeerPoolManager) pickBlockSyncPeer(ctx context.Context) (*peer.ID, error) {
	for {
		pr := p.pickBlockSyncPeerNoWait()
		if pr != nil {
			return pr, nil
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
	p.syncPeerMtx.Lock()
	defer p.syncPeerMtx.Unlock()

	// Simple mechanism to round robin the peers
	p.peerTurn += 1

	for i := 0; i < len(p.blockSyncPeers); i++ {
		pr := p.blockSyncPeers[(i+p.peerTurn)%len(p.blockSyncPeers)]
		if p.pickedBlockSyncPeers[pr] < p.maxConcurrentRequestPerPeer {
			p.pickedBlockSyncPeers[pr] += 1
			return &pr
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
