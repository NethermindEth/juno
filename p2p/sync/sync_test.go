package sync

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/utils"
	common "github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	syncclass "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/class"
	synccommon "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/event"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/header"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/receipt"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/state"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSyncEmptyBlock(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	testDB := memory.New()
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network)
	host := mocks.NewMockHost(mockCtrl)
	sync := New(bc, host, &utils.Sepolia, utils.NewNopZapLogger())
	client := mocks.NewMockClient(mockCtrl)
	sync.WithClient(client)

	iter := &synccommon.Iteration{
		Start:     &synccommon.Iteration_BlockNumber{BlockNumber: 0},
		Direction: synccommon.Iteration_Forward,
		Limit:     1,
		Step:      1,
	}

	blockHeaderIter := NewBlockHeadersResponseSeq(t)
	errNoBlock := errors.New("peer doesn't have this block")
	client.EXPECT().RequestBlockHeaders(gomock.Any(), &header.BlockHeadersRequest{Iteration: iter}).Return(blockHeaderIter, nil)
	client.EXPECT().RequestBlockHeaders(gomock.Any(), gomock.Any()).AnyTimes().Return(nil, errNoBlock)

	txIter := NewTransactionsResponseSeqEmpty()
	client.EXPECT().RequestTransactions(gomock.Any(), &synctransaction.TransactionsRequest{Iteration: iter}).Return(txIter, nil)

	classesIter := NewClassesResponseSeq()
	client.EXPECT().RequestClasses(gomock.Any(), &syncclass.ClassesRequest{Iteration: iter}).Return(classesIter, nil)

	eventsIter := NewEventsResponseSeq()
	client.EXPECT().RequestEvents(gomock.Any(), &event.EventsRequest{Iteration: iter}).Return(eventsIter, nil)

	stateDiffsIter := NewStateDiffsResponseSeq()
	client.EXPECT().RequestStateDiffs(gomock.Any(), &state.StateDiffsRequest{Iteration: iter}).Return(stateDiffsIter, nil)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	sync.Run(ctx)

	height, err := bc.Height()
	require.NoError(t, err)
	require.Equal(t, height, uint64(0))
}

func TestSyncNonEmptyBlock(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)
	testDB := memory.New()
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network)
	host := mocks.NewMockHost(mockCtrl)
	sync := New(bc, host, &utils.Sepolia, utils.NewNopZapLogger())
	client := mocks.NewMockClient(mockCtrl)
	sync.WithClient(client)

	iter := &synccommon.Iteration{
		Start:     &synccommon.Iteration_BlockNumber{BlockNumber: 0},
		Direction: synccommon.Iteration_Forward,
		Limit:     1,
		Step:      1,
	}

	snClient := feeder.NewTestClient(t, &utils.Sepolia)
	blockID := "0"
	blockSU0, err := snClient.StateUpdateWithBlock(context.Background(), blockID)
	sig, err := snClient.Signature(context.Background(), blockID)

	require.NoError(t, err)
	block0Core, err := sn2core.AdaptBlock(blockSU0.Block, sig)
	block0Header := core2p2p.AdaptHeader(
		block0Core.Header,
		&core.BlockCommitments{
			TransactionCommitment: blockSU0.Block.TransactionCommitment,
			EventCommitment:       blockSU0.Block.EventCommitment,
			ReceiptCommitment:     blockSU0.Block.ReceiptCommitment,
			StateDiffCommitment:   blockSU0.Block.StateDiffCommitment,
		},
		blockSU0.Block.StateDiffCommitment,
		blockSU0.Block.StateDiffLength)
	blockHeaderIter := NewBlockHeadersResponseSeq(t, block0Header)
	errNoBlock := errors.New("peer doesn't have this block")
	client.EXPECT().RequestBlockHeaders(gomock.Any(), &header.BlockHeadersRequest{Iteration: iter}).Return(blockHeaderIter, nil)
	client.EXPECT().RequestBlockHeaders(gomock.Any(), gomock.Any()).AnyTimes().Return(nil, errNoBlock)

	p2pTxns := make([]*synctransaction.TransactionInBlock, len(block0Core.Transactions))
	for i, txn := range block0Core.Transactions {
		p2pTxns[i] = core2p2p.AdaptTransaction(txn)
	}
	p2pReceipts := make([]*receipt.Receipt, len(block0Core.Transactions))
	for i, receipt := range block0Core.Receipts {
		p2pReceipts[i] = core2p2p.AdaptReceipt(receipt, block0Core.Transactions[i])
	}
	txIter := NewTransactionsResponseSeqFrom(p2pTxns, p2pReceipts)
	client.EXPECT().RequestTransactions(gomock.Any(), &synctransaction.TransactionsRequest{Iteration: iter}).Return(txIter, nil)

	classesIter := NewClassesResponseSeq()
	client.EXPECT().RequestClasses(gomock.Any(), &syncclass.ClassesRequest{Iteration: iter}).Return(classesIter, nil)

	eventsIter := NewEventsResponseSeq()
	client.EXPECT().RequestEvents(gomock.Any(), &event.EventsRequest{Iteration: iter}).Return(eventsIter, nil)

	stateDiffsIter := NewStateDiffsResponseSeq()
	client.EXPECT().RequestStateDiffs(gomock.Any(), &state.StateDiffsRequest{Iteration: iter}).Return(stateDiffsIter, nil)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	sync.Run(ctx)

	height, err := bc.Height()
	require.NoError(t, err)
	require.Equal(t, height, uint64(0))
}

// NewBlockHeadersResponseSeq returns an iter.Seq emitting one Header message followed by a Fin message.
func NewBlockHeadersResponseSeqEmpty(t *testing.T) iter.Seq[*header.BlockHeadersResponse] {
	hdr := &header.SignedBlockHeader{
		BlockHash:        toHash(utils.HexToFelt(t, "0x40a15319f25a679938dedd6c1e2815bcf5a4f8282cb5210add954f65438fec2")),
		ParentHash:       toHash(utils.HexToFelt(t, "0x0")),
		Number:           0,
		Time:             1_650_000_000,
		SequencerAddress: toAddress(utils.HexToFelt(t, "0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")),
		StateRoot:        toHash(utils.HexToFelt(t, "0x0")),
		StateDiffCommitment: &synccommon.StateDiffCommitment{
			StateDiffLength: 0,
			Root:            toHash(utils.HexToFelt(t, "0x0")),
		},
		Transactions: &common.Patricia{
			NLeaves: 0,
			Root:    toHash(utils.HexToFelt(t, "0x0")),
		},
		Events: &common.Patricia{
			NLeaves: 0,
			Root:    toHash(utils.HexToFelt(t, "0x0")),
		},
		Receipts:               toHash(utils.HexToFelt(t, "0x0")),
		ProtocolVersion:        "0.13.4",
		L1GasPriceFri:          toUint128(utils.HexToFelt(t, "0x3")),
		L1GasPriceWei:          toUint128(utils.HexToFelt(t, "0x3")),
		L1DataGasPriceFri:      toUint128(utils.HexToFelt(t, "0x3")),
		L1DataGasPriceWei:      toUint128(utils.HexToFelt(t, "0x3")),
		L2GasPriceFri:          toUint128(utils.HexToFelt(t, "0x3")),
		L2GasPriceWei:          toUint128(utils.HexToFelt(t, "0x3")),
		L1DataAvailabilityMode: common.L1DataAvailabilityMode(0),
		Signatures:             nil, // Todo
	}

	return func(yield func(*header.BlockHeadersResponse) bool) {
		// send the header
		headerMsg := &header.BlockHeadersResponse{
			HeaderMessage: &header.BlockHeadersResponse_Header{Header: hdr},
		}
		if !yield(headerMsg) {
			return
		}
		// send the Fin frame
		finMsg := &header.BlockHeadersResponse{
			HeaderMessage: &header.BlockHeadersResponse_Fin{},
		}
		yield(finMsg)
	}
}

// NewBlockHeadersResponseSeq returns an iter.Seq emitting one Header message followed by a Fin message.
func NewBlockHeadersResponseSeq(t *testing.T, blockheader *header.SignedBlockHeader) iter.Seq[*header.BlockHeadersResponse] {
	return func(yield func(*header.BlockHeadersResponse) bool) {
		// send the header
		headerMsg := &header.BlockHeadersResponse{
			HeaderMessage: &header.BlockHeadersResponse_Header{Header: blockheader},
		}
		if !yield(headerMsg) {
			return
		}
		// send the Fin frame
		finMsg := &header.BlockHeadersResponse{
			HeaderMessage: &header.BlockHeadersResponse_Fin{},
		}
		yield(finMsg)
	}
}

// NewTransactionsResponseSeq returns an iter.Seq emitting one TransactionWithReceipt message followed by a Fin message.
func NewTransactionsResponseSeqEmpty() iter.Seq[*synctransaction.TransactionsResponse] {
	// Empty block - no txns
	return func(yield func(*synctransaction.TransactionsResponse) bool) {
		// send the Fin frame
		fin := &synctransaction.TransactionsResponse{
			TransactionMessage: &synctransaction.TransactionsResponse_Fin{},
		}
		yield(fin)
	}
}

func NewTransactionsResponseSeqFrom(
	txns []*synctransaction.TransactionInBlock,
	recs []*receipt.Receipt,
) iter.Seq[*synctransaction.TransactionsResponse] {
	return func(yield func(*synctransaction.TransactionsResponse) bool) {
		n := len(txns)
		if len(recs) < n {
			n = len(recs)
		}

		for i := 0; i < n; i++ {
			twr := &synctransaction.TransactionWithReceipt{
				Transaction: txns[i],
				Receipt:     recs[i],
			}
			resp := &synctransaction.TransactionsResponse{
				TransactionMessage: &synctransaction.TransactionsResponse_TransactionWithReceipt{
					TransactionWithReceipt: twr,
				},
			}
			if !yield(resp) {
				return
			}
		}

		// send the Fin frame
		fin := &synctransaction.TransactionsResponse{
			TransactionMessage: &synctransaction.TransactionsResponse_Fin{},
		}
		yield(fin)
	}
}

// NewEventsResponseSeq returns an iter.Seq emitting one Event message followed by a Fin message.
func NewEventsResponseSeq() iter.Seq[*event.EventsResponse] {
	// Empty block - no events
	return func(yield func(*event.EventsResponse) bool) {
		// send the Fin frame
		fin := &event.EventsResponse{
			EventMessage: &event.EventsResponse_Fin{},
		}
		yield(fin)
	}
}

// NewClassesResponseSeq returns an iter.Seq emitting one Class message followed by a Fin message.
func NewClassesResponseSeq() iter.Seq[*syncclass.ClassesResponse] {
	// Empty block - no classes
	return func(yield func(*syncclass.ClassesResponse) bool) {
		// send the Fin frame
		fin := &syncclass.ClassesResponse{
			ClassMessage: &syncclass.ClassesResponse_Fin{},
		}
		yield(fin)
	}
}

// NewStateDiffsResponseSeq returns an iter.Seq emitting one StateDiff message followed by a Fin message.
func NewStateDiffsResponseSeq() iter.Seq[*state.StateDiffsResponse] {
	// Empty block - no contract or class was altered
	return func(yield func(*state.StateDiffsResponse) bool) {
		// send the Fin frame
		fin := &state.StateDiffsResponse{
			StateDiffMessage: &state.StateDiffsResponse_Fin{},
		}
		yield(fin)
	}
}

func toHash(felt *felt.Felt) *common.Hash {
	feltBytes := felt.Bytes()
	return &common.Hash{Elements: feltBytes[:]}
}

func toAddress(felt *felt.Felt) *common.Address {
	feltBytes := felt.Bytes()
	return &common.Address{Elements: feltBytes[:]}
}

func toUint128(f *felt.Felt) *common.Uint128 {
	// bits represents value in little endian byte order
	// i.e. first is least significant byte
	bits := f.Bits()
	return &common.Uint128{
		Low:  bits[0],
		High: bits[1],
	}
}
