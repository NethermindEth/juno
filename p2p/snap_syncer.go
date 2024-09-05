package p2p

import (
	"context"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/p2p/starknet"
	big "math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"
)

const JobDuration = time.Second * 10

type Blockchain interface {
	GetClasses(felts []*felt.Felt) ([]core.Class, error)
	PutClasses(blockNumber uint64, v1CompiledHashes map[felt.Felt]*felt.Felt, newClasses map[felt.Felt]core.Class) error
	PutContracts(address, nonces, classHash []*felt.Felt) error
	PutStorage(storage map[felt.Felt]map[felt.Felt]*felt.Felt) error
}

var _ Blockchain = (*blockchain.Blockchain)(nil)

type SnapSyncher struct {
	baseSync     service.Service
	starknetData starknetdata.StarknetData
	client       starknet.Client
	blockchain   Blockchain
	log          utils.Logger

	startingBlock          *core.Header
	lastBlock              *core.Header
	currentGlobalStateRoot *felt.Felt

	contractRangeDone chan interface{}
	storageRangeDone  chan interface{}

	storageRangeJobCount int32
	storageRangeJobQueue chan *storageRangeJob
	storageRefreshJob    chan *storageRangeJob

	classesJob chan *felt.Felt

	// Three lock priority lock
	mtxM *sync.Mutex
	mtxN *sync.Mutex
	mtxL *sync.Mutex
}

type storageRangeJob struct {
	path         *felt.Felt
	storageRoot  *felt.Felt
	startAddress *felt.Felt
	classHash    *felt.Felt
	nonce        uint64
}

func NewSnapSyncer(
	baseSyncher service.Service,
	consensus starknetdata.StarknetData,
	client starknet.Client,
	bc *blockchain.Blockchain,
	log utils.Logger,
) *SnapSyncher {
	return &SnapSyncher{
		baseSync:     baseSyncher,
		starknetData: consensus,
		client:       client,
		blockchain:   bc,
		log:          log,
	}
}

var (
	// magic values linter does not like
	start  = 1.0
	factor = 1.5
	count  = 30

	rangeProgress = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "juno_range_progress",
		Help: "Time in address get",
	})

	pivotUpdates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "juno_pivot_update",
		Help: "Time in address get",
	})

	storageAddressCount = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "juno_storage_address_count",
		Help:    "Time in address get",
		Buckets: prometheus.ExponentialBuckets(start, factor, count),
	})
)

var (
	storageJobWorker    = 8
	storageBatchSize    = 500
	storageJobQueueSize = storageJobWorker * storageBatchSize // Too high and the progress from address range would be inaccurate.

	// For some reason, the trie throughput is higher if the batch size is small.
	classRangeChunksPerProof   = 500
	contractRangeChunkPerProof = 500
	storageRangeChunkPerProof  = 500
	maxStorageBatchSize        = 500
	maxMaxPerStorageSize       = 500

	fetchClassWorkerCount = 8 // Fairly parallelizable. But this is brute force...
	classesJobQueueSize   = 128

	maxPivotDistance     = 32        // Set to 1 to test updated storage.
	newPivotHeadDistance = uint64(0) // This should be the reorg depth
)

func (s *SnapSyncher) Run(ctx context.Context) error {
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

	err := s.runPhase1(ctx)
	if err != nil {
		return err
	}

	s.log.Infow("delegating to standard synchronizer")
	return s.baseSync.Run(ctx)
}

//nolint:gocyclo,nolintlint
func (s *SnapSyncher) runPhase1(ctx context.Context) error {
	starttime := time.Now()

	err := s.initState(ctx)
	if err != nil {
		return errors.Join(err, errors.New("error initialising snap syncer state"))
	}

	eg, ectx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		defer func() {
			s.log.Infow("pool latest block done")
			if err := recover(); err != nil {
				s.log.Errorw("latest block pool panicked", "err", err)
			}
		}()

		return s.poolLatestBlock(ectx)
	})

	eg.Go(func() error {
		defer func() {
			if err := recover(); err != nil {
				s.log.Errorw("class range panicked", "err", err)
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

		err := s.runContractRangeWorker(ectx)
		if err != nil {
			s.log.Errorw("error in address range worker", "err", err)
		}

		s.log.Infow("contract range done")
		close(s.contractRangeDone)
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

	eg.Go(func() error {
		defer func() {
			if err := recover(); err != nil {
				s.log.Errorw("storage refresh paniced", "err", err)
			}
		}()

		err := s.runStorageRefreshWorker(ectx)
		if err != nil {
			s.log.Errorw("error in storage refresh worker", "err", err)
		}

		return err
	})

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

	s.log.Infow("first phase completed", "duration", time.Since(starttime))

	return nil
}

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

	fmt.Printf("Start state root is %s\n", s.startingBlock.GlobalStateRoot)
	s.currentGlobalStateRoot = s.startingBlock.GlobalStateRoot.Clone()
	s.storageRangeJobCount = 0
	s.storageRangeJobQueue = make(chan *storageRangeJob, storageJobQueueSize)
	s.classesJob = make(chan *felt.Felt, classesJobQueueSize)

	s.contractRangeDone = make(chan interface{})
	s.storageRangeDone = make(chan interface{})

	s.mtxM = &sync.Mutex{}
	s.mtxN = &sync.Mutex{}
	s.mtxL = &sync.Mutex{}

	return nil
}

func calculatePercentage(f *felt.Felt) uint64 {
	const maxPercent = 100
	maxint := big.NewInt(1)
	maxint.Lsh(maxint, core.GlobalTrieHeight)

	percent := f.BigInt(big.NewInt(0))
	percent.Mul(percent, big.NewInt(maxPercent))
	percent.Div(percent, maxint)

	return percent.Uint64()
}

//nolint:gocyclo,nolintlint
func (s *SnapSyncher) runClassRangeWorker(ctx context.Context) error {
	totaladded := 0
	completed := false
	startAddr := &felt.Zero
	for !completed {
		s.log.Infow("class range progress", "progress", calculatePercentage(startAddr))

		stateRoot := s.currentGlobalStateRoot

		s.log.Infow("class range state root", "stateroot", stateRoot)

		// TODO: Maybe timeout
		classIter, err := s.client.RequestClassRange(ctx, &spec.ClassRangeRequest{
			Root:           core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptHash(startAddr),
			ChunksPerProof: uint32(classRangeChunksPerProof),
		})
		if err != nil {
			return err
		}

		classIter(func(response *spec.ClassRangeResponse) bool {
			if response == nil {
				// State root missing.
				return false
			}
			classWrp := response.GetClasses()
			if classWrp == nil || classWrp.Classes == nil {
				// State root missing.
				return false
			}
			classes := classWrp.Classes
			if response.RangeProof == nil {
				// Proofs missing.
				return false
			}

			s.log.Infow("got", "res", len(classes), "startAdr", startAddr)

			classRoot := p2p2core.AdaptHash(response.ClassesRoot)
			contractRoot := p2p2core.AdaptHash(response.ContractsRoot)
			err = VerifyGlobalStateRoot(stateRoot, classRoot, contractRoot)
			if err != nil {
				s.log.Infow("global state root verification failure")
				// Root verification failed
				// TODO: Ban peer
				return false
			}

			if classRoot.Equal(&felt.Zero) {
				// Special case, no V1 at all
				completed = true
				return false
			}

			paths := make([]*felt.Felt, len(classes))
			values := make([]*felt.Felt, len(classes))
			coreClasses := make([]core.Class, len(classes))

			egrp := errgroup.Group{}

			for i, cls := range classes {
				coreClass := p2p2core.AdaptClass(cls)
				i := i
				egrp.Go(func() error {
					coreClasses[i] = coreClass
					paths[i] = CalculateClassHash(coreClass)
					values[i] = CalculateCompiledClassHash(coreClass)

					// For verification, should be
					// leafValue := crypto.Poseidon(leafVersion, compiledClassHash)
					return nil
				})
			}

			err = egrp.Wait()
			if err != nil {
				return false
			}

			proofs := P2pProofToTrieProofs(response.RangeProof)
			hasNext, err := VerifyTrie(classRoot, paths, values, proofs, core.GlobalTrieHeight, crypto.Poseidon)
			if err != nil {
				// Root verification failed
				// TODO: Ban peer
				return false
			}

			// Ingest
			coreClassesMap := map[felt.Felt]core.Class{}
			coreClassesHashMap := map[felt.Felt]*felt.Felt{}
			for i, coreClass := range coreClasses {
				coreClassesMap[*paths[i]] = coreClass
				coreClassesHashMap[*paths[i]] = values[i]
			}

			err = s.blockchain.PutClasses(s.lastBlock.Number, coreClassesHashMap, coreClassesMap)
			if err != nil {
				return false
			}

			if !hasNext {
				s.log.Infow("class range completed", "totalClass", totaladded)
				completed = true
				return false
			}

			// Increment addr, start loop again
			startAddr = paths[len(paths)-1]

			return true
		})

		if err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocyclo
func (s *SnapSyncher) runFetchClassJob(ctx context.Context) error {
	keyBatches := make([]*felt.Felt, 0)
	for {
	requestloop:
		for len(keyBatches) < 100 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(JobDuration):
				// Just request whatever we have
				if len(keyBatches) > 0 {
					break requestloop
				}
				s.log.Infow("waiting for more class job", "count", s.storageRangeJobCount)
			case key := <-s.classesJob:
				if key == nil {
					// channel finished.
					if len(keyBatches) > 0 {
						break requestloop
					} else {
						// Worker finished
						return nil
					}
				} else {
					if key.Equal(&felt.Zero) {
						continue
					}

					// TODO: Can be done in batches
					cls, err := s.blockchain.GetClasses([]*felt.Felt{key})
					if err != nil {
						s.log.Errorw("error getting class", "err", err)
						return err
					}

					if cls[0] == nil {
						keyBatches = append(keyBatches, key)
					}
				}
			}
		}

		var hashes []*spec.Hash
		for _, key := range keyBatches {
			hashes = append(hashes, core2p2p.AdaptHash(key))
		}

		classIter, err := s.client.RequestClassesByKeys(ctx, &spec.ClassHashesRequest{
			ClassHashes: hashes,
		})

		if err != nil {
			s.log.Errorw("error getting class from client", "err", err)
			return err
		}

		classes := make([]*spec.Class, len(keyBatches))
		classIter(func(response *spec.ClassesResponse) bool {
			switch v := response.ClassMessage.(type) {
			case *spec.ClassesResponse_Class:
				classes = append(classes, v.Class)
				return true
			case *spec.ClassesResponse_Fin:
				return false
			default:
				s.log.Warnw("Unexpected ClassMessage from getClasses", "v", v)
				return false
			}
		})

		processedClasses := map[felt.Felt]bool{}
		newClasses := map[felt.Felt]core.Class{}
		classHashes := map[felt.Felt]*felt.Felt{}
		for i, class := range classes {
			if class == nil {
				s.log.Infow("class empty", "key", keyBatches[i])
				continue
			}

			coreClass := p2p2core.AdaptClass(class)
			newClasses[*keyBatches[i]] = coreClass
			h, err := coreClass.Hash()
			if err != nil {
				s.log.Errorw("error hashing class", "err", err)
				return err
			}

			if !h.Equal(keyBatches[i]) {
				s.log.Warnw("invalid classhash", "got", h, "expected", keyBatches[i])
				return errors.New("invalid class hash")
			}

			if coreClass.Version() == 1 {
				classHashes[*keyBatches[i]] = coreClass.(*core.Cairo1Class).Compiled.Hash()
			}

			processedClasses[*keyBatches[i]] = true
		}

		if len(newClasses) != 0 {
			err = s.blockchain.PutClasses(s.lastBlock.Number, classHashes, newClasses)
			if err != nil {
				s.log.Errorw("error storing class", "err", err)
				return err
			}
		} else {
			s.log.Errorw("Unable to fetch any class from peer")
			// TODO: Penalise peer?
		}

		newBatch := make([]*felt.Felt, 0)
		for _, classHash := range keyBatches {
			if _, ok := processedClasses[*classHash]; !ok {
				newBatch = append(newBatch, classHash)
			}
		}

		keyBatches = newBatch
	}
}

//nolint:gocyclo
func (s *SnapSyncher) runContractRangeWorker(ctx context.Context) error {
	startAddr := &felt.Zero
	completed := false

	for !completed {
		var outherErr error

		stateRoot := s.currentGlobalStateRoot
		iter, err := s.client.RequestContractRange(ctx, &spec.ContractRangeRequest{
			Domain:         0, // What do this do?
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptAddress(startAddr),
			End:            nil, // No need for now.
			ChunksPerProof: uint32(contractRangeChunkPerProof),
		})
		if err != nil {
			return err
		}

		iter(func(response *spec.ContractRangeResponse) bool {
			if response == nil {
				return false
			}
			s.log.Infow("snap range progress", "progress", calculatePercentage(startAddr), "addr", startAddr)
			rangeProgress.Set(float64(calculatePercentage(startAddr)))

			crange := response.GetRange()
			if crange == nil || crange.State == nil {
				return false
			}
			if response.RangeProof == nil {
				return false
			}

			classRoot := p2p2core.AdaptHash(response.ClassesRoot)
			contractRoot := p2p2core.AdaptHash(response.ContractsRoot)
			err := VerifyGlobalStateRoot(stateRoot, classRoot, contractRoot)
			outherErr = err
			if err != nil {
				// Root verification failed
				// TODO: Ban peer
				return false
			}

			paths := make([]*felt.Felt, len(crange.State))
			values := make([]*felt.Felt, len(crange.State))

			for i, rangeValue := range crange.State {
				paths[i] = p2p2core.AdaptAddress(rangeValue.Address)
				values[i] = CalculateRangeValueHash(rangeValue)
			}

			proofs := P2pProofToTrieProofs(response.RangeProof)
			hasNext, err := VerifyTrie(contractRoot, paths, values, proofs, core.GlobalTrieHeight, crypto.Pedersen)
			outherErr = err
			if err != nil {
				// The peer should get penalised in this case
				return false
			}

			classes := []*felt.Felt{}
			nonces := []*felt.Felt{}
			for _, r := range crange.State {
				classHash := p2p2core.AdaptHash(r.Class)
				classes = append(classes, classHash)
				nonces = append(nonces, (&felt.Felt{}).SetUint64(r.Nonce))
			}

			err = s.blockchain.PutContracts(paths, nonces, classes)
			outherErr = err
			if err != nil {
				fmt.Printf("%s\n", err)
				panic(err)
			}

			// We don't actually store it directly here... only put it as part of job.
			// Can't remember why. Could be because it would be some wasted work.
			for _, r := range crange.State {
				path := p2p2core.AdaptAddress(r.Address)
				storageRoot := p2p2core.AdaptHash(r.Storage)
				classHash := p2p2core.AdaptHash(r.Class)
				nonce := r.Nonce

				err = s.queueClassJob(ctx, classHash)
				outherErr = err
				if err != nil {
					return false
				}

				err = s.queueStorageRangeJob(ctx, path, storageRoot, classHash, nonce)
				outherErr = err
				if err != nil {
					return false
				}
			}

			if !hasNext {
				s.log.Infow("address range completed")
				completed = true
				return false
			}

			if len(paths) == 0 {
				return false
			}

			startAddr = paths[len(paths)-1]
			return true
		})

		if outherErr != nil {
			s.log.Errorw("Error with contract range", "err", outherErr)
			// TODO: address the error properly
			// Well... need to figure out how to determine if its a temporary error or not.
			// For sure, the state root can be outdated, so this need to restart
			return outherErr
		}
	}

	return nil
}

//nolint:funlen,gocyclo
func (s *SnapSyncher) runStorageRangeWorker(ctx context.Context, workerIdx int) error {
	nextjobs := make([]*storageRangeJob, 0)
	for {
		jobs := nextjobs

	requestloop:
		for len(jobs) < storageBatchSize {
			contractDoneChecker := s.contractRangeDone
			if s.storageRangeJobCount > 0 {
				contractDoneChecker = nil // So that it never complete as there are job to be done
			}

			select {
			case job := <-s.storageRangeJobQueue:
				jobs = append(jobs, job)
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(JobDuration):
				if len(jobs) > 0 {
					break requestloop
				}
				s.log.Infow("waiting for more storage job", "count", s.storageRangeJobCount)
			case <-contractDoneChecker:
				// Its done...
				return nil
			}
		}

		s.log.Infow("Pending jobs count", "pending", s.storageRangeJobCount)

		requests := make([]*spec.StorageRangeQuery, 0)
		for _, job := range jobs {
			requests = append(requests, &spec.StorageRangeQuery{
				Address: core2p2p.AdaptAddress(job.path),
				Start: &spec.StorageLeafQuery{
					ContractStorageRoot: core2p2p.AdaptHash(job.storageRoot),

					// TODO: Should be address
					Key: core2p2p.AdaptFelt(job.startAddress),
				},
			})
		}

		var err error

		stateRoot := s.currentGlobalStateRoot
		processedJobs := 0
		storage := map[felt.Felt]map[felt.Felt]*felt.Felt{}
		totalPath := 0
		maxPerStorageSize := 0

		s.log.Infow("storage range",
			"rootDistance", s.lastBlock.Number-s.startingBlock.Number,
			"root", stateRoot.String(),
			"requestcount", len(requests),
		)
		iter, err := s.client.RequestStorageRange(ctx, &spec.ContractStorageRequest{
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			ChunksPerProof: uint32(storageRangeChunkPerProof),
			Query:          requests,
		})

		iter(func(response *spec.ContractStorageResponse) bool {
			if response == nil {
				return false
			}
			response.GetResponses()
			csto := response.GetStorage()
			if csto == nil || csto.KeyValue == nil {
				return false
			}
			storageRange := csto.KeyValue

			if response.RangeProof == nil {
				return false
			}

			job := jobs[processedJobs]

			storageAddr := p2p2core.AdaptAddress(response.ContractAddress)
			if !job.path.Equal(storageAddr) {
				s.log.Errorw(fmt.Sprintf(
					"storage addr differ %s %s %d\n", job.path, storageAddr, workerIdx))
				return false
			}

			// Validate response
			paths := make([]*felt.Felt, len(storageRange))
			values := make([]*felt.Felt, len(storageRange))

			for i, v := range storageRange {
				paths[i] = p2p2core.AdaptFelt(v.Key)
				values[i] = p2p2core.AdaptFelt(v.Value)
			}

			proofs := P2pProofToTrieProofs(response.RangeProof)
			hasNext, err := VerifyTrie(job.storageRoot, paths, values, proofs, core.ContractStorageTrieHeight, crypto.Pedersen)
			if err != nil {
				// It is unclear how to distinguish if the peer is malicious/broken/non-bizantine or the contracts root is outdated.
				err = s.queueStorageRefreshJob(ctx, job)
				if err != nil {
					return false
				}

				// Go to next contract
				processedJobs++
				return true
			}

			if storage[*job.path] == nil {
				storage[*job.path] = map[felt.Felt]*felt.Felt{}
			}
			for i, path := range paths {
				storage[*job.path][*path] = values[i]
			}

			totalPath += len(paths)
			if maxPerStorageSize < len(storage[*job.path]) {
				maxPerStorageSize = len(storage[*job.path])
			}

			if totalPath > maxStorageBatchSize || maxPerStorageSize > maxMaxPerStorageSize {
				// Only after a certain amount of path, we store it
				// so that the storing part is more efficient
				storageAddressCount.Observe(float64(len(storage)))
				err = s.blockchain.PutStorage(storage)
				if err != nil {
					s.log.Errorw("error store", "err", err)
					return false
				}

				totalPath = 0
				maxPerStorageSize = 0
				storage = map[felt.Felt]map[felt.Felt]*felt.Felt{}
			}

			if hasNext {
				job.startAddress = paths[len(paths)-1]
			} else {
				processedJobs++
				atomic.AddInt32(&s.storageRangeJobCount, -1) // its... done?
			}

			return true
		})

		if err != nil {
			s.log.Errorw("Error with storage range", "err", err)
			// Well... need to figure out how to determine if its a temporary error or not.
			// For sure, the state root can be outdated, so this need to restart
			continue
		}

		storageAddressCount.Observe(float64(len(storage)))
		err = s.blockchain.PutStorage(storage)
		if err != nil {
			s.log.Errorw("store raw err", "err", err)
			return err
		}

		// TODO: assign to nil to clear memory
		nextjobs = make([]*storageRangeJob, 0)
		for i := processedJobs; i < len(jobs); i++ {
			unprocessedRequest := jobs[i]
			nextjobs = append(nextjobs, unprocessedRequest)
		}
	}
}

//nolint:gocyclo
func (s *SnapSyncher) runStorageRefreshWorker(ctx context.Context) error {
	// In ethereum, this is normally done with get tries, but since we don't have that here, we'll have to be
	// creative. This does mean that this is impressively inefficient.
	var job *storageRangeJob

	for {
		if job == nil {
		requestloop:
			for {
				contractDoneChecker := s.contractRangeDone
				if s.storageRangeJobCount > 0 {
					contractDoneChecker = nil // So that it never complete as there are job to be done
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(JobDuration):
					s.log.Infow("waiting for more refresh job", "count", s.storageRangeJobCount)
				case <-contractDoneChecker:
					// Its done...
					return nil
				case job = <-s.storageRefreshJob:
					break requestloop
				}
			}
		}

		bigIntAdd := job.startAddress.BigInt(&big.Int{})
		bigIntAdd = (&big.Int{}).Add(bigIntAdd, big.NewInt(1))
		elem := fp.NewElement(0)
		limitAddr := felt.NewFelt((&elem).SetBigInt(bigIntAdd))
		var outherErr error

		stateRoot := s.currentGlobalStateRoot
		ctrIter, err := s.client.RequestContractRange(ctx, &spec.ContractRangeRequest{
			Domain:         0, // What do this do?
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptAddress(job.startAddress),
			End:            core2p2p.AdaptAddress(limitAddr),
			ChunksPerProof: 10000,
		})
		if err != nil {
			return err
		}

		ctrIter(func(response *spec.ContractRangeResponse) bool {
			if response == nil {
				return false
			}

			crange := response.GetRange()
			if crange == nil || crange.State == nil || len(crange.State) == 0 {
				return false
			}
			if response.RangeProof == nil {
				return false
			}

			classRoot := p2p2core.AdaptHash(response.ClassesRoot)
			contractRoot := p2p2core.AdaptHash(response.ContractsRoot)
			outherErr = VerifyGlobalStateRoot(stateRoot, classRoot, contractRoot)
			if outherErr != nil {
				// Root verification failed
				// TODO: Ban peer
				return false
			}

			paths := make([]*felt.Felt, len(crange.State))
			values := make([]*felt.Felt, len(crange.State))

			for i, rangeValue := range crange.State {
				paths[i] = p2p2core.AdaptAddress(rangeValue.Address)
				values[i] = CalculateRangeValueHash(rangeValue)
			}

			proofs := P2pProofToTrieProofs(response.RangeProof)
			_, outherErr = VerifyTrie(contractRoot, paths, values, proofs, core.GlobalTrieHeight, crypto.Pedersen)
			if outherErr != nil {
				// The peer should get penalised in this case
				return false
			}

			job.storageRoot = p2p2core.AdaptHash(crange.State[0].Storage)
			newClass := p2p2core.AdaptHash(crange.State[0].Storage)
			if newClass != job.classHash {
				outherErr = s.queueClassJob(ctx, newClass)
				if outherErr != nil {
					return false
				}
			}

			outherErr = s.queueStorageRangeJobJob(ctx, job)
			if outherErr != nil {
				return false
			}

			job = nil

			return true
		})

		if outherErr != nil {
			s.log.Errorw("Error with contract range", "err", outherErr)
			// Well... need to figure out how to determine if its a temporary error or not.
			// For sure, the state root can be outdated, so this need to restart
			continue
		}
	}
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
			s.log.Infow("class queue stall on class")
		}
	}
	return nil
}

func (s *SnapSyncher) queueStorageRangeJob(ctx context.Context, path, storageRoot, classHash *felt.Felt, nonce uint64) error {
	return s.queueStorageRangeJobJob(ctx, &storageRangeJob{
		path:         path,
		storageRoot:  storageRoot,
		startAddress: &felt.Zero,
		classHash:    classHash,
		nonce:        nonce,
	})
}

func (s *SnapSyncher) queueStorageRangeJobJob(ctx context.Context, job *storageRangeJob) error {
	queued := false
	for !queued {
		select {
		case s.storageRangeJobQueue <- job:
			queued = true
			atomic.AddInt32(&s.storageRangeJobCount, 1)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(JobDuration):
			s.log.Infow("queue storage range stall")
		}
	}
	return nil
}

func (s *SnapSyncher) queueStorageRefreshJob(ctx context.Context, job *storageRangeJob) error {
	queued := false
	for !queued {
		select {
		case s.storageRefreshJob <- job:
			queued = true
			atomic.AddInt32(&s.storageRangeJobCount, 1)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			s.log.Infow("storage refresh queue stall")
		}
	}
	return nil
}

func (s *SnapSyncher) poolLatestBlock(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(JobDuration):
			break
		case <-s.storageRangeDone:
			return nil
		}

		newTarget, err := s.getNextStartingBlock(ctx)
		if err != nil {
			return errors.Join(err, errors.New("error getting current head"))
		}

		// TODO: Race issue
		if newTarget.Number-s.lastBlock.Number < uint64(maxPivotDistance) {
			s.log.Infow("Not updating pivot yet", "lastblock", s.lastBlock.Number,
				"newTarget", newTarget.Number, "diff", newTarget.Number-s.lastBlock.Number)
			continue
		}

		pivotUpdates.Inc()

		s.log.Infow("Switching snap pivot", "hash", newTarget.Hash, "number", newTarget.Number)
		s.lastBlock = newTarget.Header

		fmt.Printf("Current state root is %s", s.lastBlock.GlobalStateRoot)
		s.currentGlobalStateRoot = s.lastBlock.GlobalStateRoot
	}
}

func (s *SnapSyncher) ApplyStateUpdate(blockNumber uint64) error {
	return errors.New("unimplemented")
}

func P2pProofToTrieProofs(proof *spec.PatriciaRangeProof) []trie.ProofNode {
	// TODO: Move to adapter

	proofs := make([]trie.ProofNode, len(proof.Nodes))
	for i, node := range proof.Nodes {
		if node.GetBinary() != nil {
			binary := node.GetBinary()
			proofs[i] = trie.ProofNode{
				Binary: &trie.Binary{
					LeftHash:  p2p2core.AdaptFelt(binary.Left),
					RightHash: p2p2core.AdaptFelt(binary.Right),
				},
			}
		} else {
			edge := node.GetEdge()
			// TODO. What if edge is nil too?
			key := trie.NewKey(uint8(edge.Length), edge.Path.Elements)
			proofs[i] = trie.ProofNode{
				Edge: &trie.Edge{
					Child: p2p2core.AdaptFelt(edge.Value),
					Path:  &key,
				},
			}
		}
	}

	return proofs
}

func VerifyGlobalStateRoot(globalStateRoot, classRoot, storageRoot *felt.Felt) error {
	var stateVersion = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))

	if classRoot.IsZero() {
		if globalStateRoot.Equal(storageRoot) {
			return nil
		} else {
			return errors.New("invalid global state root")
		}
	}

	if !crypto.PoseidonArray(stateVersion, storageRoot, classRoot).Equal(globalStateRoot) {
		return errors.New("invalid global state root")
	}
	return nil
}

func VerifyTrie(
	expectedRoot *felt.Felt,
	paths, hashes []*felt.Felt,
	proofs []trie.ProofNode,
	height uint8,
	hash func(*felt.Felt, *felt.Felt) *felt.Felt,
) (bool, error) {
	hasMore, valid, err := trie.VerifyRange(expectedRoot, nil, paths, hashes, proofs, hash, height)
	if err != nil {
		return false, err
	}
	if !valid {
		return false, errors.New("invalid proof")
	}

	return hasMore, nil
}

func CalculateClassHash(cls core.Class) *felt.Felt {
	hash, err := cls.Hash()
	if err != nil {
		panic(err)
	}

	return hash
}

func CalculateCompiledClassHash(cls core.Class) *felt.Felt {
	return cls.(*core.Cairo1Class).Compiled.Hash()
}

func CalculateRangeValueHash(value *spec.ContractState) *felt.Felt {
	nonce := fp.NewElement(value.Nonce)
	return calculateContractCommitment(
		p2p2core.AdaptHash(value.Storage),
		p2p2core.AdaptHash(value.Class),
		felt.NewFelt(&nonce),
	)
}

func calculateContractCommitment(storageRoot, classHash, nonce *felt.Felt) *felt.Felt {
	return crypto.Pedersen(crypto.Pedersen(crypto.Pedersen(classHash, storageRoot), nonce), &felt.Zero)
}
