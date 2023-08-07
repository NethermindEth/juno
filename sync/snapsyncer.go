package sync

import (
	"context"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"
	big "math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
)

type SnapSyncher struct {
	baseSync     service.Service
	starknetData starknetdata.StarknetData
	snapServer   *reliableSnapServer
	blockchain   *blockchain.Blockchain
	log          utils.Logger

	startingBlock    *core.Header
	lastBlock        *core.Header
	currentStateRoot *felt.Felt
	currentClassRoot *felt.Felt

	addressRangeDone chan interface{}
	storageRangeDone chan interface{}
	largeStoreDone   chan interface{}

	storageRangeJobCount int32
	storageRangeJob      chan *storageRangeJob

	largeStorageRangeJobCount int32
	largeStorageRangeJob      chan *blockchain.StorageRangeRequest
	largeStorageStoreJob      chan *largeStorageStoreJob

	classesJob chan *felt.Felt

	// Three lock priority lock
	mtxM *sync.Mutex
	mtxN *sync.Mutex
	mtxL *sync.Mutex
}

type storageRangeJob struct {
	snapServerRequest blockchain.StorageRangeRequest
	classHash         *felt.Felt
	nonce             *felt.Felt
}

type largeStorageStoreJob struct {
	storagePath *felt.Felt
	changes     []core.StorageDiff
}

func NewSnapSyncer(
	baseSyncher service.Service,
	consensus starknetdata.StarknetData,
	server blockchain.SnapServer,
	blockchain *blockchain.Blockchain,
	log utils.Logger,
) *SnapSyncher {
	return &SnapSyncher{
		baseSync:     baseSyncher,
		starknetData: consensus,
		snapServer: &reliableSnapServer{
			innerServer: server,
			log:         log,
		},
		blockchain: blockchain,
		log:        log,
	}
}

var (
	addressDurations = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "juno_address_durations",
		Help:    "Time in address get",
		Buckets: prometheus.ExponentialBuckets(1.0, 1.7, 30),
	}, []string{"phase"})
	storageDurations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "juno_storage_durations",
		Help: "Time in address get",
	}, []string{"phase"})
	storageStoreSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "juno_storage_store_size",
		Help: "Time in address get",
	})
	storageStoreSizeTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "juno_storage_store_size_total",
		Help: "Time in address get",
	})

	largeStorageDurations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "juno_large_storage_durations",
		Help: "Time in address get",
	}, []string{"phase"})
	largeStorageStoreSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "juno_large_storage_store_size",
		Help: "Time in address get",
	})

	largeStorageStoreJobSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "juno_large_storage_store_job_size",
		Help: "Time in address get",
	})
	rangeProgress = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "juno_range_progress",
		Help: "Time in address get",
	})

	largeStorageStoreJobSizeTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "juno_large_storage_store_job_size_total",
		Help: "Time in address get",
	})
	pivotUpdates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "juno_pivot_update",
		Help: "Time in address get",
	})

	updateContractTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "juno_updated_contract_totals",
		Help: "Time in address get",
	}, []string{"location"})

	storageLeafSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "juno_storage_leaf_size",
		Help:    "Time in address get",
		Buckets: prometheus.ExponentialBuckets(1.0, 1.5, 30),
	})
	storageLargeLeafSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "juno_storage_large_leaf_size",
		Help:    "Time in address get",
		Buckets: prometheus.ExponentialBuckets(1.0, 1.5, 30),
	})
)

var (
	storageJobWorker    = 4
	storageBatchSize    = 2000
	storageMaxNodes     = 100000
	storageJobQueueSize = storageJobWorker * storageBatchSize // Too high and the progress from address range would be inaccurate.

	classRangeMaxNodes   = 10000
	addressRangeMaxNodes = 5000

	largeStorageJobWorker      = runtime.NumCPU() * 2 // Large storage are largest and most parallelizable. So we want a lot of this.
	largeStorageMaxNodes       = 4000
	largeStorageJobQueueSize   = 4 // They are usually very slow. So more does not really do anything.
	largeStorageStoreQueueSize = 0 // We want the large storage to be throttled by large store

	fetchClassWorkerCount = 8 // Fairly parallelizable. But this is brute force...
	classesJobQueueSize   = 128

	maxPivotDistance     = 1 // Set to 1 to test updated storage.
	newPivotHeadDistance = uint64(1)

	storePerContractBatchSize         = 500 // For some reason, the trie throughput is higher if the batch size is small.
	storeMaxConcurrentContractTrigger = runtime.NumCPU()
	storeMaxTotalJobTrigger           = int(float64(storeMaxConcurrentContractTrigger*storePerContractBatchSize) * 4)
)

func (s *SnapSyncher) getNextStartingBlock(ctx context.Context) (*core.Block, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:

		}

		head, err := s.starknetData.BlockLatest(ctx)
		if err != nil {
			s.log.Warnw("error getting current head", "error", err)
			continue
		}
		startingBlock, err := s.starknetData.BlockByNumber(ctx, head.Number-newPivotHeadDistance)
		if err != nil {
			s.log.Warnw("error getting starting block", "error", err)
			continue
		}

		return startingBlock, nil
	}
}

func (s *SnapSyncher) initState(ctx context.Context) error {
	startingBlock, err := s.getNextStartingBlock(ctx)
	if err != nil {
		return errors.Join(err, errors.New("error getting current head"))
	}

	s.startingBlock = startingBlock.Header
	s.lastBlock = startingBlock.Header

	rootInfo, err := s.snapServer.GetTrieRootAt(ctx, s.startingBlock)
	if err != nil {
		return errors.Join(err, errors.New("error getting trie root"))
	}
	s.currentStateRoot = rootInfo.StorageRoot
	s.currentClassRoot = rootInfo.ClassRoot

	s.storageRangeJobCount = 0
	s.storageRangeJob = make(chan *storageRangeJob, storageJobQueueSize)
	s.largeStorageRangeJobCount = 0
	s.largeStorageRangeJob = make(chan *blockchain.StorageRangeRequest, largeStorageJobQueueSize)
	s.largeStorageStoreJob = make(chan *largeStorageStoreJob, largeStorageStoreQueueSize)
	s.classesJob = make(chan *felt.Felt, classesJobQueueSize)

	s.addressRangeDone = make(chan interface{})
	s.storageRangeDone = make(chan interface{})
	s.largeStoreDone = make(chan interface{})

	s.mtxM = &sync.Mutex{}
	s.mtxN = &sync.Mutex{}
	s.mtxL = &sync.Mutex{}

	return nil
}

func (s *SnapSyncher) Run(ctx context.Context) error {
	err := s.runPhase1(ctx)
	if err != nil {
		return err
	}

	for i := s.startingBlock.Number; i <= s.lastBlock.Number; i++ {
		s.log.Infow("applying block", "blockNumber", i, "lastBlock", s.lastBlock.Number)

		err = s.ApplyStateUpdate(uint64(i))
		if err != nil {
			return errors.Join(err, errors.New("error applying state update"))
		}
	}

	err = s.verifyTrie(ctx)
	if err != nil {
		return err
	}

	s.log.Infow("delegating to standard synchronizer")
	return s.baseSync.Run(ctx)
}

func (s *SnapSyncher) runPhase1(ctx context.Context) error {
	s.log.Infow("starting snap sync")
	// 1. Get the current head
	// 2. Start the snap sync with pivot set to that head
	// 3. If at any moment, if:
	//    a. The current head is too new (more than 64 block let say)
	//    b. Too many missing node
	//    then reset the pivot.
	// 4. Once finished, replay state update from starting pivot to the latest pivot.
	// 5. Then do some cleanup, mark things and complete and such.
	// 6. Probably download old state updato/bodies too
	// 7. Send back control to base sync.

	starttime := time.Now()

	err := s.initState(ctx)
	if err != nil {
		return errors.Join(err, errors.New("error initializing snap syncer state"))
	}

	eg, ectx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		defer func() {
			s.log.Infow("pool latest block done")
			if err := recover(); err != nil {
				s.log.Errorw("latest block pool paniced", "err", err)
			}
		}()

		return s.poolLatestBlock(ectx)
	})

	eg.Go(func() error {
		defer func() {
			if err := recover(); err != nil {
				s.log.Errorw("class range paniced", "err", err)
			}
		}()

		err := s.runClassRangeWorker(ectx)
		if err != nil {
			s.log.Errorw("error in class range worker", "err", err)
		}

		return err
	})

	eg.Go(func() error {
		defer func() {
			if err := recover(); err != nil {
				s.log.Errorw("address range paniced", "err", err)
			}
		}()

		err := s.runAddressRangeWorker(ectx)
		if err != nil {
			s.log.Errorw("error in address range worker", "err", err)
		}

		close(s.addressRangeDone)
		close(s.classesJob)

		return err
	})

	storageEg, sctx := errgroup.WithContext(ectx)
	for i := 0; i < storageJobWorker; i++ {
		i := i
		storageEg.Go(func() error {
			defer func() {
				if err := recover(); err != nil {
					s.log.Errorw("storage worker paniced", "err", err)
				}
			}()

			err := s.runStorageRangeWorker(sctx, i)
			if err != nil {
				s.log.Errorw("error in storage range worker", "err", err)
			}
			s.log.Infow("Storage worker completed", "workerId", i)

			return err
		})
	}

	// For notifying that storage range is done
	eg.Go(func() error {
		err := storageEg.Wait()
		if err != nil {
			return err
		}

		s.log.Infow("Storage range range completed")
		close(s.storageRangeDone)
		return nil
	})

	lStorageEg, lctx := errgroup.WithContext(ectx)
	for i := 0; i < largeStorageJobWorker; i++ {
		i := i
		lStorageEg.Go(func() error {
			defer func() {
				if err := recover(); err != nil {
					s.log.Errorw("large storage worker paniced", "err", err)
				}
			}()

			err := s.runLargeStorageRangeWorker(lctx, i)
			if err != nil && !errors.Is(err, context.Canceled) {
				s.log.Errorw("error in large storage range worker", "err", err)
			}
			s.log.Infow("Large storage worker completed", "workerId", i)
			return err
		})
	}

	eg.Go(func() error {
		err := lStorageEg.Wait()
		s.log.Infow("All larges storage worker completed")
		close(s.largeStorageStoreJob)
		close(s.largeStoreDone)
		return err
	})

	for i := 0; i < 1; i++ {
		eg.Go(func() error {
			err := s.runLargeStorageStore(ectx, 1)
			if err != nil && !errors.Is(err, context.Canceled) {
				s.log.Errorw("large storage store failed", "err", err)
			}
			return err
		})
	}

	for i := 0; i < fetchClassWorkerCount; i++ {
		i := i
		eg.Go(func() error {
			err := s.runFetchClassJob(ectx)
			if err != nil {
				s.log.Errorw("fetch class failed", "err", err)
			}
			s.log.Infow("fetch class completed", "workerId", i)
			return err
		})
	}

	err = eg.Wait()
	if err != nil {
		return err
	}

	s.log.Infow("first phase completed", "duration", time.Now().Sub(starttime).String())

	return nil
}

func calculatePercentage(f *felt.Felt) uint64 {
	maxint := big.NewInt(1)
	maxint.Lsh(maxint, 251)

	theint := f.BigInt(big.NewInt(0))
	theint.Mul(theint, big.NewInt(100))
	theint.Div(theint, maxint)

	return theint.Uint64()
}

func (s *SnapSyncher) runClassRangeWorker(ctx context.Context) error {
	totaladded := 0
	startAddr := &felt.Zero
	for {
		classRoot := s.currentClassRoot
		if classRoot == nil || classRoot.IsZero() {
			s.log.Infow("no class root", "progress", calculatePercentage(startAddr))
			return nil
		}

		s.log.Infow("class range progress", "progress", calculatePercentage(startAddr))

		hasNext, response, err := s.snapServer.GetClassRange(ctx, classRoot, startAddr, nil, uint64(classRangeMaxNodes))
		if err != nil {
			return errors.Join(err, errors.New("error get address range"))
		}

		err = s.SetClasss(response.Paths, response.ClassCommitments)
		if err != nil {
			return errors.Join(err, errors.New("error setting class"))
		}

		for _, path := range response.Paths {
			err := s.queueClassJob(ctx, path)
			if err != nil {
				return err
			}
		}

		startAddr = response.Paths[len(response.Paths)-1]
		totaladded += len(response.Paths)

		if !hasNext {
			break
		}
	}

	s.log.Infow("class range completed", "totalClass", totaladded)
	return nil
}

func (s *SnapSyncher) runAddressRangeWorker(ctx context.Context) error {
	startAddr := &felt.Zero
	for {
		s.log.Infow("snap range progress", "progress", calculatePercentage(startAddr))
		rangeProgress.Set(float64(calculatePercentage(startAddr)))

		hasNext, response, _, err := s.snapServer.GetAddressRange(ctx, s.currentStateRoot, startAddr, nil, uint64(addressRangeMaxNodes)) // Verify is slow.
		if err != nil {
			return errors.Join(err, errors.New("error get address range"))
		}

		classHashes := make([]*felt.Felt, 0)
		nonces := make([]*felt.Felt, 0)
		for i := range response.Paths {
			classHashes = append(classHashes, response.Leaves[i].ClassHash)
			nonces = append(nonces, response.Leaves[i].Nonce)
		}

		starttime := time.Now()
		for i, path := range response.Paths {
			err := s.queueStorageRangeJob(ctx, path, response, i)
			if err != nil {
				return err
			}
		}

		addressDurations.WithLabelValues("queueing").Observe(float64(time.Now().Sub(starttime).Microseconds()))
		starttime = time.Now()

		for i, _ := range response.Paths {
			err := s.queueClassJob(ctx, response.Leaves[i].ClassHash)
			if err != nil {
				return err
			}
		}

		startAddr = response.Paths[len(response.Paths)-1]
		addressDurations.WithLabelValues("class_queueing").Observe(float64(time.Now().Sub(starttime).Microseconds()))

		if !hasNext {
			break
		}
	}

	s.log.Infow("address range completed")

	return nil
}

func (s *SnapSyncher) queueClassJob(ctx context.Context, classHash *felt.Felt) error {
	queued := false
	for !queued {
		select {
		case s.classesJob <- classHash:
			queued = true
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			s.log.Infow("address queue stall on class")
		}
	}
	return nil
}

func (s *SnapSyncher) queueStorageRangeJob(ctx context.Context, path *felt.Felt, response *blockchain.AddressRangeResult, i int) error {
	queued := false
	for !queued {
		select {
		case s.storageRangeJob <- &storageRangeJob{
			snapServerRequest: blockchain.StorageRangeRequest{
				Path:      path,
				Hash:      response.Leaves[i].ContractStorageRoot,
				StartAddr: &felt.Zero,
			},
			classHash: response.Leaves[i].ClassHash,
			nonce:     response.Leaves[i].Nonce,
		}:
			queued = true
			atomic.AddInt32(&s.storageRangeJobCount, 1)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			s.log.Infow("address queue stall")
		}
	}
	return nil
}

func (s *SnapSyncher) runStorageRangeWorker(ctx context.Context, workerIdx int) error {
	totalprocessed := 0
	nextjobs := make([]*storageRangeJob, 0)
	for {
		jobs := nextjobs

	requestloop:
		for len(jobs) < storageBatchSize {
			addressdonechecker := s.addressRangeDone
			if s.storageRangeJobCount > 0 {
				addressdonechecker = nil // So that it never complete as there are job to be done
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second * 10):
				if len(jobs) > 0 {
					break requestloop
				}
				s.log.Infow("waiting for more storage job", "count", s.storageRangeJobCount)
			case <-addressdonechecker:
				// Its done...
				return nil
			case job := <-s.storageRangeJob:
				jobs = append(jobs, job)
			}
		}

		requests := make([]*blockchain.StorageRangeRequest, 0)
		for _, job := range jobs {
			requests = append(requests, &job.snapServerRequest)
		}

		curstateroot := s.currentStateRoot

		s.log.Infow("storage range", "rootDistance", s.lastBlock.Number-s.startingBlock.Number, "root", curstateroot.String(), "requestcount", len(requests))

		starttime := time.Now()
		responses, hasNexes, err := s.snapServer.GetContractRange(ctx, curstateroot, requests, uint64(storageMaxNodes), 20)
		storageDurations.WithLabelValues("get").Add(float64(time.Now().Sub(starttime).Microseconds()))
		if err != nil {
			return err
		}

		// Requeue job without response
		// TODO: Just slice?
		nextjobs = make([]*storageRangeJob, 0)
		for i := len(responses); i < len(jobs); i++ {
			unprocessedRequest := jobs[i]
			nextjobs = append(nextjobs, unprocessedRequest)
		}

		totalSize := 0
		allDiffs := map[felt.Felt][]core.StorageDiff{}
		nonces := map[felt.Felt]*felt.Felt{}
		classHashes := map[felt.Felt]*felt.Felt{}
		largeStorageRequests := make([]*blockchain.StorageRangeRequest, 0)

		for i, response := range responses {
			request := requests[i]
			job := jobs[i]
			storageLeafSize.Observe(float64(len(response.Paths)))

			if response.UpdatedContract != nil {
				updateContractTotal.WithLabelValues("storage").Inc()
				job.nonce = response.UpdatedContract.Nonce
				job.classHash = response.UpdatedContract.ClassHash
			}

			if len(response.Paths) == 0 {
				if !request.Hash.Equal(&felt.Zero) {
					return fmt.Errorf("empty path got non zero hash")
				}
				// TODO: need to check if its really empty
				atomic.AddInt32(&s.storageRangeJobCount, -1)
				totalprocessed++

				allDiffs[*request.Path] = nil
				nonces[*request.Path] = job.nonce
				classHashes[*request.Path] = job.classHash

				continue
			}

			diffs := make([]core.StorageDiff, 0)
			for i, path := range response.Paths {
				diffs = append(diffs, core.StorageDiff{
					Key:   path,
					Value: response.Values[i],
				})
			}
			totalSize += len(diffs)

			allDiffs[*request.Path] = diffs
			nonces[*request.Path] = job.nonce
			classHashes[*request.Path] = job.classHash

			if hasNexes[i] {
				request.StartAddr = diffs[len(diffs)-1].Key
				largeStorageRequests = append(largeStorageRequests, request)
			}

			atomic.AddInt32(&s.storageRangeJobCount, -1)
			totalprocessed++
		}

		storageStoreSize.Set(float64(len(allDiffs)))

		starttime = time.Now()
		err = s.SetStorage(allDiffs, classHashes, nonces, true, false)
		storageDurations.WithLabelValues("set").Add(float64(time.Now().Sub(starttime).Microseconds()))
		starttime = time.Now()

		// Need to be after SetStorage or some contract would not be deployed yet.
		for _, request := range largeStorageRequests {
			err := s.enqueueLargeStorageRangeJob(ctx, request)
			if err != nil {
				return err
			}
		}
		storageDurations.WithLabelValues("queueing").Add(float64(time.Now().Sub(starttime).Microseconds()))

		if err != nil {
			return err
		}

	}
}

func (s *SnapSyncher) enqueueLargeStorageRangeJob(ctx context.Context, request *blockchain.StorageRangeRequest) error {
	queued := false
	for !queued {
		select {
		case s.largeStorageRangeJob <- request:
			queued = true
			atomic.AddInt32(&s.largeStorageRangeJobCount, 1)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			s.log.Infow("torage queue stall")
		}
	}
	return nil
}

func (s *SnapSyncher) runLargeStorageRangeWorker(ctx context.Context, workerIdx int) error {
	for {
		storageRangeDone := s.storageRangeDone
		if s.largeStorageRangeJobCount > 0 {
			storageRangeDone = nil // So that it never complete
		}

		var job *blockchain.StorageRangeRequest

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-storageRangeDone:
			// Its done...
			return nil
		case job = <-s.largeStorageRangeJob:
		case <-time.After(time.Second * 5):
			continue
		}

		s.log.Infow("new large storage job", "path", job.Path.String(), "remainingJob", s.largeStorageRangeJobCount)

		err := s.fetchLargeStorageSlot(ctx, workerIdx, job)
		if err != nil {
			return err
		}

		atomic.AddInt32(&s.largeStorageRangeJobCount, -1)
	}
}

func (s *SnapSyncher) fetchLargeStorageSlot(ctx context.Context, workerIdx int, job *blockchain.StorageRangeRequest) error {
	startAddr := job.StartAddr
	hasNext := true
	totalPaths := 0
	for hasNext {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		s.log.Infow("large storage", "workerId", workerIdx, "path", job.Path, "percentage", calculatePercentage(startAddr))

		job.StartAddr = startAddr
		starttime := time.Now()
		curstateroot := s.currentStateRoot
		responses, hasNexes, err := s.snapServer.GetContractRange(ctx, curstateroot, []*blockchain.StorageRangeRequest{job}, uint64(largeStorageMaxNodes), uint64(largeStorageMaxNodes))
		largeStorageDurations.WithLabelValues("get").Add(float64(time.Now().Sub(starttime).Microseconds()))
		starttime = time.Now()
		if err != nil {
			return errors.Join(errors.New("fetch failed"), err)
		}

		response := responses[0] // TODO: it can return nothing
		hasNext = hasNexes[0]

		if response.UpdatedContract != nil {
			updateContractTotal.WithLabelValues("lstorage").Inc()

			job.Hash = response.UpdatedContract.ContractStorageRoot

			diffs := map[felt.Felt][]core.StorageDiff{
				*job.Path: nil,
			}
			classes := map[felt.Felt]*felt.Felt{
				*job.Path: response.UpdatedContract.ClassHash,
			}
			nonces := map[felt.Felt]*felt.Felt{
				*job.Path: response.UpdatedContract.Nonce,
			}

			err = s.SetStorage(diffs, classes, nonces, true, false)
			if err != nil {
				return errors.Join(err, errors.New("unable to update updated contract"))
			}
		}

		diffs := make([]core.StorageDiff, len(response.Paths))
		for i, path := range response.Paths {
			diffs[i] = core.StorageDiff{
				Key:   path,
				Value: response.Values[i],
			}
		}

		select {
		case s.largeStorageStoreJob <- &largeStorageStoreJob{
			storagePath: job.Path,
			changes:     diffs,
		}:
		case <-ctx.Done():
		}

		largeStorageDurations.WithLabelValues("queue").Add(float64(time.Now().Sub(starttime).Microseconds()))

		startAddr = response.Paths[len(response.Paths)-1]
		totalPaths += len(diffs)
	}

	storageLargeLeafSize.Observe(float64(totalPaths))

	return nil
}

func (s *SnapSyncher) runLargeStorageStore(ctx context.Context, workerId int) error {
	// Attempt to buffer the jobs into maps of path and diffs. This is done because it can be parallelized.
	// I did try other more complicated technique but unable to make it significantly faster.
	curmap := map[felt.Felt][]core.StorageDiff{}
	totalChanges := 0
	for job := range s.largeStorageStoreJob {
		totalChanges += len(job.changes)

		curmap[*job.storagePath] = append(curmap[*job.storagePath], job.changes...)
		length := len(curmap)

		for totalChanges > storeMaxTotalJobTrigger || length > storeMaxConcurrentContractTrigger {
			// In an effort to improve parallelism, we try to limit each contract to a batch size so that no
			// single contract take too much time, delaying other contract.
			tostore := map[felt.Felt][]core.StorageDiff{}
			for k, diffs := range curmap {
				if len(diffs) > storePerContractBatchSize {
					tostore[k] = diffs[:storePerContractBatchSize]
				} else {
					tostore[k] = diffs
				}
			}

			jobsize := 0
			for k, v := range tostore {
				jobsize += len(tostore[k])
				if len(curmap[k]) > len(v) {
					curmap[k] = curmap[k][len(v):]
				} else {
					delete(curmap, k)
				}
			}
			length = len(curmap)

			largeStorageStoreSize.Set(float64(len(tostore)))
			largeStorageStoreJobSize.Set(float64(jobsize))
			totalChanges -= jobsize

			starttime := time.Now()
			err := s.SetStorage(tostore, nil, nil, false, true)
			largeStorageDurations.WithLabelValues("set").Add(float64(time.Now().Sub(starttime).Microseconds()))
			if err != nil {
				return errors.Join(err, errors.New("error storing large storage"))
			}
			// s.log.Infow("large storage store", "workerId", workerId, "size", len(curmap), "total", jobsize, "length", length, "totalChanges", totalChanges, "time", time.Now().Sub(starttime))
		}
	}

	s.log.Infow("final store", "workerId", workerId)
	for len(curmap) > 0 {
		// In an effort to improve parallelism, we try to limit each contract to a batch size so that no
		// single contract take too much time, delaying other contract.
		tostore := map[felt.Felt][]core.StorageDiff{}
		for k, diffs := range curmap {
			if len(diffs) > storePerContractBatchSize {
				tostore[k] = diffs[:storePerContractBatchSize]
			} else {
				tostore[k] = diffs
			}
		}

		jobsize := 0
		for k, v := range tostore {
			jobsize += len(tostore[k])
			if len(curmap[k]) > len(v) {
				curmap[k] = curmap[k][len(v):]
			} else {
				delete(curmap, k)
			}
		}

		largeStorageStoreSize.Set(float64(len(tostore)))
		largeStorageStoreJobSize.Set(float64(jobsize))
		totalChanges -= jobsize

		starttime := time.Now()
		err := s.SetStorage(tostore, nil, nil, false, true)
		largeStorageDurations.WithLabelValues("set").Add(float64(time.Now().Sub(starttime).Microseconds()))
		if err != nil {
			return errors.Join(err, errors.New("error storing large storage"))
		}
		s.log.Infow("large storage store", "workerId", workerId, "size", len(curmap), "total", jobsize, "totalChanges", totalChanges, "time", time.Now().Sub(starttime))
	}
	s.log.Infow("large storage store job completed", "workerId", workerId)

	return nil
}

func (s *SnapSyncher) poolLatestBlock(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second * 10):
			break
		case <-s.largeStoreDone:
			return nil
		}

		newTarget, err := s.getNextStartingBlock(ctx)
		if err != nil {
			return errors.Join(err, errors.New("error getting current head"))
		}

		// TODO: Race issue
		if newTarget.Number-s.lastBlock.Number < uint64(maxPivotDistance) {
			s.log.Infow("Not updating pivot yet", "lastblock", s.lastBlock.Number, "newTarget", newTarget.Number, "diff", newTarget.Number-s.lastBlock.Number)
			continue
		}

		pivotUpdates.Inc()

		s.log.Infow("Switching snap pivot", "hash", newTarget.Hash, "number", newTarget.Number)
		s.lastBlock = newTarget.Header

		rootInfo, err := s.snapServer.GetTrieRootAt(ctx, s.lastBlock)
		if err != nil {
			return errors.Join(err, errors.New("error getting trie root"))
		}
		s.currentStateRoot = rootInfo.StorageRoot
		s.currentClassRoot = rootInfo.ClassRoot
	}
}

func (s *SnapSyncher) ApplyStateUpdate(blockNumber uint64) error {
	ctx := context.Background()

	update, err := s.starknetData.StateUpdate(ctx, blockNumber)
	if err != nil {
		return errors.Join(err, errors.New("error fetching state update"))
	}

	unknownClasses, err := s.fetchUnknownClasses(ctx, update)
	if err != nil {
		return err
	}

	block, err := s.starknetData.BlockByNumber(ctx, blockNumber)
	if err != nil {
		return err
	}

	return s.blockchain.StoreNoVerify(block, update, unknownClasses)
}

func (s *SnapSyncher) fetchUnknownClasses(ctx context.Context, stateUpdate *core.StateUpdate) (map[felt.Felt]core.Class, error) {
	state, closer, err := s.blockchain.HeadState()
	if err != nil {
		// if err is db.ErrKeyNotFound we are on an empty DB
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		closer = func() error {
			return nil
		}
	}

	newClasses := make(map[felt.Felt]core.Class)
	fetchIfNotFound := func(classHash *felt.Felt) error {
		if _, ok := newClasses[*classHash]; ok {
			return nil
		}

		stateErr := db.ErrKeyNotFound
		if state != nil {
			_, stateErr = state.Class(classHash)
		}

		if errors.Is(stateErr, db.ErrKeyNotFound) {
			class, fetchErr := s.starknetData.Class(ctx, classHash)
			if fetchErr == nil {
				newClasses[*classHash] = class
			}
			return fetchErr
		}
		return stateErr
	}

	for _, deployedContract := range stateUpdate.StateDiff.DeployedContracts {
		if err = fetchIfNotFound(deployedContract.ClassHash); err != nil {
			return nil, db.CloseAndWrapOnError(closer, err)
		}
	}
	for _, classHash := range stateUpdate.StateDiff.DeclaredV0Classes {
		if err = fetchIfNotFound(classHash); err != nil {
			return nil, db.CloseAndWrapOnError(closer, err)
		}
	}
	for _, declaredV1 := range stateUpdate.StateDiff.DeclaredV1Classes {
		if err = fetchIfNotFound(declaredV1.ClassHash); err != nil {
			return nil, db.CloseAndWrapOnError(closer, err)
		}
	}

	return newClasses, db.CloseAndWrapOnError(closer, nil)
}

func (s *SnapSyncher) GetStateRoot() (*felt.Felt, error) {
	state, close, err := s.blockchain.HeadState()
	if err == db.ErrKeyNotFound {
		return &felt.Zero, nil
	}
	if err != nil {
		return nil, err
	}

	trie, closer2, err := state.(*core.State).StorageTrie()
	if err != nil {
		return nil, err
	}

	root, err := trie.Root()
	if err != nil {
		return nil, err
	}

	closer2()
	close()

	return root, nil
}

func (s *SnapSyncher) SetClasss(paths []*felt.Felt, classCommitments []*felt.Felt) error {
	s.mtxN.Lock()
	s.mtxM.Lock()
	defer s.mtxM.Unlock()
	s.mtxN.Unlock()

	return s.blockchain.StoreClassCommitments(paths, classCommitments)
}

func (s *SnapSyncher) SetStorage(diffs map[felt.Felt][]core.StorageDiff, classes map[felt.Felt]*felt.Felt, nonces map[felt.Felt]*felt.Felt, higherPriority bool, isLargeStore bool) error {
	if !higherPriority {
		s.mtxL.Lock()
		defer s.mtxL.Unlock()
	}

	s.mtxN.Lock()
	s.mtxM.Lock()
	defer s.mtxM.Unlock()
	s.mtxN.Unlock()

	starttime := time.Now()
	err := s.blockchain.StoreStorageDirect(diffs, classes, nonces)

	if isLargeStore {
		jobsize := 0
		for _, storageDiffs := range diffs {
			jobsize += len(storageDiffs)
		}
		largeStorageDurations.WithLabelValues("effective_set").Add(float64(time.Now().Sub(starttime).Microseconds()))
		largeStorageStoreJobSizeTotal.Add(float64(jobsize))
	} else {
		jobsize := 0
		for _, storageDiffs := range diffs {
			jobsize += len(storageDiffs)
		}
		storageDurations.WithLabelValues("effective_set").Add(float64(time.Now().Sub(starttime).Microseconds()))
		storageStoreSizeTotal.Add(float64(jobsize))
	}

	return err

}

func (s *SnapSyncher) runFetchClassJob(ctx context.Context) error {
	keyBatches := make([]*felt.Felt, 0)
	for key := range s.classesJob {
		if key == nil || key.IsZero() {
			// Not sure why...
			continue
		}

		cls, err := s.blockchain.GetClasses([]*felt.Felt{key})
		if err != nil {
			s.log.Infow("error getting class", "err", err)
			return err
		}

		if cls[0] == nil {
			keyBatches = append(keyBatches, key)
		}

		if len(keyBatches) > 1000 {
			classes, err := s.snapServer.GetClasses(ctx, keyBatches)
			if err != nil {
				s.log.Infow("error getting class from outside", "err", err)
				return err
			}

			newBatch := make([]*felt.Felt, 0)
			newClassKeys := make([]*felt.Felt, 0)
			newClasses := make([]core.Class, 0)
			for i, class := range classes {
				if class == nil {
					s.log.Warnw("class %s not found", keyBatches[i])
					newBatch = append(newBatch, keyBatches[i])
					continue
				}

				newClassKeys = append(newClassKeys, keyBatches[i])
				newClasses = append(newClasses, class)
			}

			err = s.blockchain.StoreClasses(newClassKeys, newClasses)
			if err != nil {
				s.log.Infow("error storing class", "err", err)
				return err
			}

			keyBatches = newBatch
		}
	}

	return nil
}

func (s *SnapSyncher) verifyTrie(ctx context.Context) error {

	sr, cl, err := s.blockchain.HeadState()
	if err != nil {
		return err
	}

	defer func() {
		err := cl()
		if err != nil {
			fmt.Printf("error %s\n", err)
		}
	}()

	srs := sr.(*core.State)

	srstate, cl2, err := srs.StorageTrie()
	if err != nil {
		return err
	}

	defer func() {
		err := cl2()
		if err != nil {
			fmt.Printf("error %s\n", err)
		}
	}()
	cstate, cl3, err := srs.ClassTrie()
	if err != nil {
		return err
	}
	defer func() {
		err := cl3()
		if err != nil {
			fmt.Printf("error %s\n", err)
		}
	}()

	sroot, err := srstate.Root()
	if err != nil {
		return err
	}

	croot, err := cstate.Root()
	if err != nil {
		return err
	}

	s.log.Infow("address range completed", "sroot", sroot.String(), "csroot", s.currentStateRoot.String(), "croot", croot.String(), "ccroot", s.currentClassRoot.String())

	mismatched := false
	if !sroot.Equal(s.currentStateRoot) {
		mismatched = true
		fmt.Printf("mismatched state root")
	}
	if !croot.Equal(s.currentClassRoot) {
		mismatched = true
		fmt.Printf("mismatched class root")
	}

	if mismatched {
		addrChanged := make([]string, 0)
		startAddr := &felt.Zero
		for {
			s.log.Infow("double check", "progress", calculatePercentage(startAddr))
			rangeProgress.Set(float64(calculatePercentage(startAddr)))

			hasNext, response, hashes, err := s.snapServer.GetAddressRange(ctx, s.currentStateRoot, startAddr, nil, 4000) // Verify is slow.
			if err != nil {
				return errors.Join(err, errors.New("error get address range"))
			}

			for i, path := range response.Paths {
				h, err := srstate.Get(path)
				if err != nil {
					return errors.Join(err, errors.New("error get address range"))
				}

				if !h.Equal(hashes[i]) {
					ctrk, err := srs.Contract(path)
					if err != nil {
						return errors.Join(err, errors.New("unable to open contract"))
					}

					nonce, _ := ctrk.Nonce()
					chash, _ := ctrk.ClassHash()
					strie, _ := ctrk.StorageTrie()
					sroot, _ := strie.Root()

					msg := fmt.Sprintf("Path not same %s: %s vs %s\n", path.String(), h.String(), hashes[i].String())
					fmt.Print(msg)
					addrChanged = append(addrChanged, msg)
					msg = fmt.Sprintf("Nonce %s vs %s\n", response.Leaves[i].Nonce.String(), nonce.String())
					fmt.Print(msg)
					addrChanged = append(addrChanged, msg)
					msg = fmt.Sprintf("ClassHash %s vs %s\n", response.Leaves[i].ClassHash.String(), chash.String())
					fmt.Print(msg)
					addrChanged = append(addrChanged, msg)
					msg = fmt.Sprintf("Sroot %s vs %s\n", response.Leaves[i].ContractStorageRoot.String(), sroot.String())
					fmt.Print(msg)
					addrChanged = append(addrChanged, msg)
				}
			}

			startAddr = response.Paths[len(response.Paths)-1]

			if !hasNext {
				break
			}
		}

		for _, s2 := range addrChanged {
			fmt.Printf(s2)
		}

		if len(addrChanged) > 0 {
			return fmt.Errorf("not same")
		}
	}

	return nil
}

var _ service.Service = (*SnapSyncher)(nil)
