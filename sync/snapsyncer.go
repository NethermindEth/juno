package sync

import (
	"context"
	"fmt"
	big "math/big"
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

type Consensus interface {
	GetCurrentHead() (*core.Header, error)
	GetStateUpdateForBlock(blockNumber uint64) (*core.StateUpdate, error)
}

type MutableStorage interface {
	SetClasss(path *felt.Felt, classHash *felt.Felt, class core.Class) error
	SetAddress(paths []*felt.Felt, nodeHashes []*felt.Felt, classHashes []*felt.Felt, nonces []*felt.Felt) error
	SetStorage(storagePath *felt.Felt, paths []*felt.Felt, values []*felt.Felt) error
	GetStateRoot() (*felt.Felt, error)
	ApplyStateUpdate(update *core.StateUpdate, validate bool) error
}

type SnapSyncher struct {
	baseSync   *Synchronizer
	consensus  Consensus
	snapServer blockchain.SnapServer
	targetTrie MutableStorage
	log        utils.Logger

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
	s.log.Infow("first phase completed")

	for i := s.startingBlock.Number + 1; i <= s.lastBlock.Number; i++ {
		stateUpdate, err := s.consensus.GetStateUpdateForBlock(i)
		if err != nil {
			return errors.Wrap(err, "error fetching state update")
		}

		shouldValidate := i == s.lastBlock.Number

		err = s.targetTrie.ApplyStateUpdate(stateUpdate, shouldValidate)
		if err != nil {
			return errors.Wrap(err, "error applying state update")
		}
	}

	return nil
	// return s.baseSync.Run(ctx)
}

func (s *SnapSyncher) runAddressRangeWorker(ctx context.Context) error {
	defer func() {
		fmt.Printf("Address range completed\n")
		close(s.addressRangeDone)
	}()

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

		curstateroot := s.currentStateRoot

		s.log.Infow("snap range progress", "progress", theint)

		response, err := s.snapServer.GetAddressRange(curstateroot, startAddr, nil)
		if err != nil {
			return errors.Wrap(err, "error get address range")
		}

		// TODO: Verify hashes
		hasNext, err = trie.VerifyTrie(curstateroot, response.Paths, response.Hashes, response.Proofs, crypto.Pedersen)
		if err != nil {
			return errors.Wrap(err, "error verifying tree")
		}

		classHashes := make([]*felt.Felt, 0)
		nonces := make([]*felt.Felt, 0)

		for i := range response.Paths {
			classHashes = append(classHashes, response.Leaves[i].ClassHash)
			nonces = append(nonces, response.Leaves[i].Nonce)
		}

		// TODO: l0 class not in trie
		err = s.targetTrie.SetAddress(response.Paths, response.Hashes, classHashes, nonces)
		if err != nil {
			return errors.Wrap(err, "error setting address")
		}

		for i, path := range response.Paths {
			totaladded++
			select {
			case s.storageRangeJob <- &blockchain.StorageRangeRequest{
				Path:      path,
				Hash:      response.Leaves[i].StorageRoot,
				StartAddr: &felt.Zero,
			}:
				atomic.AddInt32(&s.storageRangeJobCount, 1)
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		startAddr = response.Paths[len(response.Paths)-1]
	}

	fmt.Printf("Address range completed %d\n", totaladded)

	return nil
}

func (s *SnapSyncher) runStorageRangeWorker(ctx context.Context, workerIdx int) error {
	totalprocessed := 0
	for {
		batchSize := 1000
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

		responses, err := s.snapServer.GetContractRange(curstateroot, requests)
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

			hasNext, err := trie.VerifyTrie(request.Hash, response.Paths, response.Values, response.Proofs, crypto.Pedersen)
			if err != nil {
				fmt.Printf("Verification failed\n")
				fmt.Printf("Request %s %s\n", request.Hash.String(), request.Path.String())
				for i, path := range response.Paths {
					fmt.Printf("S %s -> %s\n", path.String(), response.Values[i].String())
				}

				return err
			}

			if hasNext {
				requests[i].StartAddr = response.Paths[len(response.Paths)-1]
				s.largeStorageRangeJob <- requests[i]
				atomic.AddInt32(&s.largeStorageRangeJobCount, 1)
			}

			err = s.targetTrie.SetStorage(request.Path, response.Paths, response.Values)
			if err != nil {
				fmt.Printf("Set storage failed\n")
				fmt.Printf("Request %s %s\n", request.Hash.String(), request.Path.String())
				return err
			}

			atomic.AddInt32(&s.storageRangeJobCount, -1)
			totalprocessed++
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
			responses, err := s.snapServer.GetContractRange(curstateroot, []*blockchain.StorageRangeRequest{job})
			if err != nil {
				fmt.Printf("Contract range failed\n")
				return err
			}

			response := responses[0] // TODO: it can return nothing

			// TODO: Verify hashes
			hasNext, err = trie.VerifyTrie(job.Hash, response.Paths, response.Values, response.Proofs, crypto.Pedersen)
			if err != nil {
				return err
			}

			err = s.targetTrie.SetStorage(job.Path, response.Paths, response.Values)
			if err != nil {
				fmt.Printf("Contract range storage failed\n")
				return err
			}

			startAddr = response.Paths[len(response.Paths)-1]
		}

		atomic.AddInt32(&s.largeStorageRangeJobCount, -1)
	}
}

func (s *SnapSyncher) initState(context.Context) error {
	head, err := s.consensus.GetCurrentHead()
	if err != nil {
		return errors.Wrap(err, "error getting current head")
	}

	s.startingBlock = head
	s.lastBlock = head

	rootInfo, err := s.snapServer.GetTrieRootAt(s.startingBlock.Hash)
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

		head, err := s.consensus.GetCurrentHead()
		if err != nil {
			return errors.Wrap(err, "error getting current head")
		}

		// TODO: Race issue
		if head.Number-s.startingBlock.Number < 64 {
			continue
		}

		s.log.Infow("Switching snap pivot", "hash", head.Hash, "number", head.Number)
		s.lastBlock = head

		rootInfo, err := s.snapServer.GetTrieRootAt(s.startingBlock.Hash)
		if err != nil {
			return errors.Wrap(err, "error getting trie root")
		}
		s.currentStateRoot = rootInfo.StorageRoot
		s.currentClassRoot = rootInfo.ClassRoot
	}
}

var _ service.Service = (*SnapSyncher)(nil)
