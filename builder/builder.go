package builder

import (
	"context"
	"errors"
	"time"

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

	bc       *blockchain.Blockchain
	vm       vm.VM
	newHeads *feed.Feed[*core.Header]
	log      utils.Logger

	PendingBlock *blockchain.Pending
}

func New(privKey *ecdsa.PrivateKey, ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM, log utils.Logger) *Builder {
	return &Builder{
		ownAddress: *ownAddr,
		privKey:    privKey,

		bc:       bc,
		vm:       builderVM,
		newHeads: feed.New[*core.Header](),
		log:      log,
	}
}

func (b *Builder) Run(ctx context.Context) error {
	if err := b.InitPendingBlock(); err != nil {
		return err
	}

	const blockInterval = 5 * time.Second
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(blockInterval):
			if err := b.Finalise(); err != nil {
				return err
			}
		}
	}
}

// InitPendingBlock initialises a new pending block
func (b *Builder) InitPendingBlock() error {
	nextHeight := uint64(0)
	parentHash := felt.Zero
	parentState := felt.Zero
	if head, err := b.bc.HeadsHeader(); err == nil {
		nextHeight = head.Number + 1
		parentHash = *head.Hash
		// todo: this is not entirely correct, should consider genesis state as well
		parentState = *head.GlobalStateRoot
	}

	receipts := make([]*core.TransactionReceipt, 0)
	pendingBlock := &core.Block{
		Header: &core.Header{
			ParentHash:       &parentHash,
			SequencerAddress: &b.ownAddress,
			Timestamp:        uint64(time.Now().Unix()), // todo: genesis timestamp should be configurable
			ProtocolVersion:  blockchain.SupportedStarknetVersion.Original(),
			EventsBloom:      core.EventsBloom(receipts),
			GasPrice:         new(felt.Felt).SetUint64(1), // todo
			GasPriceSTRK:     new(felt.Felt).SetUint64(2), // todo
		},
		Transactions: make([]core.Transaction, 0),
		Receipts:     receipts,
	}

	stateDiff, err := blockchain.MakeStateDiffForEmptyBlock(b.bc, nextHeight)
	if err != nil {
		return err
	}

	b.PendingBlock = &blockchain.Pending{
		Block: pendingBlock,
		StateUpdate: &core.StateUpdate{
			OldRoot:   &parentState,
			StateDiff: stateDiff,
		},
		NewClasses: make(map[felt.Felt]core.Class, 0),
	}
	return nil
}

// Finalise the pending block and initialise the next one
func (b *Builder) Finalise() error {
	var err error
	if err = b.bc.Finalise(b.PendingBlock, b.Sign); err != nil {
		return err
	}
	b.log.Infow("Finalised block", "number", b.PendingBlock.Block.Number, "hash",
		b.PendingBlock.Block.Hash.ShortString(), "state", b.PendingBlock.Block.GlobalStateRoot.ShortString())

	return b.InitPendingBlock()
}

// ValidateAgainstPendingState validates a user transaction against the pending state
// only hard-failures result in an error, reverts are not reported back to caller
func (b *Builder) ValidateAgainstPendingState(userTxn *mempool.BroadcastedTransaction) error {
	declaredClasses := []core.Class{}
	if userTxn.DeclaredClass != nil {
		declaredClasses = []core.Class{userTxn.DeclaredClass}
	}

	nextHeight := uint64(0)
	if height, err := b.bc.Height(); err == nil {
		nextHeight = height + 1
	} else if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
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

	_, _, err = b.vm.Execute([]core.Transaction{userTxn.Transaction}, declaredClasses, nextHeight,
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
