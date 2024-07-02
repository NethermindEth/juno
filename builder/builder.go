package builder

import (
	"context"
	"errors"
	"fmt"
	stdsync "sync"
	"time"

	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
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

	prefundAccounts  bool
	bootstrap        bool
	bootstrapToBlock uint64
	starknetData     starknetdata.StarknetData
}

func New(privKey *ecdsa.PrivateKey, ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM,
	blockTime time.Duration, pool *mempool.Pool, log utils.Logger,
) *Builder {
	return &Builder{
		ownAddress: *ownAddr,
		privKey:    privKey,
		blockTime:  blockTime,
		log:        log,
		listener:   &SelectiveListener{},

		bc:       bc,
		pool:     pool,
		vm:       builderVM,
		newHeads: feed.New[*core.Header](),
	}
}

func (b *Builder) WithEventListener(l EventListener) *Builder {
	b.listener = l
	return b
}

func (b *Builder) WithBootstrap(bootstrap bool) *Builder {
	b.bootstrap = bootstrap
	return b
}

func (b *Builder) WithStarknetData(starknetData starknetdata.StarknetData) *Builder {
	b.starknetData = starknetData
	return b
}

func (b *Builder) WithBootstrapToBlock(bootstrapToBlock uint64) *Builder {
	b.bootstrapToBlock = bootstrapToBlock
	return b
}

func (b *Builder) WithPrefundAccounts(prefundAccounts bool) *Builder {
	b.prefundAccounts = prefundAccounts
	return b
}

func (b *Builder) BootstrapSeq(ctx context.Context, toBlockNum uint64) error {
	var i uint64
	for i = 0; i < toBlockNum; i++ {
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
	}
	return nil
}

func (b *Builder) Run(ctx context.Context) error {
	if b.bootstrap {
		err := b.BootstrapSeq(ctx, b.bootstrapToBlock)
		if err != nil {
			return err
		}
	}

	if b.prefundAccounts {
		fmt.Println("building genesis state with prefunded accounts.")
		initMintAmnt := new(felt.Felt).SetUint64(1_000_000_000_000)
		classes := []string{"./genesis/testdata/strk.json", "./genesis/testdata/simpleAccount.json"}
		genesisConfig := genesis.GenesisConfigAccountsTokens(*initMintAmnt, classes)
		stateDiff, newClasses, err := genesis.GenesisStateDiff(&genesisConfig, b.vm, b.bc.Network())
		if err != nil {
			return err
		}
		err = b.bc.StoreGenesis(stateDiff, newClasses)
		if err != nil {
			return err
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

	for {
		select {
		case <-ctx.Done():
			<-doneListen
			return nil
		case <-time.After(b.blockTime):
			b.log.Debugw("Finalising new block")
			if err := b.Finalise(); err != nil {
				return err
			}
		}
	}
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
func (b *Builder) Finalise() error {
	b.pendingLock.Lock()
	defer b.pendingLock.Unlock()

	if err := b.bc.Finalise(&b.pendingBlock, b.Sign); err != nil {
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
	_, _, _, err = b.vm.Execute([]core.Transaction{userTxn.Transaction}, declaredClasses, []*felt.Felt{}, blockInfo, state, //nolint:dogsled
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

func Receipt(fee *felt.Felt, feeUnit core.FeeUnit, txHash *felt.Felt, trace *vm.TransactionTrace) *core.TransactionReceipt {
	return &core.TransactionReceipt{
		Fee:                fee,
		FeeUnit:            feeUnit,
		Events:             vm2core.AdaptOrderedEvents(trace.AllEvents()),
		ExecutionResources: vm2core.AdaptExecutionResources(trace.TotalExecutionResources()),
		L2ToL1Message:      vm2core.AdaptOrderedMessagesToL1(trace.AllMessages()),
		TransactionHash:    txHash,
		Reverted:           trace.IsReverted(),
		RevertReason:       trace.RevertReason(),
	}
}

func StateDiff(trace *vm.StateDiff) *core.StateDiff {
	newStorageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	for _, sd := range trace.StorageDiffs {
		entries := make(map[felt.Felt]*felt.Felt)
		for _, entry := range sd.StorageEntries {
			entries[entry.Key] = &entry.Value
		}
		newStorageDiffs[sd.Address] = entries
	}

	newNonces := make(map[felt.Felt]*felt.Felt)
	for _, nonce := range trace.Nonces {
		newNonces[nonce.ContractAddress] = &nonce.Nonce
	}

	newDeployedContracts := make(map[felt.Felt]*felt.Felt)
	for _, dc := range trace.DeployedContracts {
		newDeployedContracts[dc.Address] = &dc.ClassHash
	}

	newDeclaredV1Classes := make(map[felt.Felt]*felt.Felt)
	for _, dc := range trace.DeclaredClasses {
		newDeclaredV1Classes[dc.ClassHash] = &dc.CompiledClassHash
	}

	newReplacedClasses := make(map[felt.Felt]*felt.Felt)
	for _, rc := range trace.ReplacedClasses {
		newReplacedClasses[rc.ContractAddress] = &rc.ClassHash
	}

	return &core.StateDiff{
		StorageDiffs:      newStorageDiffs,
		Nonces:            newNonces,
		DeployedContracts: newDeployedContracts,
		DeclaredV0Classes: trace.DeprecatedDeclaredClasses,
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
		qwe, _ := (userTxn.Transaction).(*core.InvokeTransaction)
		fmt.Println("run txn", userTxn.Transaction, qwe.SenderAddress.String())
		if err = b.runTxn(&userTxn); err != nil {
			var txnExecutionError vm.TransactionExecutionError
			if !errors.As(err, &txnExecutionError) {
				return err
			}
			b.log.Debugw("failed txn", "hash", userTxn.Transaction.Hash().String(), "err", err.Error())
			fmt.Println("run txn err", err)
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

	blockInfo := &vm.BlockInfo{
		Header: &core.Header{
			Number:           b.pendingBlock.Block.Number,
			Timestamp:        b.pendingBlock.Block.Timestamp,
			SequencerAddress: b.pendingBlock.Block.SequencerAddress,
			GasPrice:         b.pendingBlock.Block.GasPrice,
			GasPriceSTRK:     b.pendingBlock.Block.GasPriceSTRK,
		},
	}

	fee, _, trace, err := b.vm.Execute([]core.Transaction{txn.Transaction}, classes, []*felt.Felt{}, blockInfo, state,
		b.bc.Network(), false, false, false, false)
	if err != nil {
		return err
	}

	b.pendingBlock.Block.Transactions = append(b.pendingBlock.Block.Transactions, txn.Transaction)
	b.pendingBlock.Block.TransactionCount = uint64(len(b.pendingBlock.Block.Transactions))

	feeUnit := core.WEI
	if txn.Transaction.TxVersion().Is(3) {
		feeUnit = core.STRK
	}

	receipt := Receipt(fee[0], feeUnit, txn.Transaction.Hash(), &trace[0])
	b.pendingBlock.Block.Receipts = append(b.pendingBlock.Block.Receipts, receipt)
	b.pendingBlock.Block.EventCount += uint64(len(receipt.Events))
	b.pendingBlock.StateUpdate.StateDiff = StateDiff(trace[0].StateDiff)
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
