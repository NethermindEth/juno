package sync

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	big "math/big"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/pkg/errors"
)

type MutableStorage interface {
	SetClasss(path *felt.Felt, classHash *felt.Felt, class core.Class) error
	SetAddress(paths []*felt.Felt, nodeHashes []*felt.Felt, classHashes []*felt.Felt, nonces []*felt.Felt) error
	SetStorage(storagePath *felt.Felt, paths []*felt.Felt, values []*felt.Felt) error
	GetStateRoot() (*felt.Felt, error)
	ApplyStateUpdate(update *core.StateUpdate, validate bool) error
}

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

	processingLock *sync.Mutex

	addressRangeDone chan interface{}
	storageRangeDone chan interface{}
	phase1Done       chan interface{}

	storageRangeJobCount int32
	storageRangeJob      chan *blockchain.StorageRangeRequest
	storageRangeJobRetry chan *blockchain.StorageRangeRequest

	largeStorageRangeJobCount int32
	largeStorageRangeJob      chan *blockchain.StorageRangeRequest
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
		},
		blockchain: blockchain,
		log:        log,
	}
}

var (
	addressDurations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "juno_address_durations",
		Help: "Time in address get",
	}, []string{"phase"})
	storageDurations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "juno_storage_durations",
		Help: "Time in address get",
	}, []string{"phase"})
	largeStorageDurations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "juno_large_storage_durations",
		Help: "Time in address get",
	}, []string{"phase"})
)
var metricOnce *sync.Once = &sync.Once{}

func (s *SnapSyncher) Run(ctx context.Context) error {
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
	go func() {
		metricOnce.Do(func() {
			http.Handle("/metrics", promhttp.Handler())
			http.ListenAndServe(":2112", nil)
		})
	}()

	err := s.initState(ctx)
	if err != nil {
		return errors.Wrap(err, "error initializing snap syncer state")
	}

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.poolLatestBlock(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.runClassRangeWorker(ctx)
		if err != nil {
			s.log.Errorw("error in class range worker", "err", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.runAddressRangeWorker(ctx)
		if err != nil {
			s.log.Errorw("error in address range worker", "err", err)
		}
	}()

	storageWg := &sync.WaitGroup{}
	storageRangeWorkerCount := 4
	for i := 0; i < storageRangeWorkerCount; i++ {
		i := i
		storageWg.Add(1)
		go func() {
			defer storageWg.Done()
			err := s.runStorageRangeWorker(ctx, i)
			if err != nil {
				s.log.Errorw("error in storage range worker", "err", err)
			}
			s.log.Infow("Storage worker completed", "workerId", i)
		}()
	}

	// For notifying that storage range is done
	wg.Add(1)
	go func() {
		defer wg.Done()
		storageWg.Wait()

		fmt.Printf("Storage range range completed")
		close(s.storageRangeDone)
	}()

	largeStorageRangeWorkerCount := 4
	for i := 0; i < largeStorageRangeWorkerCount; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.runLargeStorageRangeWorker(ctx, i)
			if err != nil {
				s.log.Errorw("error in large storage range worker", "err", err)
			}
			s.log.Infow("Large storage worker completed", "workerId", i)
		}()
	}

	close(s.phase1Done)
	wg.Wait()
	s.log.Infow("first phase completed", "duration", time.Now().Sub(starttime).String())

	for i := s.startingBlock.Number + 1; i <= s.lastBlock.Number; i++ {
		stateUpdate, err := s.starknetData.StateUpdate(ctx, i)
		if err != nil {
			return errors.Wrap(err, "error fetching state update")
		}

		shouldValidate := i == s.lastBlock.Number

		err = s.ApplyStateUpdate(stateUpdate, shouldValidate)
		if err != nil {
			return errors.Wrap(err, "error applying state update")
		}
	}

	return s.baseSync.Run(ctx)
}

func (s *SnapSyncher) runClassRangeWorker(ctx context.Context) error {
	totaladded := 0
	maxint := big.NewInt(1)
	maxint.Lsh(maxint, 251)
	startAddr := &felt.Zero
	hasNext := true
	for hasNext {
		// TODO: Error need some way
		theint := startAddr.BigInt(big.NewInt(0))
		theint.Mul(theint, big.NewInt(100))
		theint.Div(theint, maxint)

		classRoot := s.currentClassRoot
		if classRoot == nil || classRoot.IsZero() {
			s.log.Infow("no class root", "progress", theint)
			return nil
		}

		s.log.Infow("class range progress", "progress", theint)

		response, err := s.snapServer.GetClassRange(classRoot, startAddr, nil)
		if err != nil {
			return errors.Wrap(err, "error get address range")
		}

		// TODO: Verify hashes
		hasNext, err = trie.VerifyTrie(classRoot, response.Paths, response.ClassHashes, response.Proofs, crypto.Poseidon)
		if err != nil {
			return errors.Wrap(err, "error verifying tree")
		}

		for i, path := range response.Paths {
			err := s.SetClasss(path, response.ClassHashes[i], response.Classes[i])
			if err != nil {
				return errors.Wrap(err, "error verifying tree")
			}
		}

		startAddr = response.Paths[len(response.Paths)-1]
	}

	fmt.Printf("Class range completed %d\n", totaladded)
	return nil
}

func (s *SnapSyncher) runAddressRangeWorker(ctx context.Context) error {
	defer func() {
		fmt.Printf("Address range completed\n")
		close(s.addressRangeDone)
	}()

	maxint := big.NewInt(1)
	maxint.Lsh(maxint, 251)
	startAddr := &felt.Zero
	hasNext := true
	for hasNext {
		// TODO: Error need some way

		theint := startAddr.BigInt(big.NewInt(0))
		theint.Mul(theint, big.NewInt(100))
		theint.Div(theint, maxint)

		curstateroot := s.currentStateRoot

		s.log.Infow("snap range progress", "progress", theint)
		nextHasNext, nextStartAddress, err := s.fetchAndProcessRange(ctx, curstateroot, startAddr)
		if errors2.Is(err, context.Canceled) {
			return err
		}
		if err != nil {
			s.log.Warnw("error fetching snap range", "error", err)
		} else {
			hasNext = nextHasNext
			startAddr = nextStartAddress
		}
	}

	fmt.Printf("Address range completed\n")

	return nil
}

func (s *SnapSyncher) fetchAndProcessRange(ctx context.Context, curstateroot *felt.Felt, startAddr *felt.Felt) (bool, *felt.Felt, error) {
	starttime := time.Now()
	response, err := s.snapServer.GetAddressRange(curstateroot, startAddr, nil)
	addressDurations.WithLabelValues("get").Add(float64(time.Now().Sub(starttime).Microseconds()))
	starttime = time.Now()
	if err != nil {
		return false, nil, errors.Wrap(err, "error get address range")
	}

	// TODO: Verify hashes
	hasNext, err := trie.VerifyTrie(curstateroot, response.Paths, response.Hashes, response.Proofs, crypto.Pedersen)
	addressDurations.WithLabelValues("verify").Add(float64(time.Now().Sub(starttime).Microseconds()))
	starttime = time.Now()
	if err != nil {
		return false, nil, errors.Wrap(err, "error verifying tree")
	}

	classHashes := make([]*felt.Felt, 0)
	nonces := make([]*felt.Felt, 0)

	for i := range response.Paths {
		classHashes = append(classHashes, response.Leaves[i].ClassHash)
		nonces = append(nonces, response.Leaves[i].Nonce)
	}

	// TODO: l0 class not in trie
	starttime = time.Now()
	err = s.SetAddress(response.Paths, response.Hashes, classHashes, nonces)
	addressDurations.WithLabelValues("set").Add(float64(time.Now().Sub(starttime).Microseconds()))
	starttime = time.Now()
	if err != nil {
		return false, nil, errors.Wrap(err, "error setting address")
	}

	for i, path := range response.Paths {
		if response.Leaves[i].ContractStorageRoot == nil {
			return false, nil, errors.New("storage root is nil")
		}

		starttime = time.Now()
		select {
		case s.storageRangeJob <- &blockchain.StorageRangeRequest{
			Path:      path,
			Hash:      response.Leaves[i].ContractStorageRoot,
			StartAddr: &felt.Zero,
		}:
			atomic.AddInt32(&s.storageRangeJobCount, 1)
		case <-ctx.Done():
			return false, nil, ctx.Err()
		}
		addressDurations.WithLabelValues("queueing").Add(float64(time.Now().Sub(starttime).Microseconds()))
	}

	return hasNext, response.Paths[len(response.Paths)-1], nil
}

func (s *SnapSyncher) runStorageRangeWorker(ctx context.Context, workerIdx int) error {
	totalprocessed := 0
	for {
		batchSize := 100000
		requests := make([]*blockchain.StorageRangeRequest, 0)

	requestloop:
		for len(requests) < batchSize {
			addressdonechecker := s.addressRangeDone
			if s.storageRangeJobCount > 0 {
				addressdonechecker = nil // So that it never complete
			}

			// Take from retry first, or there can be a deadlock
			// TODO: use a loop
			select {
			case job := <-s.storageRangeJobRetry:
				requests = append(requests, job)
				continue
			default:
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second * 5):
				if len(requests) > 0 {
					break requestloop
				}
				fmt.Printf("storage no job\n")
			case <-addressdonechecker:
				// Its done...
				return nil
			case job := <-s.storageRangeJob:
				requests = append(requests, job)
			}
		}

		fmt.Printf("storage jobs %d batch size %d queue size %d %d\n", workerIdx, len(requests), s.storageRangeJobCount, totalprocessed)

		curstateroot := s.currentStateRoot

		starttime := time.Now()
		responses, err := s.snapServer.GetContractRange(curstateroot, requests)
		storageDurations.WithLabelValues("get").Add(float64(time.Now().Sub(starttime).Microseconds()))
		if err != nil {
			fmt.Printf("Contract range failed\n")
			request := requests[0]
			fmt.Printf("Request %s %s\n", request.Hash.String(), request.Path.String())
			return err
		}

		for i := len(responses); i < len(requests); i++ {
			unprocessedRequest := requests[i]
			select {
			case s.storageRangeJobRetry <- unprocessedRequest:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		allDiffs := map[felt.Felt][]core.StorageDiff{}

		for i, response := range responses {
			request := requests[i]

			// TODO: it could be nil if its updated and therefore require a refresh
			if len(response.Paths) == 0 {
				if !request.Hash.Equal(&felt.Zero) {
					return fmt.Errorf("empty path got non zero hash")
				}
				// TODO: need to check if its really empty
				atomic.AddInt32(&s.storageRangeJobCount, -1)
				totalprocessed++
				continue
			}

			starttime := time.Now()
			hasNext, err := trie.VerifyTrie(request.Hash, response.Paths, response.Values, response.Proofs, crypto.Pedersen)
			storageDurations.WithLabelValues("verify").Add(float64(time.Now().Sub(starttime).Microseconds()))
			if err != nil {
				fmt.Printf("Verification failed\n")
				fmt.Printf("Request %s %s\n", request.Hash.String(), request.Path.String())
				for i, path := range response.Paths {
					fmt.Printf("S %s -> %s\n", path.String(), response.Values[i].String())
				}

				return err
			}

			diffs := make([]core.StorageDiff, 0)
			for i, path := range response.Paths {
				diffs = append(diffs, core.StorageDiff{
					Key:   path,
					Value: response.Values[i],
				})
			}

			allDiffs[*request.Path] = diffs
			starttime = time.Now()
			if hasNext {
				fmt.Printf("Adding large storage %s, %s, %s\n", request.Path, request.Hash, request.StartAddr)
				select {
				case s.largeStorageRangeJob <- request:
				case <-ctx.Done():
				}
				atomic.AddInt32(&s.largeStorageRangeJobCount, 1)
			}
			storageDurations.WithLabelValues("queueing").Add(float64(time.Now().Sub(starttime).Microseconds()))

			atomic.AddInt32(&s.storageRangeJobCount, -1)
			totalprocessed++
		}

		starttime = time.Now()
		err = s.SetStorage(allDiffs)
		storageDurations.WithLabelValues("set").Add(float64(time.Now().Sub(starttime).Microseconds()))
		if err != nil {
			return err
		}
	}
}

func (s *SnapSyncher) runLargeStorageRangeWorker(ctx context.Context, workerIdx int) error {
	maxint := big.NewInt(1)
	maxint.Lsh(maxint, 251)

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

		fmt.Printf("large storage jobs for %s with queue length %d\n", job.Path.String(), s.largeStorageRangeJobCount)

		curstateroot := s.currentStateRoot

		startAddr := job.StartAddr
		hasNext := true
		for hasNext {
			theint := startAddr.BigInt(big.NewInt(0))
			theint.Mul(theint, big.NewInt(100))
			theint.Div(theint, maxint)
			fmt.Printf("long range %d: %s %s %s%% %s %v\n", workerIdx, job.Path, job.Hash, theint.String(), startAddr.String(), hasNext)

			job.StartAddr = startAddr
			starttime := time.Now()
			responses, err := s.snapServer.GetContractRange(curstateroot, []*blockchain.StorageRangeRequest{job})
			largeStorageDurations.WithLabelValues("get").Add(float64(time.Now().Sub(starttime).Microseconds()))
			starttime = time.Now()
			if err != nil {
				fmt.Printf("Contract range failed\n")
				return err
			}

			response := responses[0] // TODO: it can return nothing

			// TODO: Verify hashes
			hasNext, err = trie.VerifyTrie(job.Hash, response.Paths, response.Values, response.Proofs, crypto.Pedersen)
			largeStorageDurations.WithLabelValues("verify").Add(float64(time.Now().Sub(starttime).Microseconds()))
			starttime = time.Now()
			if err != nil {
				return err
			}

			diffs := make([]core.StorageDiff, 0)
			for i, path := range response.Paths {
				diffs = append(diffs, core.StorageDiff{
					Key:   path,
					Value: response.Values[i],
				})
			}

			err = s.SetStorage(map[felt.Felt][]core.StorageDiff{
				*job.Path: diffs,
			})
			largeStorageDurations.WithLabelValues("set").Add(float64(time.Now().Sub(starttime).Microseconds()))
			starttime = time.Now()
			if err != nil {
				fmt.Printf("Contract range storage failed\n")
				return err
			}

			startAddr = response.Paths[len(response.Paths)-1]
		}

		atomic.AddInt32(&s.largeStorageRangeJobCount, -1)
	}
}

func (s *SnapSyncher) initState(ctx context.Context) error {
	head, err := s.starknetData.BlockLatest(ctx)
	if err != nil {
		return errors.Wrap(err, "error getting current head")
	}

	s.startingBlock = head.Header
	s.lastBlock = head.Header

	rootInfo, err := s.snapServer.GetTrieRootAt(ctx, s.startingBlock)
	if err != nil {
		return errors.Wrap(err, "error getting trie root")
	}
	s.currentStateRoot = rootInfo.StorageRoot
	s.currentClassRoot = rootInfo.ClassRoot

	s.storageRangeJobCount = 0
	s.storageRangeJob = make(chan *blockchain.StorageRangeRequest, 10000)
	s.storageRangeJobRetry = make(chan *blockchain.StorageRangeRequest, 10000000)
	s.largeStorageRangeJobCount = 0
	s.largeStorageRangeJob = make(chan *blockchain.StorageRangeRequest, 10000)

	s.addressRangeDone = make(chan interface{})
	s.storageRangeDone = make(chan interface{})
	s.phase1Done = make(chan interface{})

	return nil
}

func (s *SnapSyncher) poolLatestBlock(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		case <-s.phase1Done:
			return nil
		}

		head, err := s.starknetData.BlockLatest(ctx)
		if err != nil {
			return errors.Wrap(err, "error getting current head")
		}

		// TODO: Race issue
		if head.Number-s.startingBlock.Number < 64 {
			continue
		}

		s.log.Infow("Switching snap pivot", "hash", head.Hash, "number", head.Number)
		s.lastBlock = head.Header

		rootInfo, err := s.snapServer.GetTrieRootAt(ctx, s.startingBlock)
		if err != nil {
			return errors.Wrap(err, "error getting trie root")
		}
		s.currentStateRoot = rootInfo.StorageRoot
		s.currentClassRoot = rootInfo.ClassRoot
	}
}

func (s *SnapSyncher) ApplyStateUpdate(update *core.StateUpdate, validate bool) error {
	return nil
}

func (s *SnapSyncher) GetStateRoot() (*felt.Felt, error) {
	state, close, err := s.blockchain.HeadState()
	if err == db.ErrKeyNotFound {
		return &felt.Zero, nil
	}
	if err != nil {
		return nil, err
	}

	trie, closer2, err := state.(core.StateReaderStorage).StorageTrie()
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

func (s *SnapSyncher) SetClasss(path *felt.Felt, classHash *felt.Felt, class core.Class) error {
	// TODO: this
	return nil
}

func (s *SnapSyncher) SetAddress(paths []*felt.Felt, nodeHashes []*felt.Felt, classHashes []*felt.Felt, nonces []*felt.Felt) error {
	return s.blockchain.StoreDirect(paths, classHashes, nodeHashes, nonces)
}

func (s *SnapSyncher) SetStorage(diffs map[felt.Felt][]core.StorageDiff) error {
	return s.blockchain.StoreStorageDirect(diffs)
}

var _ service.Service = (*SnapSyncher)(nil)
