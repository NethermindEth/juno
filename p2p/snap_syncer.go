package p2p

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/NethermindEth/juno/p2p/starknet"
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

type SnapSyncer struct {
	starknetData starknetdata.StarknetData
	client       *starknet.Client
	blockchain   Blockchain
	log          utils.SimpleLogger

	startingBlock          *core.Header
	lastBlock              *core.Header
	currentGlobalStateRoot *felt.Felt

	contractRangeDone chan interface{}
	storageRangeDone  chan interface{}

	storageRangeJobCount int32
	storageRangeJobQueue chan *storageRangeJob
	storageRefreshJob    chan *storageRangeJob

	classFetchJobCount int32
	classesJob         chan *felt.Felt

	// Three lock priority lock
	mtxM *sync.Mutex
	mtxN *sync.Mutex
	mtxL *sync.Mutex
}

var _ service.Service = (*SnapSyncer)(nil)

type storageRangeJob struct {
	path        *felt.Felt
	storageRoot *felt.Felt
	startKey    *felt.Felt
	classHash   *felt.Felt
	nonce       uint64
}

func NewSnapSyncer(
	client *starknet.Client,
	bc *blockchain.Blockchain,
	log utils.SimpleLogger,
) *SnapSyncer {
	return &SnapSyncer{
		client:     client,
		blockchain: bc,
		log:        log,
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
	storageJobWorkerCount = 4
	storageBatchSize      = 10
	storageJobQueueSize   = storageJobWorkerCount * storageBatchSize // Too high and the progress from address range would be inaccurate.

	// For some reason, the trie throughput is higher if the batch size is small.
	classRangeChunksPerProof   = 50
	contractRangeChunkPerProof = 150
	storageRangeChunkPerProof  = 300
	maxStorageBatchSize        = 1000
	maxMaxPerStorageSize       = 1000

	fetchClassWorkerCount = 3 // Fairly parallelizable. But this is brute force...
	classesJobQueueSize   = 64
	classBatchSize        = 30

	maxPivotDistance     = 32        // Set to 1 to test updated storage.
	newPivotHeadDistance = uint64(0) // This should be the reorg depth
)

func (s *SnapSyncer) Run(ctx context.Context) error {
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

	s.starknetData = &MockStarkData{}

	err := s.runPhase1(ctx)
	if err != nil {
		return err
	}
	s.log.Infow("phase 1 completed")

	if err = s.PhraseVerify(ctx); err != nil {
		return err
	}
	s.log.Infow("trie roots verification completed")

	s.log.Infow("delegating to standard synchronizer")

	return nil
	// TODO: start p2p syncer
	// s.baseSync.start(ctx)
}

//nolint:gocyclo,nolintlint
func (s *SnapSyncer) runPhase1(ctx context.Context) error {
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
	for i := 0; i < storageJobWorkerCount; i++ {
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
			err := s.runFetchClassWorker(ectx, i)
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

func (s *SnapSyncer) PhraseVerify(ctx context.Context) error {
	// 1. Get the actual class & contract trie roots
	st, closer, err := s.blockchain.(*blockchain.Blockchain).HeadStateFreakingState()
	defer func() { _ = closer() }()
	if err != nil {
		s.log.Errorw("error getting state for state root", "err", err)
		return err
	}
	contractRoot, classRoot, err := st.StateAndClassRoot()
	if err != nil {
		s.log.Errorw("error getting contract and class root", "err", err)
		return err
	}

	// 2. Verify the global state root
	err = VerifyGlobalStateRoot(s.currentGlobalStateRoot, classRoot, contractRoot)
	if err == nil {
		s.log.Infow("PhraseVerify",
			"global state root", s.currentGlobalStateRoot, "contract root", contractRoot, "class root", classRoot)
		// all good no need for additional verification
		return nil
	}

	if err != nil {
		s.log.Errorw("global state root verification failure", "err", err)
	}

	// 3. Get the correct tries roots from the client
	iter, err := s.client.RequestContractRange(ctx, &spec.ContractRangeRequest{
		StateRoot:      core2p2p.AdaptHash(s.currentGlobalStateRoot),
		Start:          core2p2p.AdaptAddress(&felt.Zero),
		ChunksPerProof: 1,
	})
	if err != nil {
		s.log.Errorw("error getting contract range from client", "err", err)
		return err
	}

	var classR, contractR *felt.Felt
	iter(func(response *spec.ContractRangeResponse) bool {
		if _, ok := response.GetResponses().(*spec.ContractRangeResponse_Range); ok {
			classR = p2p2core.AdaptHash(response.ClassesRoot)
			contractR = p2p2core.AdaptHash(response.ContractsRoot)
		} else {
			s.log.Errorw("unexpected response", "response", response)
		}

		return false
	})
	if classR == nil || contractR == nil {
		s.log.Errorw("cannot obtain the trie roots from client response")
		return errors.New("cannot obtain the trie roots")
	}

	// 4. Log which one is incorrect
	s.log.Infow("Contract trie root", "expected", contractR, "actual", contractRoot)
	s.log.Infow("Class trie root", "expected", classR, "actual", classRoot)

	return errors.New("trie roots verification failed")
}

func (s *SnapSyncer) getNextStartingBlock(ctx context.Context) (*core.Block, error) {
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

func (s *SnapSyncer) initState(ctx context.Context) error {
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

	s.log.Infow("init state completed", "startingBlock", s.startingBlock.Number)
	return nil
}

func CalculatePercentage(f *felt.Felt) uint64 {
	const maxPercent = 100
	maxint := big.NewInt(1)
	maxint.Lsh(maxint, core.GlobalTrieHeight)

	percent := f.BigInt(big.NewInt(0))
	percent.Mul(percent, big.NewInt(maxPercent))
	percent.Div(percent, maxint)

	return percent.Uint64()
}

//nolint:gocyclo,nolintlint
func (s *SnapSyncer) runClassRangeWorker(ctx context.Context) error {
	totalAdded := 0
	completed := false
	startAddr := &felt.Zero

	s.log.Infow("class range worker entering infinite loop")
	for !completed {
		stateRoot := s.currentGlobalStateRoot

		// TODO: Maybe timeout
		classIter, err := s.client.RequestClassRange(ctx, &spec.ClassRangeRequest{
			Root:           core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptHash(startAddr),
			ChunksPerProof: uint32(classRangeChunksPerProof),
		})
		if err != nil {
			s.log.Errorw("error getting class range from client", "err", err)
			// retry policy? with increased intervals
			// reason for stopping - it's irrecoverable - we don't have a peer
			return err
		}

	ResponseIter:
		for response := range classIter {
			if response == nil {
				if response == nil {
					s.log.Errorw("contract range respond with nil response")
					continue
				}
			}

			var classes []*spec.Class
			switch v := response.GetResponses().(type) {
			case *spec.ClassRangeResponse_Classes:
				classes = v.Classes.Classes
			case *spec.ClassRangeResponse_Fin:
				break ResponseIter
			default:
				s.log.Warnw("Unexpected class range message", "GetResponses", v)
				continue
			}

			if classes == nil || len(classes) == 0 {
				s.log.Errorw("class range respond with empty classes")
				continue
			}
			if response.RangeProof == nil {
				s.log.Errorw("class range respond with nil proof")
				continue
			}

			classRoot := p2p2core.AdaptHash(response.ClassesRoot)
			contractRoot := p2p2core.AdaptHash(response.ContractsRoot)
			err = VerifyGlobalStateRoot(stateRoot, classRoot, contractRoot)
			if err != nil {
				// Root verification failed
				// TODO: Ban peer
				s.log.Errorw("global state root verification failure", "err", err)
				return err
			}

			s.log.Infow("class range progress", "progress", CalculatePercentage(startAddr))
			s.log.Infow("class range info", "classes", len(classes), "totalAdded", totalAdded, "startAddr", startAddr)

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
				s.log.Errorw("class range adaptation failure", "err", err)
				return err
			}

			proofs := P2pProofToTrieProofs(response.RangeProof)
			hasNext, err := VerifyTrie(classRoot, paths, values, proofs, core.GlobalTrieHeight, crypto.Poseidon)
			if err != nil {
				// TODO: Ban peer
				s.log.Errorw("trie verification failed", "err", err)
				return err
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
				fmt.Printf("Unable to update the chain %s\n", err)
				panic(err)
			}
			totalAdded += len(classes)

			if !hasNext {
				s.log.Infow("class range completed", "totalClass", totalAdded)
				completed = true
				return nil
			}

			// Increment addr, start loop again
			startAddr = paths[len(paths)-1]
		}
	}

	s.log.Infow("class range worker exits infinite loop")
	return nil
}

//nolint:gocyclo
func (s *SnapSyncer) runFetchClassWorker(ctx context.Context, workerIdx int) error {
	keyBatches := make([]*felt.Felt, 0)
	s.log.Infow("class fetch worker entering infinite loop", "worker", workerIdx)
	for {
	requestloop:
		for len(keyBatches) < classBatchSize {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(JobDuration):
				// Just request whatever we have
				if len(keyBatches) > 0 {
					break requestloop
				}
				s.log.Infow("waiting for more class job", "worker", workerIdx, "pendind", s.classFetchJobCount)
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

					// TODO: Can be done in batches, Note: return nil if class is not found, no error
					cls, err := s.blockchain.GetClasses([]*felt.Felt{key})
					if err != nil {
						s.log.Errorw("error getting class", "err", err)
						return err
					}

					if cls[0] == nil {
						keyBatches = append(keyBatches, key)
					}
				}
				atomic.AddInt32(&s.classFetchJobCount, -1)
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
			s.log.Errorw("error fetching classes by hash from client", "err", err)
			// retry?
			return err
		}

		classes := make([]*spec.Class, 0, len(keyBatches))
	ResponseIter:
		for response := range classIter {
			if response == nil {
				s.log.Errorw("class by keys respond with nil response")
				continue
			}

			switch v := response.ClassMessage.(type) {
			case *spec.ClassesResponse_Class:
				classes = append(classes, v.Class)
			case *spec.ClassesResponse_Fin:
				break ResponseIter
			default:
				s.log.Warnw("Unexpected ClassMessage from getClasses", "v", v)
			}
		}

		processedClasses := map[felt.Felt]bool{}
		newClasses := map[felt.Felt]core.Class{}
		classHashes := map[felt.Felt]*felt.Felt{}
		for i, class := range classes {
			if class == nil {
				s.log.Infow("class empty", "hash", keyBatches[i])
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
func (s *SnapSyncer) runContractRangeWorker(ctx context.Context) error {
	totalAdded := 0
	startAddr := &felt.Zero
	completed := false

	s.log.Infow("contract range worker entering infinite loop")
	for !completed {
		stateRoot := s.currentGlobalStateRoot
		iter, err := s.client.RequestContractRange(ctx, &spec.ContractRangeRequest{
			Domain:         0, // What do this do?
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptAddress(startAddr),
			End:            nil, // No need for now.
			ChunksPerProof: uint32(contractRangeChunkPerProof),
		})
		if err != nil {
			s.log.Errorw("error getting contract range from client", "err", err)
			// retry?
			return err
		}

	ResponseIter:
		for response := range iter {
			if response == nil {
				s.log.Errorw("contract range respond with nil response")
				continue
			}

			var crange *spec.ContractRange
			switch v := response.GetResponses().(type) {
			case *spec.ContractRangeResponse_Range:
				crange = v.Range
			case *spec.ContractRangeResponse_Fin:
				break ResponseIter
			default:
				s.log.Warnw("Unexpected contract range message", "GetResponses", v)
				continue
			}

			if crange == nil || crange.State == nil {
				s.log.Errorw("contract range respond with nil state")
				continue
			}
			if response.RangeProof == nil {
				s.log.Errorw("contract range respond with nil proof")
				continue
			}

			s.log.Infow("contract range progress", "progress", CalculatePercentage(startAddr))
			s.log.Infow("contract range info", "states", len(crange.State), "totalAdded", totalAdded, "startAddr", startAddr)
			rangeProgress.Set(float64(CalculatePercentage(startAddr)))

			classRoot := p2p2core.AdaptHash(response.ClassesRoot)
			contractRoot := p2p2core.AdaptHash(response.ContractsRoot)
			err := VerifyGlobalStateRoot(stateRoot, classRoot, contractRoot)
			if err != nil {
				// TODO: Ban peer
				s.log.Errorw("global state root verification failure", "err", err)
				return err
			}

			paths := make([]*felt.Felt, len(crange.State))
			values := make([]*felt.Felt, len(crange.State))

			for i, state := range crange.State {
				paths[i] = p2p2core.AdaptAddress(state.Address)
				values[i] = CalculateContractStateHash(state)
			}

			proofs := P2pProofToTrieProofs(response.RangeProof)
			hasNext, err := VerifyTrie(contractRoot, paths, values, proofs, core.GlobalTrieHeight, crypto.Pedersen)
			if err != nil {
				// The peer should get penalised in this case
				s.log.Errorw("trie verification failed", "err", err)
				return err
			}

			classes := []*felt.Felt{}
			nonces := []*felt.Felt{}
			for _, r := range crange.State {
				classHash := p2p2core.AdaptHash(r.Class)
				classes = append(classes, classHash)
				nonces = append(nonces, (&felt.Felt{}).SetUint64(r.Nonce))
			}

			err = s.blockchain.PutContracts(paths, nonces, classes)
			if err != nil {
				fmt.Printf("Unable to update the chain %s\n", err)
				panic(err)
			}
			totalAdded += len(paths)

			// We don't actually store it directly here... only put it as part of job.
			// Can't remember why. Could be because it would be some wasted work.
			for _, r := range crange.State {
				path := p2p2core.AdaptAddress(r.Address)
				storageRoot := p2p2core.AdaptHash(r.Storage)
				classHash := p2p2core.AdaptHash(r.Class)
				nonce := r.Nonce

				err = s.queueClassJob(ctx, classHash)
				if err != nil {
					s.log.Errorw("error queue class fetch job", "err", err)
					return err
				}

				err = s.queueStorageRangeJob(ctx, path, storageRoot, classHash, nonce)
				if err != nil {
					s.log.Errorw("error queue storage refresh job", "err", err)
					return err
				}
			}

			if !hasNext {
				s.log.Infow("[hasNext] contract range completed")
				completed = true
				return nil
			}

			if len(paths) == 0 {
				return nil
			}

			startAddr = paths[len(paths)-1]
		}
	}
	s.log.Infow("contract range worker exits infinite loop")

	return nil
}

//nolint:funlen,gocyclo
func (s *SnapSyncer) runStorageRangeWorker(ctx context.Context, workerIdx int) error {
	nextjobs := make([]*storageRangeJob, 0)
	s.log.Infow("storage range worker entering infinite loop", "worker", workerIdx)
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
				s.log.Infow("waiting for more storage job", "len(jobs)", len(jobs), "worker", workerIdx, "pending", s.storageRangeJobCount)
			case <-contractDoneChecker:
				// Its done...
				return nil
			}
		}

		// s.log.Infow("storage range job completes batch", "jobs", len(jobs), "worker", workerIdx, "pending", s.storageRangeJobCount)

		requests := make([]*spec.StorageRangeQuery, 0)
		for _, job := range jobs {
			requests = append(requests, &spec.StorageRangeQuery{
				Address: core2p2p.AdaptAddress(job.path),
				Start: &spec.StorageLeafQuery{
					ContractStorageRoot: core2p2p.AdaptHash(job.storageRoot),

					// TODO: Should be address
					Key: core2p2p.AdaptFelt(job.startKey),
				},
			})
		}

		stateRoot := s.currentGlobalStateRoot
		processedJobs := struct {
			jobIdx  int
			jobAddr *felt.Felt
			address *felt.Felt
		}{}
		storage := map[felt.Felt]map[felt.Felt]*felt.Felt{}
		totalPath := 0
		maxPerStorageSize := 0

		//s.log.Infow("storage range",
		//	"rootDistance", s.lastBlock.Number-s.startingBlock.Number,
		//	"root", stateRoot.String(),
		//	"requestcount", len(requests),
		//	"worker", workerIdx,
		//)
		iter, err := s.client.RequestStorageRange(ctx, &spec.ContractStorageRequest{
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			ChunksPerProof: uint32(storageRangeChunkPerProof),
			Query:          requests,
		})
		if err != nil {
			s.log.Errorw("Error with storage range request", "err", err)
			// Well... need to figure out how to determine if its a temporary error or not.
			// For sure, the state root can be outdated, so this need to restart
			return err
		}

	ResponseIter:
		for response := range iter {
			if response == nil {
				s.log.Errorw("storage range respond with nil response",
					"worker", workerIdx, "job", processedJobs.jobIdx)
				continue
			}

			var csto *spec.ContractStorage
			switch v := response.GetResponses().(type) {
			case *spec.ContractStorageResponse_Storage:
				csto = v.Storage
			case *spec.ContractStorageResponse_Fin:
				break ResponseIter
			default:
				s.log.Warnw("Unexpected storage range message", "GetResponses", v)
				continue
			}

			if csto == nil || csto.KeyValue == nil {
				s.log.Errorw("storage range respond with nil storage",
					"worker", workerIdx, "job", processedJobs.jobIdx,
					"address", p2p2core.AdaptAddress(response.ContractAddress),
					"job addr", processedJobs.address)
				continue
			}
			if response.RangeProof == nil {
				s.log.Errorw("storage range respond with nil proof",
					"worker", workerIdx, "job", processedJobs.jobIdx,
					"address", p2p2core.AdaptAddress(response.ContractAddress),
					"job addr", processedJobs.address)
				continue
			}

			storageAddr := p2p2core.AdaptAddress(response.ContractAddress)
			storageRange := csto.KeyValue
			processedJobs.address = storageAddr

			job := jobs[processedJobs.jobIdx]
			processedJobs.jobAddr = job.path
			if !job.path.Equal(storageAddr) {
				s.log.Infow("[Missed response?] storage chunks completed",
					"address", storageAddr, "jobAddr", processedJobs.jobAddr, "worker", workerIdx, "job", processedJobs.jobIdx)
				// move to the next job
				processedJobs.jobIdx++
				job = jobs[processedJobs.jobIdx]

				// sanity check
				if !job.path.Equal(storageAddr) {
					s.log.Errorw("Sanity check: next job does not match response",
						"job addr", job.path, "got", storageAddr, "worker", workerIdx)
					// what to do?
				}
				processedJobs.address = storageAddr
				processedJobs.jobAddr = job.path
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
				s.log.Errorw("trie verification failed",
					"contract", storageAddr, "expected root", job.storageRoot, "err", err)
				// It is unclear how to distinguish if the peer is malicious/broken/non-bizantine or the contracts root is outdated.
				err = s.queueStorageRefreshJob(ctx, job)
				if err != nil {
					s.log.Errorw("error queue storage refresh job", "err", err)
					return err
				}

				// Ok, what now - we cannot just move on to next job, because server will
				// still respond to this contract until it exhaust all leaves
				continue
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
					return err
				}

				totalPath = 0
				maxPerStorageSize = 0
				storage = map[felt.Felt]map[felt.Felt]*felt.Felt{}
			}

			if hasNext {
				// note this is just tracking leaves keys on our side
				// but it cannot change the request. Maybe can be used for retry procedure
				job.startKey = paths[len(paths)-1]
			} else {
				processedJobs.jobIdx++
				atomic.AddInt32(&s.storageRangeJobCount, -1) // its... done?
			}
		}

		storageAddressCount.Observe(float64(len(storage)))
		err = s.blockchain.PutStorage(storage)
		if err != nil {
			s.log.Errorw("store raw err", "err", err)
			return err
		}

		// TODO: assign to nil to clear memory
		nextjobs = make([]*storageRangeJob, 0)
		for i := processedJobs.jobIdx; i < len(jobs); i++ {
			unprocessedRequest := jobs[i]
			nextjobs = append(nextjobs, unprocessedRequest)
		}
		processedJobs.jobIdx = 0
		processedJobs.address = nil
		processedJobs.jobAddr = nil
	}
}

//nolint:gocyclo
func (s *SnapSyncer) runStorageRefreshWorker(ctx context.Context) error {
	// In ethereum, this is normally done with get tries, but since we don't have that here, we'll have to be
	// creative. This does mean that this is impressively inefficient.
	var job *storageRangeJob

	s.log.Infow("storage refresh worker entering infinite loop")
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
					s.log.Infow("no storage refresh job")
				case <-contractDoneChecker:
					// Its done...
					return nil
				case job = <-s.storageRefreshJob:
					break requestloop
				}
			}
		}

		stateRoot := s.currentGlobalStateRoot
		ctrIter, err := s.client.RequestContractRange(ctx, &spec.ContractRangeRequest{
			Domain:         0, // What do this do?
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptAddress(job.path),
			End:            core2p2p.AdaptAddress(job.path),
			ChunksPerProof: 10,
		})
		if err != nil {
			s.log.Errorw("Error with contract range refresh request", "err", err)
			return err
		}

	ResponseIter:
		for response := range ctrIter {
			if response == nil {
				s.log.Errorw("contract range [storage refresh] respond with nil response")
				continue
			}

			var crange *spec.ContractRange
			switch v := response.GetResponses().(type) {
			case *spec.ContractRangeResponse_Range:
				crange = v.Range
			case *spec.ContractRangeResponse_Fin:
				break ResponseIter
			default:
				s.log.Warnw("Unexpected contract range message [storage refresh]", "GetResponses", v)
				continue
			}

			if crange == nil || crange.State == nil {
				s.log.Errorw("contract range [storage refresh] respond with nil state")
				continue
			}
			if response.RangeProof == nil {
				s.log.Errorw("contract range [storage refresh] respond with nil proof")
				continue
			}

			classRoot := p2p2core.AdaptHash(response.ClassesRoot)
			contractRoot := p2p2core.AdaptHash(response.ContractsRoot)
			err := VerifyGlobalStateRoot(stateRoot, classRoot, contractRoot)
			if err != nil {
				// Root verification failed
				// TODO: Ban peer
				return err
			}

			paths := make([]*felt.Felt, len(crange.State))
			values := make([]*felt.Felt, len(crange.State))

			for i, rangeValue := range crange.State {
				paths[i] = p2p2core.AdaptAddress(rangeValue.Address)
				values[i] = CalculateContractStateHash(rangeValue)
			}

			proofs := P2pProofToTrieProofs(response.RangeProof)
			_, err = VerifyTrie(contractRoot, paths, values, proofs, core.GlobalTrieHeight, crypto.Pedersen)
			if err != nil {
				// The peer should get penalised in this case
				return err
			}

			job.storageRoot = p2p2core.AdaptHash(crange.State[0].Storage)
			newClass := p2p2core.AdaptHash(crange.State[0].Storage)
			if newClass != job.classHash {
				err = s.queueClassJob(ctx, newClass)
				if err != nil {
					s.log.Errorw("error queue class fetch job", "err", err)
					return err
				}
			}

			err = s.queueStorageRangeJobJob(ctx, job)
			if err != nil {
				s.log.Errorw("error queue storage refresh job", "err", err)
				return err
			}

			job = nil
		}
	}
	s.log.Infow("storage refresh worker exits infinite loop")
	return nil
}

func (s *SnapSyncer) queueClassJob(ctx context.Context, classHash *felt.Felt) error {
	queued := false
	for !queued {
		select {
		case s.classesJob <- classHash:
			atomic.AddInt32(&s.classFetchJobCount, 1)
			queued = true
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
			s.log.Infow("class queue stall on class")
		}
	}
	return nil
}

func (s *SnapSyncer) queueStorageRangeJob(ctx context.Context, path, storageRoot, classHash *felt.Felt, nonce uint64) error {
	return s.queueStorageRangeJobJob(ctx, &storageRangeJob{
		path:        path,
		storageRoot: storageRoot,
		startKey:    &felt.Zero,
		classHash:   classHash,
		nonce:       nonce,
	})
}

func (s *SnapSyncer) queueStorageRangeJobJob(ctx context.Context, job *storageRangeJob) error {
	if job.storageRoot == nil || job.storageRoot.IsZero() {
		// contract's with storage root of 0x0 has no storage
		return nil
	}

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

func (s *SnapSyncer) queueStorageRefreshJob(ctx context.Context, job *storageRangeJob) error {
	queued := false
	for !queued {
		select {
		case s.storageRefreshJob <- job:
			queued = true
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			s.log.Infow("storage refresh queue stall")
		}
	}
	return nil
}

func (s *SnapSyncer) poolLatestBlock(ctx context.Context) error {
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

func (s *SnapSyncer) ApplyStateUpdate(blockNumber uint64) error {
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
	stateVersion := new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))

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

func CalculateContractStateHash(value *spec.ContractState) *felt.Felt {
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
