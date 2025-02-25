package juno

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	stdsync "sync"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc"
)

type Adapter interface {
	AdaptBlockHeader(header *core.Header) rpcv6.BlockHeader
}

type v6Adapter struct{}

var V6Adapter Adapter = v6Adapter{}

type v7Adapter struct{}

var V7Adapter Adapter = v7Adapter{}

type Handler struct {
	bcReader      blockchain.Reader
	syncReader    sync.Reader
	idgen         func() uint64
	subscriptions stdsync.Map // map[uint64]*subscription
	newHeads      *feed.Feed[*core.Header]
	log           utils.Logger
	version       string
}

type subscription struct {
	cancel func()
	wg     conc.WaitGroup
	conn   jsonrpc.Conn
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, version string, logger utils.Logger) *Handler {
	return &Handler{
		bcReader:   bcReader,
		syncReader: syncReader,
		log:        logger,
		idgen: func() uint64 {
			var n uint64
			for err := binary.Read(rand.Reader, binary.LittleEndian, &n); err != nil; {
			}
			return n
		},
		version:  version,
		newHeads: feed.New[*core.Header](),
	}
}

func (h *Handler) WithIDGen(idgen func() uint64) *Handler {
	h.idgen = idgen
	return h
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}

func (h *Handler) Run(ctx context.Context) error {
	newHeadsSub := h.syncReader.SubscribeNewHeads().Subscription
	defer newHeadsSub.Unsubscribe()
	feed.Tee(newHeadsSub, h.newHeads)
	<-ctx.Done()
	h.subscriptions.Range(func(key, value any) bool {
		sub := value.(*subscription)
		sub.wg.Wait()
		return true
	})
	return nil
}
