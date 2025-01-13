package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	stdsync "sync"

	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/plugin"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/sourcegraph/conc"
)

var (
	_                    service.Service = (*Builder)(nil)
	_                    sync.Reader     = (*Builder)(nil)
	ErrPendingParentHash                 = errors.New("pending block parent hash does not match chain head")
)

type Builder struct {
	ownAddress felt.Felt
	privKey    *ecdsa.PrivateKey

	bc        *blockchain.Blockchain
	db        db.DB
	vm        vm.VM
	newHeads  *feed.Feed[*core.Header]
	log       utils.Logger
	blockTime time.Duration
	pool      *mempool.Pool
	listener  EventListener

	pendingBlock atomic.Pointer[sync.Pending]
	headState    core.StateReader
	headCloser   blockchain.StateCloser

	disableFees       bool
	shadowMode        bool
	shadowStateUpdate *core.StateUpdate
	shadowBlock       *core.Block
	shadowSyncToBlock uint64
	starknetData      starknetdata.StarknetData
	junoEndpoint      string
	snBlockTraces     []rpc.TracedBlockTransaction

	chanNumTxnsToShadow chan int // Todo: can remove when we execute transactions in blocks
	chanFinaliseBlock   chan struct{}
	chanFinalised       chan struct{}

	blockHashToBeRevealed *felt.Felt

	muSubs         stdsync.Mutex // protects subscriptions.
	subscriptions  map[uint64]*subscription
	pendingTxsFeed *feed.Feed[[]core.Transaction]

	plugin plugin.JunoPlugin
}

type subscription struct {
	cancel func()
	wg     conc.WaitGroup
	conn   jsonrpc.Conn
}

func New(privKey *ecdsa.PrivateKey, ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM,
	blockTime time.Duration, pool *mempool.Pool, log utils.Logger, disableFees bool, database db.DB,
) *Builder {
	return &Builder{
		ownAddress: *ownAddr,
		privKey:    privKey,
		blockTime:  blockTime,
		log:        log,
		listener:   &SelectiveListener{},

		disableFees:    disableFees,
		bc:             bc,
		db:             database,
		pool:           pool,
		vm:             builderVM,
		newHeads:       feed.New[*core.Header](),
		pendingTxsFeed: feed.New[[]core.Transaction](),
	}
}

func NewShadow(privKey *ecdsa.PrivateKey, ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM,
	blockTime time.Duration, pool *mempool.Pool, log utils.Logger, starknetData starknetdata.StarknetData,
	database db.DB,
) *Builder {
	return &Builder{
		ownAddress: *ownAddr,
		privKey:    privKey,
		blockTime:  blockTime,
		log:        log,
		listener:   &SelectiveListener{},

		chanFinaliseBlock:   make(chan struct{}, 1),
		chanFinalised:       make(chan struct{}, 1),
		chanNumTxnsToShadow: make(chan int, 1),

		bc:       bc,
		db:       database,
		pool:     pool,
		vm:       builderVM,
		newHeads: feed.New[*core.Header](),

		shadowMode:     true,
		starknetData:   starknetData,
		pendingTxsFeed: feed.New[[]core.Transaction](),
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

func (b *Builder) WithPlugin(junoPlugin plugin.JunoPlugin) *Builder {
	b.plugin = junoPlugin
	return b
}

func (b *Builder) WithSyncToBlock(syncTo uint64) *Builder {
	b.shadowSyncToBlock = syncTo
	return b
}

func (b *Builder) Pending() (*sync.Pending, error) {
	p := b.pendingBlock.Load()
	if p == nil {
		return nil, sync.ErrPendingBlockNotFound
	}

	expectedParentHash := &felt.Zero
	if head, err := b.bc.HeadsHeader(); err == nil {
		expectedParentHash = head.Hash
	}
	if p.Block.ParentHash.Equal(expectedParentHash) {
		return p, nil
	}

	return nil, ErrPendingParentHash
}

func (b *Builder) PendingBlock() *core.Block {
	pending, err := b.Pending()
	if err != nil {
		return nil
	}
	return pending.Block
}

func (b *Builder) PendingState() (core.StateReader, func() error, error) {
	txn, err := b.db.NewTransaction(false)
	if err != nil {
		return nil, nil, err
	}

	pending, err := b.Pending()
	if err != nil {
		return nil, nil, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return sync.NewPendingState(pending.StateUpdate.StateDiff, pending.NewClasses, core.NewState(txn)), txn.Discard, nil
}

func (b *Builder) Run(ctx context.Context) error {
	defer func() {
		if pErr := b.ClearPending(); pErr != nil {
			b.log.Errorw("clearing pending", "err", pErr)
		}
	}()
	switch {
	case b.shadowMode:
		return b.runShadowMode(ctx)
	default:
		return b.runSequencer(ctx)
	}
}

func (b *Builder) ClearPending() error {
	b.pendingBlock.Store(&sync.Pending{})

	if b.headState != nil {
		if err := b.headCloser(); err != nil {
			return err
		}
		b.headState = nil
		b.headCloser = nil
	}
	return nil
}

func (b *Builder) runSequencer(ctx context.Context) error {
	if err := b.InitPendingBlock(); err != nil {
		return err
	}

	doneListen := make(chan struct{})
	go func() {
		if pErr := b.listenPool(ctx); pErr != nil {
			b.log.Errorw("listening pool", "err", pErr)
		}
		close(doneListen)
	}()

	for {
		select {
		case <-ctx.Done():
			<-doneListen
			return nil
		case <-time.After(b.blockTime):
			b.log.Infof("Finalising new block")
			err := b.Finalise(b.Sign)
			if err != nil {
				return err
			}
		}
	}
}

func (b *Builder) runShadowMode(ctx context.Context) error {
	if err := b.syncStore(); err != nil {
		return err
	}

	if err := b.InitPendingBlock(); err != nil {
		return err
	}

	doneListen := make(chan struct{})
	go func() {
		// Todo: in shadow mode, we can execute all transactions at once,
		// popping them from the mempool and executing one by one is less efficient
		if pErr := b.listenPool(ctx); pErr != nil {
			b.log.Errorw("listening pool", "err", pErr)
		}
		close(doneListen)
	}()

	go func() {
		if pErr := b.shadowTxns(ctx); pErr != nil {
			b.log.Errorw("shadowing transactions", "err", pErr)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			<-doneListen
			return nil
		case <-b.chanFinaliseBlock:
			err := b.cleanStorageDiff()
			if err != nil {
				return err
			}
			b.log.Infof("Finalising new block")
			err = b.Finalise(nil)
			if err != nil {
				return err
			}
			<-b.chanFinalised
		}
	}
}

func (b *Builder) cleanStorageDiff() error {
	b.log.Debugw("Removing values in the storage diff that don't affect state")
	pending, err := b.Pending()
	if err != nil {
		return err
	}
	sd := pending.StateUpdate.StateDiff
	state, stateCloser, err := b.PendingState()
	if err != nil {
		return err
	}

	defer func() {
		if err = stateCloser(); err != nil {
			b.log.Errorw("closing state in cleanStorageDiff", "err", err)
		}
	}()

	for addr, storage := range sd.StorageDiffs {
		for k, v := range storage {
			previousValue, err := state.ContractStorage(&addr, &k)
			if err != nil {
				return err
			}
			if previousValue.Equal(v) {
				b.log.Debugw("the key %v at the storage of address %v is being deleted", k.String(), addr.String())
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
	b.pendingBlock.Store(pending)
	return nil
}

func (b *Builder) InitPendingBlock() error {
	header, err := b.bc.HeadsHeader()
	if err != nil {
		return err
	}
	pendingBlock := core.Block{
		Header: &core.Header{
			ParentHash:       header.Hash,
			Number:           header.Number + 1,
			SequencerAddress: &b.ownAddress,
			GasPrice:         new(felt.Felt).SetUint64(1),
			GasPriceSTRK:     new(felt.Felt).SetUint64(1),
			L1DAMode:         core.Calldata,
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: new(felt.Felt).SetUint64(1),
				PriceInFri: new(felt.Felt).SetUint64(1),
			},
		},
	}
	newClasses := map[felt.Felt]core.Class{}
	su := core.StateUpdate{
		StateDiff: core.EmptyStateDiff(),
	}
	pending := sync.Pending{
		Block:       &pendingBlock,
		StateUpdate: &su,
		NewClasses:  newClasses,
	}
	b.pendingBlock.Store(&pending)
	b.headState, b.headCloser, err = b.bc.HeadState()
	return err
}

// Finalise the pending block and initialise the next one
func (b *Builder) Finalise(signFunc blockchain.BlockSignFunc) error {
	pending, err := b.Pending()
	if err != nil {
		return err
	}
	if err := b.bc.Finalise(pending.Block, pending.StateUpdate, pending.NewClasses, b.Sign, b.shadowStateUpdate, b.shadowBlock); err != nil {
		return err
	}
	b.log.Infow("Finalised block", "number", pending.Block.Number, "hash",
		pending.Block.Hash.ShortString(), "state", pending.Block.GlobalStateRoot.ShortString())
	b.listener.OnBlockFinalised(pending.Block.Header)

	if b.plugin != nil {
		err := b.plugin.NewBlock(pending.Block, pending.StateUpdate, pending.NewClasses)
		if err != nil {
			b.log.Errorw("error sending new block to plugin", err)
		}
	}
	if err := b.ClearPending(); err != nil {
		return err
	}
	return b.InitPendingBlock()
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

func (b *Builder) listenPool(ctx context.Context) error {
	for {
		if err := b.depletePool(ctx); err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}
		}

		// send the pending transactions to the feed
		b.pendingTxsFeed.Send(b.PendingBlock().Transactions)

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
			b.log.Debugw("failed txn", "hash", userTxn.Transaction.Hash().String(), "err", err.Error())
			var txnExecutionError vm.TransactionExecutionError
			if !errors.As(err, &txnExecutionError) {
				return err
			}
		}

		if b.shadowMode {
			numTxnsToExecute := <-b.chanNumTxnsToShadow
			b.chanNumTxnsToShadow <- numTxnsToExecute - 1
			if numTxnsToExecute-1 == 0 {
				b.chanFinaliseBlock <- struct{}{}
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

func (b *Builder) runTxn(txn *mempool.BroadcastedTransaction) error { //nolint:gocyclo
	pending, err := b.Pending()
	if err != nil {
		return err
	}
	state := sync.NewPendingStateWriter(pending.StateUpdate.StateDiff, pending.NewClasses, b.headState)

	var classes []core.Class
	if txn.DeclaredClass != nil {
		classes = append(classes, txn.DeclaredClass)
	}
	fee, _, trace, txnReceipts, _, err := b.vm.Execute(
		[]core.Transaction{txn.Transaction},
		classes,
		[]*felt.Felt{new(felt.Felt).SetUint64(1)},
		&vm.BlockInfo{
			Header:                pending.Block.Header,
			BlockHashToBeRevealed: b.blockHashToBeRevealed,
		},
		state,
		b.bc.Network(),
		false, false, false)
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

	receipt := vm2core.Receipt(fee[0], feeUnit, txn.Transaction.Hash(), &trace[0], &txnReceipts[0])
	if b.shadowBlock != nil {
		err = b.overrideTraces(receipt)
		if err != nil {
			return err
		}
	}
	if b.junoEndpoint != "" {
		seqTrace := vm2core.AdaptStateDiff(trace[0].StateDiff)
		refTrace := vm2core.AdaptStateDiff(b.snBlockTraces[pending.Block.TransactionCount].TraceRoot.StateDiff)
		diffString, diffsNotEqual := seqTrace.Diff(refTrace, "sequencer", "sepolia")
		if diffsNotEqual {
			// Can't be fatal since FGW may remove values later (eg if the storage update element doesn't alter state)
			b.log.Debugw("Generated transaction trace does not match that from Sepolia")
			b.log.Debugw(diffString) // Todo: Debug doesn't seem to format this nicely, use print?
		}

		if differ, diffStr := core.CompareReceipts(receipt, b.shadowBlock.Receipts[pending.Block.TransactionCount]); differ {
			b.log.Debugw("CompareReceipts")
			b.log.Debugw(diffStr)
		}
	}

	pending.Block.Receipts = append(pending.Block.Receipts, receipt)
	pending.Block.Transactions = append(pending.Block.Transactions, txn.Transaction)
	pending.Block.TransactionCount += 1
	pending.Block.EventCount += uint64(len(receipt.Events))
	pending.StateUpdate.StateDiff = MergeStateDiffs(pending.StateUpdate.StateDiff, vm2core.StateDiff(&trace[0]))
	return b.StorePending(pending)
}

// StorePending stores a pending block given that it is for the next height
func (b *Builder) StorePending(newPending *sync.Pending) error {
	expectedParentHash := new(felt.Felt)
	h, err := b.bc.HeadsHeader()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	} else if err == nil {
		expectedParentHash = h.Hash
	}

	if !expectedParentHash.Equal(newPending.Block.ParentHash) {
		return fmt.Errorf("store pending: %w", blockchain.ErrParentDoesNotMatchHead)
	}
	b.pendingBlock.Store(newPending)
	return nil
}

func MergeStateDiffs(oldStateDiff, newStateDiff *core.StateDiff) *core.StateDiff {
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

func (b *Builder) overrideTraces(receipt *core.TransactionReceipt) error {
	// Note: the error message changes between blockifier-rc2 and blockifier-rc3.
	// If we run with blockifier-rc3, we won't get the same revert-reason that was
	// generated if the FGW was running blockifier-rc2. We account for this here.
	pending, err := b.Pending()
	if err != nil {
		return err
	}
	receipt.RevertReason = b.shadowBlock.Receipts[pending.Block.TransactionCount].RevertReason
	return nil
}

// shadowTxns pulls transactions from the FGW and feeds them into the mempool for execution.
// If the optional RPC endpoint is provided, traces are queried and compared against those
// generated by the Juno sequencer, which can be very helpful with debugging.
func (b *Builder) shadowTxns(ctx context.Context) error { //nolint:gocyclo
	for {
		b.chanFinalised <- struct{}{}
		builderHeadBlock, err := b.bc.Head()
		if err != nil {
			return err
		}
		nextBlockToSequence := builderHeadBlock.Number + 1

		var snHeadBlock *core.Block
		snHeadBlock, err = b.starknetData.BlockLatest(ctx)
		if err != nil {
			return err
		}
		for nextBlockToSequence >= snHeadBlock.Number {
			var sleepTime uint = 5
			b.log.Infof("Juno Sequencer is at Sepolia chain head. Sleeping for %ds before querying for a new block.", sleepTime)
			time.Sleep(time.Second * time.Duration(sleepTime))
			snHeadBlock, err = b.starknetData.BlockLatest(ctx)
			if err != nil {
				return err
			}
		}

		b.log.Infof(fmt.Sprintf("Juno currently at block %d, Sepolia at block %d. Attempting to sequence block %d.",
			builderHeadBlock.Number, snHeadBlock.Number, nextBlockToSequence))

		block, su, classes, err := b.getSyncData(nextBlockToSequence)
		if err != nil {
			return err
		}
		if b.junoEndpoint != "" {
			snBlockTraces, err := b.rpcGetBlockTrace(int(block.Number))
			if err != nil {
				return err
			}
			if len(snBlockTraces) != int(block.TransactionCount) {
				b.log.Fatalf("number of transaction traces does not equal the number of transactions")
			}
			b.snBlockTraces = snBlockTraces
		}

		pending, err := b.Pending()
		if err != nil {
			return err
		}

		b.shadowStateUpdate = su
		b.shadowBlock = block
		err = b.setPendingHeader(block, nextBlockToSequence)
		if err != nil {
			return err
		}
		blockHashStorage := pending.StateUpdate.StateDiff.StorageDiffs[*new(felt.Felt).SetUint64(1)]
		for _, blockHash := range blockHashStorage {
			b.blockHashToBeRevealed = blockHash // Affects execution
		}

		// Todo: should be able to sequence the entire block of transactions at once (ie skip mempool)
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
	}
}

func (b *Builder) setPendingHeader(refBlock *core.Block, nextBlockToSequence uint64) error {
	pending, err := b.Pending()
	if err != nil {
		return err
	}
	pending.Block.Transactions = nil
	pending.Block.Number = nextBlockToSequence
	pending.Block.SequencerAddress = refBlock.SequencerAddress      // Affects post 0.13.2 block hash
	pending.Block.Timestamp = refBlock.Timestamp                    // Affects post 0.13.2 block hash
	pending.Block.Header.ProtocolVersion = refBlock.ProtocolVersion // Affects post 0.13.2 block hash
	pending.Block.Header.GasPrice = refBlock.GasPrice               // Affects post 0.13.2 block hash
	pending.Block.Header.GasPriceSTRK = refBlock.GasPriceSTRK       // Affects post 0.13.2 block hash
	pending.Block.Header.L1DataGasPrice = refBlock.L1DataGasPrice   // Affects post 0.13.2 block hash
	pending.Block.Header.L1DAMode = refBlock.L1DAMode               // Affects data_availability
	return b.StorePending(pending)
}

// syncStore pulls blocks, classes and state-updates directly from the FGW and stores them in the
// blockchain. This is needed when a block can not be sequenced, eg Sepolia block0 uses deprecated
// transactions to bootstrap the network, etc.
func (b *Builder) syncStore() error {
	syncFromBlock := uint64(0)
	syncToBlock := b.shadowSyncToBlock
	block, err := b.bc.Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	if block != nil {
		syncFromBlock = block.Number + 1
	}

	if syncFromBlock > syncToBlock {
		b.log.Debugw("Skipping sync: sequencer is already ahead of or at the target block",
			"next block to sync", syncFromBlock, "targetBlock", syncToBlock)
		return nil
	}

	var i uint64
	for i = syncFromBlock; i <= syncToBlock; i++ {
		b.log.Infow("Syncing block number", "blockNumber", i)
		block, su, classes, err := b.getSyncData(i)
		if err != nil {
			return err
		}
		err = b.StorePending(&sync.Pending{
			Block:       block,
			NewClasses:  classes,
			StateUpdate: su,
		})
		if err != nil {
			return err
		}
		err = b.Finalise(nil)
		if err != nil {
			return err
		}
	}
	return b.ClearPending()
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

// rpcGetBlockTrace helps debug traces by comparing them against the traces that are returned
// over RPC. This method is not needed to run the sequencer, but it's useful for debugging.
func (b *Builder) rpcGetBlockTrace(blockNum int) ([]rpc.TracedBlockTransaction, error) {
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, b.junoEndpoint, bytes.NewBuffer(jsonData))
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

func (b *Builder) SubscribePendingTxs() sync.PendingTxSubscription {
	return sync.PendingTxSubscription{b.pendingTxsFeed.Subscribe()}
}

// The builder has no reorg logic (centralised sequencer that can't reorg)
func (b *Builder) SubscribeReorg() sync.ReorgSubscription {
	return sync.ReorgSubscription{}
}
