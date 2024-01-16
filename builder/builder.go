package builder

import (
	"context"
	"errors"
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
	"github.com/NethermindEth/juno/service"
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

func (b *Builder) Run(ctx context.Context) error {
	if err := b.initPendingBlock(); err != nil {
		return err
	}
	defer func() {
		if pErr := b.clearPending(); pErr != nil {
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
			if err := b.Finalise(); err != nil {
				return err
			}
		}
	}
}

func (b *Builder) initPendingBlock() error {
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

func (b *Builder) clearPending() error {
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

	if err := b.clearPending(); err != nil {
		return err
	}
	return b.initPendingBlock()
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

	_, _, err = b.vm.Execute([]core.Transaction{userTxn.Transaction}, declaredClasses, pendingBlock.Block.Number,
		pendingBlock.Block.Timestamp, &b.ownAddress, state, b.bc.Network(), []*felt.Felt{},
		false, false, false, pendingBlock.Block.GasPrice, pendingBlock.Block.GasPriceSTRK, false)
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

	fee, trace, err := b.vm.Execute([]core.Transaction{txn.Transaction}, classes, b.pendingBlock.Block.Number,
		b.pendingBlock.Block.Timestamp, b.pendingBlock.Block.SequencerAddress, state, b.bc.Network(),
		make([]*felt.Felt, 0), false, false, false, b.pendingBlock.Block.GasPrice, b.pendingBlock.Block.GasPriceSTRK, false)
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
	return nil
}
