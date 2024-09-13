package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	stdsync "sync"
	"time"

	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
)

var (
	_ service.Service = (*Builder)(nil)
	_ sync.Reader     = (*Builder)(nil)
)

type Builder struct {
	ownAddress felt.Felt
	privKey    *ecdsa.PrivateKey

	bc        *blockchain.Blockchain
	vm        vm.VM
	newHeads  *feed.Feed[*core.Header]
	log       utils.Logger
	blockTime time.Duration
	pool      *mempool.Pool
	listener  EventListener

	pendingLock  stdsync.Mutex
	pendingBlock blockchain.Pending
	headState    core.StateReader
	headCloser   blockchain.StateCloser

	shadowMode          bool
	shadowStateUpdate   *core.StateUpdate
	shadowBlock         *core.Block
	shadowSyncToBlock   uint
	starknetData        starknetdata.StarknetData
	junoEndpoint        string
	blockTraces         []rpc.TracedBlockTransaction
	chanNumTxnsToShadow chan int
	chanFinaliseShadow  chan struct{}

	chanFinalise  chan struct{}
	chanFinalised chan struct{}

	blockHashToBeRevealed *felt.Felt
}

func New(privKey *ecdsa.PrivateKey, ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM,
	blockTime time.Duration, pool *mempool.Pool, log utils.Logger,
) *Builder {
	return &Builder{
		ownAddress:    *ownAddr,
		privKey:       privKey,
		blockTime:     blockTime,
		log:           log,
		listener:      &SelectiveListener{},
		chanFinalise:  make(chan struct{}),
		chanFinalised: make(chan struct{}, 1),

		bc:       bc,
		pool:     pool,
		vm:       builderVM,
		newHeads: feed.New[*core.Header](),
	}
}

func NewShadow(privKey *ecdsa.PrivateKey, ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM,
	blockTime time.Duration, pool *mempool.Pool, log utils.Logger, starknetData starknetdata.StarknetData,
) *Builder {
	return &Builder{
		ownAddress:    *ownAddr,
		privKey:       privKey,
		blockTime:     blockTime,
		log:           log,
		listener:      &SelectiveListener{},
		chanFinalise:  make(chan struct{}, 1),
		chanFinalised: make(chan struct{}, 1),

		bc:       bc,
		pool:     pool,
		vm:       builderVM,
		newHeads: feed.New[*core.Header](),

		shadowMode:          true,
		starknetData:        starknetData,
		chanNumTxnsToShadow: make(chan int, 1),
		chanFinaliseShadow:  make(chan struct{}, 1),
	}
}

func (b *Builder) WithEventListener(l EventListener) *Builder {
	b.listener = l
	return b
}

func (b *Builder) WithJunoEndpoint(endpoint string) *Builder {
	b.junoEndpoint = endpoint
	return b
}
func (b *Builder) WithSyncToBlock(syncTo uint) *Builder {
	b.shadowSyncToBlock = syncTo
	return b
}

func (b *Builder) Run(ctx context.Context) error {
	signFunc := b.Sign
	if b.shadowMode {
		b.log.Debugw("b.shadowMode")
		signFunc = nil
		syncToBlockNum := uint64(b.shadowSyncToBlock) // Todo: skipped problematic transaction in block 129751,133371, 134062,134073
		block, err := b.bc.Head()
		if err != nil {
			return err
		}
		// fmt.Println("sequencer head block", block.Number, block.ParentHash.String(), block.GlobalStateRoot.String())
		b.log.Debugw("attempting to sycn-store from block %i to %i", block.Number, syncToBlockNum)
		if block.Number < syncToBlockNum {
			if err := b.syncStore(block.Number, syncToBlockNum); err != nil {
				return err
			}
		} else {
			if err := b.bc.CleanPendingState(); err != nil {
				return err
			}
		}

	}

	if err := b.InitPendingBlock(); err != nil {
		return err
	}

	defer func() {
		if pErr := b.ClearPending(); pErr != nil {
			b.log.Errorw("clearing pending", "err", pErr)
		}
	}()

	doneListen := make(chan struct{})
	go func() {
		if pErr := b.listenPool(ctx); pErr != nil {
			b.log.Errorw("listening pool", "err", pErr)
		}
		close(doneListen)
	}()
	if b.shadowMode {
		go func() {
			if pErr := b.shadowTxns(ctx); pErr != nil {
				b.log.Errorw("shadowTxns", "err", pErr)
			}
		}()
	}

	go func() {
		if b.shadowMode {
			for {
				select {
				case <-b.chanFinaliseShadow:
					b.chanFinalise <- struct{}{}
				case <-ctx.Done():
					return
				}
			}
		}
		for {
			select {
			case <-time.After(b.blockTime):
				b.chanFinalise <- struct{}{}
			case <-ctx.Done():
				return
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			<-doneListen
			return nil
		case <-b.chanFinalise:
			err := b.cleanStorageDiff(b.pendingBlock.StateUpdate.StateDiff)
			if err != nil {
				return err
			}
			b.log.Infof("Finalising new block")
			err = b.Finalise(signFunc)
			if err != nil {
				return err
			}
			<-b.chanFinalised
		}
	}
}

func (b *Builder) cleanStorageDiff(sd *core.StateDiff) error {
	b.log.Debugw("Removing values in the storage diff that don't affect state")
	state, closer, err := b.bc.HeadState()
	if err != nil {
		return err
	}
	defer closer()
	for addr, storage := range sd.StorageDiffs {
		for k, v := range storage {
			previousValue, err := state.ContractStorage(&addr, &k)
			if err != nil {
				return err
			}
			if previousValue.Equal(v) {
				b.log.Infof("the key %v at the storage of address %v is being deleted", k.String(), addr.String())
				delete(sd.StorageDiffs[addr], k)
			}
		}
	}
	for addr := range sd.StorageDiffs {
		if len(sd.StorageDiffs[addr]) == 0 {
			delete(sd.StorageDiffs, addr)
		}
	}

	// If accounts and deployed, and upgraded in the same block, then move
	// replaced_classes to deployed_contracts
	for addr, classHash := range sd.ReplacedClasses {
		_, err := state.ContractClassHash(&addr)
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				b.log.Debugw("moving replaced class to deployed contracts")
				sd.DeployedContracts[addr] = classHash
				delete(sd.ReplacedClasses, addr)
			}
			b.log.Errorw("class is being replaced, but was not found in previous state")
		}
	}

	// Todo: solve duplicate-problem at source
	encountered := map[string]bool{}
	result := []*felt.Felt{}
	for i := range sd.DeclaredV0Classes {
		if !encountered[sd.DeclaredV0Classes[i].String()] {
			result = append(result, sd.DeclaredV0Classes[i])
			encountered[sd.DeclaredV0Classes[i].String()] = true
		}
	}
	sd.DeclaredV0Classes = result
	return nil
}

func (b *Builder) InitPendingBlock() error {
	if b.pendingBlock.Block != nil {
		return nil
	}

	bcPending, err := b.bc.Pending()
	if err != nil {
		return err
	}

	b.pendingBlock, err = utils.Clone[blockchain.Pending](bcPending)
	if err != nil {
		return err
	}
	b.pendingBlock.Block.SequencerAddress = &b.ownAddress

	b.headState, b.headCloser, err = b.bc.HeadState()
	return err
}

func (b *Builder) ClearPending() error {
	b.pendingBlock = blockchain.Pending{}
	if b.headState != nil {
		if err := b.headCloser(); err != nil {
			return err
		}
		b.headState = nil
		b.headCloser = nil
	}
	return nil
}

// Finalise the pending block and initialise the next one
func (b *Builder) Finalise(signFunc blockchain.BlockSignFunc) error {
	b.pendingLock.Lock()
	defer b.pendingLock.Unlock()

	if err := b.bc.Finalise(&b.pendingBlock, signFunc, b.shadowStateUpdate, b.shadowBlock); err != nil {
		return err
	}
	b.log.Infow("Finalised block", "number", b.pendingBlock.Block.Number, "hash",
		b.pendingBlock.Block.Hash.ShortString(), "state", b.pendingBlock.Block.GlobalStateRoot.ShortString())
	b.listener.OnBlockFinalised(b.pendingBlock.Block.Header)

	if err := b.ClearPending(); err != nil {
		return err
	}
	return b.InitPendingBlock()
}

// ValidateAgainstPendingState validates a user transaction against the pending state
// only hard-failures result in an error, reverts are not reported back to caller
func (b *Builder) ValidateAgainstPendingState(userTxn *mempool.BroadcastedTransaction) error {
	declaredClasses := []core.Class{}
	if userTxn.DeclaredClass != nil {
		declaredClasses = []core.Class{userTxn.DeclaredClass}
	}

	pendingBlock, err := b.bc.Pending()
	if err != nil {
		return err
	}

	state, stateCloser, err := b.bc.PendingState()
	if err != nil {
		return err
	}

	defer func() {
		if err = stateCloser(); err != nil {
			b.log.Errorw("closing state in ValidateAgainstPendingState", "err", err)
		}
	}()

	blockInfo := &vm.BlockInfo{
		Header: &core.Header{
			Number:           pendingBlock.Block.Number,
			Timestamp:        pendingBlock.Block.Timestamp,
			SequencerAddress: &b.ownAddress,
			GasPrice:         pendingBlock.Block.GasPrice,
			GasPriceSTRK:     pendingBlock.Block.GasPriceSTRK,
		},
	}
	_, _, _, _, _, err = b.vm.Execute([]core.Transaction{userTxn.Transaction}, declaredClasses, []*felt.Felt{}, blockInfo, state, //nolint:dogsled
		b.bc.Network(), false, false, false, false)
	return err
}

func (b *Builder) StartingBlockNumber() (uint64, error) {
	return 0, nil
}

func (b *Builder) HighestBlockHeader() *core.Header {
	return nil
}

func (b *Builder) SubscribeNewHeads() sync.HeaderSubscription {
	return sync.HeaderSubscription{
		Subscription: b.newHeads.Subscribe(),
	}
}

// Sign returns the builder's signature over data.
func (b *Builder) Sign(blockHash, stateDiffCommitment *felt.Felt) ([]*felt.Felt, error) {
	data := crypto.PoseidonArray(blockHash, stateDiffCommitment).Bytes()
	signatureBytes, err := b.privKey.Sign(data[:], nil)
	if err != nil {
		return nil, err
	}
	sig := make([]*felt.Felt, 0)
	for start := 0; start < len(signatureBytes); {
		step := len(signatureBytes[start:])
		if step > felt.Bytes {
			step = felt.Bytes
		}
		sig = append(sig, new(felt.Felt).SetBytes(signatureBytes[start:step]))
		start += step
	}
	return sig, nil
}

func Receipt(fee *felt.Felt, feeUnit core.FeeUnit, txHash *felt.Felt, trace *vm.TransactionTrace, txnReceipt *vm.TransactionReceipt) *core.TransactionReceipt {
	return &core.TransactionReceipt{
		Fee:                fee,
		FeeUnit:            feeUnit,
		Events:             vm2core.AdaptOrderedEvents(trace.AllEvents()),
		ExecutionResources: vm2core.AdaptExecutionResources(trace.TotalExecutionResources(), &txnReceipt.Gas),
		L2ToL1Message:      vm2core.AdaptOrderedMessagesToL1(trace.AllMessages()),
		TransactionHash:    txHash,
		Reverted:           trace.IsReverted(),
		RevertReason:       trace.RevertReason(),
	}
}

func StateDiff(trace *vm.TransactionTrace) *core.StateDiff {
	if trace.StateDiff == nil {
		return nil
	}
	stateDiff := trace.StateDiff
	newStorageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	for _, sd := range stateDiff.StorageDiffs {
		entries := make(map[felt.Felt]*felt.Felt)
		for _, entry := range sd.StorageEntries {
			val := entry.Value
			entries[entry.Key] = &val
		}
		newStorageDiffs[sd.Address] = entries
	}

	newNonces := make(map[felt.Felt]*felt.Felt)
	for _, nonce := range stateDiff.Nonces {
		nonc := nonce.Nonce
		newNonces[nonce.ContractAddress] = &nonc
	}

	newDeployedContracts := make(map[felt.Felt]*felt.Felt)
	for _, dc := range stateDiff.DeployedContracts {
		ch := dc.ClassHash
		newDeployedContracts[dc.Address] = &ch
	}

	newDeclaredV1Classes := make(map[felt.Felt]*felt.Felt)
	for _, dc := range stateDiff.DeclaredClasses {
		cch := dc.CompiledClassHash
		newDeclaredV1Classes[dc.ClassHash] = &cch
	}

	newReplacedClasses := make(map[felt.Felt]*felt.Felt)
	for _, rc := range stateDiff.ReplacedClasses {
		ch := rc.ClassHash
		newReplacedClasses[rc.ContractAddress] = &ch
	}

	return &core.StateDiff{
		StorageDiffs:      newStorageDiffs,
		Nonces:            newNonces,
		DeployedContracts: newDeployedContracts,
		DeclaredV0Classes: stateDiff.DeprecatedDeclaredClasses,
		DeclaredV1Classes: newDeclaredV1Classes,
		ReplacedClasses:   newReplacedClasses,
	}
}

func (b *Builder) listenPool(ctx context.Context) error {
	for {
		if err := b.depletePool(ctx); err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-b.pool.Wait():
			continue
		}
	}
}

func (b *Builder) depletePool(ctx context.Context) error {
	for {
		userTxn, err := b.pool.Pop()
		if err != nil {
			return err
		}
		b.log.Debugw("running txn", "hash", userTxn.Transaction.Hash().String())
		if err = b.runTxn(&userTxn); err != nil {
			var txnExecutionError vm.TransactionExecutionError
			if !errors.As(err, &txnExecutionError) {
				return err
			}
			b.log.Debugw("failed txn", "hash", userTxn.Transaction.Hash().String(), "err", err.Error())
		}

		if b.shadowMode {
			numTxnsToExecute := <-b.chanNumTxnsToShadow
			b.chanNumTxnsToShadow <- numTxnsToExecute - 1
			if numTxnsToExecute-1 == 0 {
				b.chanFinaliseShadow <- struct{}{}
				<-b.chanNumTxnsToShadow
			}
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func (b *Builder) runTxn(txn *mempool.BroadcastedTransaction) error {
	b.pendingLock.Lock()
	defer b.pendingLock.Unlock()
	state := blockchain.NewPendingStateWriter(b.pendingBlock.StateUpdate.StateDiff, b.pendingBlock.NewClasses, b.headState)
	var classes []core.Class
	if txn.DeclaredClass != nil {
		classes = append(classes, txn.DeclaredClass)
	}
	feesPaidOnL1 := []*felt.Felt{new(felt.Felt).SetUint64(1)}
	blockInfo := &vm.BlockInfo{
		Header: &core.Header{
			Number:           b.shadowBlock.Number,           // Affects post 0.13.2 block hash
			SequencerAddress: b.shadowBlock.SequencerAddress, // Affects post 0.13.2 block hash
			Timestamp:        b.shadowBlock.Timestamp,        // Affects post 0.13.2 block hash
			ProtocolVersion:  b.shadowBlock.ProtocolVersion,  // Affects post 0.13.2 block hash
			GasPrice:         b.shadowBlock.GasPrice,         // Affects post 0.13.2 block hash
			GasPriceSTRK:     b.shadowBlock.GasPriceSTRK,     // Affects post 0.13.2 block hash
			L1DataGasPrice:   b.shadowBlock.L1DataGasPrice,   // Affects post 0.13.2 block hash
			L1DAMode:         b.shadowBlock.L1DAMode,         // Affects data_availability
		},
		BlockHashToBeRevealed: b.blockHashToBeRevealed,
	}
	fee, _, trace, txnReceipts, _, err := b.vm.Execute([]core.Transaction{txn.Transaction}, classes, feesPaidOnL1, blockInfo, state,
		b.bc.Network(), false, false, false, true)
	if err != nil {
		return err
	}

	feeUnit := core.WEI
	if txn.Transaction.TxVersion().Is(3) {
		feeUnit = core.STRK
	}
	if trace[0].StateDiff.DeclaredClasses != nil || trace[0].StateDiff.DeprecatedDeclaredClasses != nil {
		if t, ok := (txn.Transaction).(*core.DeclareTransaction); ok {
			err := state.SetContractClass(t.ClassHash, txn.DeclaredClass)
			if err != nil {
				b.log.Errorw("failed to set contract class : %s", err)
			}
			if t.CompiledClassHash != nil {
				err := state.SetCompiledClassHash(t.ClassHash, t.CompiledClassHash)
				if err != nil {
					b.log.Errorw("failed to SetCompiledClassHash  : %s", err)
				}
			}
		}
	}

	receipt := Receipt(fee[0], feeUnit, txn.Transaction.Hash(), &trace[0], &txnReceipts[0])

	if b.junoEndpoint != "" {
		seqTrace := vm2core.AdaptStateDiff(trace[0].StateDiff)
		refTrace := vm2core.AdaptStateDiff(b.blockTraces[b.pendingBlock.Block.TransactionCount].TraceRoot.StateDiff)
		diffString, diffsNotEqual := seqTrace.Diff(refTrace, "sequencer", "sepolia")
		if diffsNotEqual {
			// Can't be fatal since FGW may remove values later (eg if the storage update element doesn't alter state)
			fmt.Println(diffString)
			b.log.Debugw("Generated transaction trace does not match that from Sepolia ")
		}

		// Note: the error message changes between blockifier-rc2 and blockifier-rc3.
		// If we run with blockifier-rc3, we won't get the same revert-reason that was
		// generated if the FGW was running blockifier-rc2. We account for this here.
		receipt.RevertReason = b.shadowBlock.Receipts[b.pendingBlock.Block.TransactionCount].RevertReason
		if differ, diffStr := core.CompareReceipts(receipt, b.shadowBlock.Receipts[b.pendingBlock.Block.TransactionCount]); differ {
			b.log.Debugw("CompareReceipts")
			b.log.Debugw(diffStr)
		}
		// b.pendingBlock.StateUpdate.StateDiff.Print()
		// b.shadowStateUpdate.StateDiff.Print()
	}
	b.pendingBlock.Block.Receipts = append(b.pendingBlock.Block.Receipts, receipt)
	b.pendingBlock.Block.Transactions = append(b.pendingBlock.Block.Transactions, txn.Transaction)
	b.pendingBlock.Block.TransactionCount = uint64(len(b.pendingBlock.Block.Transactions))
	b.pendingBlock.Block.EventCount += uint64(len(receipt.Events))
	b.pendingBlock.StateUpdate.StateDiff = mergeStateDiffs(b.pendingBlock.StateUpdate.StateDiff, StateDiff(&trace[0]))
	return nil
}

func mergeStateDiffs(oldStateDiff, newStateDiff *core.StateDiff) *core.StateDiff {
	mergeMaps := func(oldMap, newMap map[felt.Felt]*felt.Felt) {
		for key, value := range newMap {
			oldMap[key] = value
		}
	}

	mergeStorageDiffs := func(oldMap, newMap map[felt.Felt]map[felt.Felt]*felt.Felt) {
		for addr, newAddrStorage := range newMap {
			if oldAddrStorage, exists := oldMap[addr]; exists {
				mergeMaps(oldAddrStorage, newAddrStorage)
			} else {
				oldMap[addr] = newAddrStorage
			}
		}
	}

	mergeStorageDiffs(oldStateDiff.StorageDiffs, newStateDiff.StorageDiffs)
	mergeMaps(oldStateDiff.Nonces, newStateDiff.Nonces)
	mergeMaps(oldStateDiff.DeployedContracts, newStateDiff.DeployedContracts)
	mergeMaps(oldStateDiff.DeclaredV1Classes, newStateDiff.DeclaredV1Classes)
	mergeMaps(oldStateDiff.ReplacedClasses, newStateDiff.ReplacedClasses)
	oldStateDiff.DeclaredV0Classes = append(oldStateDiff.DeclaredV0Classes, newStateDiff.DeclaredV0Classes...)

	return oldStateDiff
}

func (b *Builder) shadowTxns(ctx context.Context) error {
	for {
		b.chanFinalised <- struct{}{}
		builderHeadBlock, err := b.bc.Head()
		if err != nil {
			return err
		}
		nextBlockToSequence := builderHeadBlock.Number + 1
		snHeadBlock, err := b.starknetData.BlockLatest(ctx)
		if err != nil {
			return err
		}

		b.log.Debugw(fmt.Sprintf("Juno currently at block %d, Sepolia at block %d. Attempting to sequence block %d.",
			builderHeadBlock.Number, snHeadBlock.Number, nextBlockToSequence))
		if builderHeadBlock.Number < snHeadBlock.Number {
			block, su, classes, err := b.getSyncData(nextBlockToSequence)
			if err != nil {
				return err
			}
			if b.junoEndpoint != "" {
				blockTraces, err := b.JunoGetBlockTrace(int(block.Number))
				if err != nil {
					return err
				}
				if len(blockTraces) != int(block.TransactionCount) {
					b.log.Fatalf("number of transaction traces does not equal the number of transactions")
				}
				b.blockTraces = blockTraces
			}

			b.shadowStateUpdate = su
			b.shadowBlock = block
			b.pendingBlock.Block.Transactions = nil
			b.pendingBlock.Block.SequencerAddress = block.SequencerAddress      // Affects post 0.13.2 block hash
			b.pendingBlock.Block.Timestamp = block.Timestamp                    // Affects post 0.13.2 block hash
			b.pendingBlock.Block.Header.ProtocolVersion = block.ProtocolVersion // Affects post 0.13.2 block hash
			b.pendingBlock.Block.Header.GasPrice = block.GasPrice               // Affects post 0.13.2 block hash
			b.pendingBlock.Block.Header.GasPriceSTRK = block.GasPriceSTRK       // Affects post 0.13.2 block hash
			b.pendingBlock.Block.Header.L1DataGasPrice = block.L1DataGasPrice   // Affects post 0.13.2 block hash
			b.pendingBlock.Block.Header.L1DAMode = block.L1DAMode               // Affects data_availability

			blockHashStorage := b.pendingBlock.StateUpdate.StateDiff.StorageDiffs[*new(felt.Felt).SetUint64(1)]
			for _, blockHash := range blockHashStorage {
				b.blockHashToBeRevealed = blockHash
			}

			b.chanNumTxnsToShadow <- int(block.TransactionCount)
			for _, txn := range block.Transactions {
				var declaredClass core.Class
				declareTxn, ok := txn.(*core.DeclareTransaction)
				if ok {
					declaredClass = classes[*declareTxn.ClassHash]
				}
				err = b.pool.Push(
					&mempool.BroadcastedTransaction{
						Transaction:   txn,
						DeclaredClass: declaredClass,
					})
				if err != nil {
					return err
				}
			}

		} else {
			var sleepTime uint = 1
			b.log.Debugw("Juno Sequencer is at Sepolia chain head. Sleeping for %ds before querying for a new block.", sleepTime)
			time.Sleep(time.Duration(sleepTime))
		}
	}
}

func (b *Builder) syncStore(curBlockNum, toBlockNum uint64) error {
	var i uint64
	for i = curBlockNum + 1; i < toBlockNum; i++ {
		b.log.Infow("Sequencer, syncing block", "blockNumber", i)
		block, su, classes, err := b.getSyncData(i)
		if err != nil {
			return err
		}
		commitments, err := b.bc.SanityCheckNewHeight(block, su, classes)
		if err != nil {
			return err
		}
		err = b.bc.Store(block, commitments, su, classes)
		if err != nil {
			return err
		}
		seqBlock, err := b.bc.BlockByNumber(i)
		if err != nil {
			return err
		}
		if !seqBlock.GlobalStateRoot.Equal(block.GlobalStateRoot) {
			return fmt.Errorf("sequencers state root %s != shadow block state root %s",
				seqBlock.GlobalStateRoot.String(), block.GlobalStateRoot.String())
		}
	}
	return nil
}

func (b *Builder) getSyncData(blockNumber uint64) (*core.Block, *core.StateUpdate,
	map[felt.Felt]core.Class, error,
) {
	block, err := b.starknetData.BlockByNumber(context.Background(), blockNumber)
	if err != nil {
		return nil, nil, nil, err
	}
	su, err := b.starknetData.StateUpdate(context.Background(), blockNumber)
	if err != nil {
		return nil, nil, nil, err
	}
	txns := block.Transactions
	classes := make(map[felt.Felt]core.Class)
	for _, txn := range txns {
		if t, ok := txn.(*core.DeclareTransaction); ok {
			class, err := b.starknetData.Class(context.Background(), t.ClassHash)
			if err != nil {
				return nil, nil, nil, err
			}
			classes[*t.ClassHash] = class
		}
	}
	return block, su, classes, nil
}

func (b *Builder) JunoGetBlockTrace(blockNum int) ([]rpc.TracedBlockTransaction, error) {
	type RequestPayload struct {
		JSONRPC string                 `json:"jsonrpc"`
		Method  string                 `json:"method"`
		Params  map[string]interface{} `json:"params"`
		ID      int                    `json:"id"`
	}
	type ResponsePayload struct {
		JSONRPC string      `json:"jsonrpc"`
		Result  interface{} `json:"result"`
		Error   interface{} `json:"error"`
		ID      int         `json:"id"`
	}
	payload := RequestPayload{
		JSONRPC: "2.0",
		Method:  "starknet_traceBlockTransactions",
		Params: map[string]interface{}{
			"block_id": map[string]int{"block_number": blockNum},
		},
		ID: 1,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequest("POST", b.junoEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HTTP request: %w", err)
	}
	defer resp.Body.Close()

	var responsePayload ResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&responsePayload); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}
	var tracedTransactions []rpc.TracedBlockTransaction
	resultBytes, err := json.Marshal(responsePayload.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	if err := json.Unmarshal(resultBytes, &tracedTransactions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result into []TracedBlockTransaction: %w", err)
	}
	return tracedTransactions, nil
}
