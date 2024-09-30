package p2p

import (
	"context"
	"os"
	"sync/atomic"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/host"
)

type Downloader struct {
	isFeeder   bool
	mode       atomic.Uint32
	baseSyncer *SyncService
	snapSyncer *SnapSyncer
	log        utils.SimpleLogger
}

func NewDownloader(isFeeder bool, syncMode SyncMode, p2pHost host.Host, network *utils.Network, bc *blockchain.Blockchain, log utils.SimpleLogger) *Downloader {
	dl := &Downloader{
		isFeeder: isFeeder,
		log:      log,
	}

	dl.baseSyncer = newSyncService(bc, p2pHost, network, log)

	var snapSyncer *SnapSyncer
	if syncMode == SnapSync {
		snapSyncer = NewSnapSyncer(dl.baseSyncer.Client(), bc, log)
	}
	dl.snapSyncer = snapSyncer

	// TODO: when syncing becomes more mature, we need a way to dynamically determine which sync mode to use
	// For now, we will use the sync mode that is passed in the constructor
	dl.mode.Store(uint32(syncMode))

	return dl
}

func (d *Downloader) Start(ctx context.Context) error {
	// Feeder node doesn't sync using P2P
	if d.isFeeder {
		return nil
	}

	d.log.Infow("Downloader start", "mode", d.getMode())
	if d.getMode() == SnapSync {
		// TODO: a hack, remove this
		if os.Getenv("JUNO_P2P_NO_SYNC") == "" {
			err := d.snapSyncer.Run(ctx)
			if err != nil {
				d.log.Errorw("Snapsyncer failed to start")
				return err
			}
		} else {
			d.log.Infow("Syncing is disabled")
			return nil
		}
	}

	d.baseSyncer.Start(ctx)
	return nil
}

func (d *Downloader) getMode() SyncMode {
	return SyncMode(d.mode.Load())
}

func (d *Downloader) WithListener(l sync.EventListener) {
	d.baseSyncer.WithListener(l)
}
