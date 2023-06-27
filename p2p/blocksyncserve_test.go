package p2p

import (
	"bytes"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/p2pproto"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"io"
	"strconv"
	"testing"
)

func TestBlockSyncServer_HandleGetBlockHeader(t *testing.T) {
	blockProvider := &memBlockProvider{}
	c := &converter{}
	blocksyncserver := &blockSyncServer{
		blockchain: blockProvider,
		converter:  c,
		log:        utils.NewNopZapLogger(),
	}

	for i := 0; i < 5; i++ {
		if i == 0 {
			blockProvider.blocks = append(blockProvider.blocks, blockGen(nil))
		} else {
			blockProvider.blocks = append(blockProvider.blocks, blockGen(blockProvider.blocks[len(blockProvider.blocks)-1]))
		}
	}

	tests := []struct {
		name     string
		request  *p2pproto.GetBlockHeaders
		expected *p2pproto.BlockHeaders
	}{
		{
			name: "by block number",
			request: &p2pproto.GetBlockHeaders{
				StartBlock: &p2pproto.GetBlockHeaders_BlockNumber{
					BlockNumber: 2,
				},
				Count: 2,
			},
			expected: &p2pproto.BlockHeaders{
				Headers: []*p2pproto.BlockHeader{
					convertToProtoHeader(c, blockProvider.blocks[2]),
					convertToProtoHeader(c, blockProvider.blocks[3]),
				},
			},
		},
		{
			name: "by block number reversed",
			request: &p2pproto.GetBlockHeaders{
				StartBlock: &p2pproto.GetBlockHeaders_BlockNumber{
					BlockNumber: 2,
				},
				Direction: p2pproto.Direction_BACKWARD,
				Count:     2,
			},
			expected: &p2pproto.BlockHeaders{
				Headers: []*p2pproto.BlockHeader{
					convertToProtoHeader(c, blockProvider.blocks[2]),
					convertToProtoHeader(c, blockProvider.blocks[1]),
				},
			},
		},
		{
			name: "by hash",
			request: &p2pproto.GetBlockHeaders{
				StartBlock: &p2pproto.GetBlockHeaders_BlockHash{
					BlockHash: feltToFieldElement(blockProvider.blocks[2].Hash),
				},
				Count: 1,
			},
			expected: &p2pproto.BlockHeaders{
				Headers: []*p2pproto.BlockHeader{
					convertToProtoHeader(c, blockProvider.blocks[2]),
				},
			},
		},
		{
			name: "unknown block number",
			request: &p2pproto.GetBlockHeaders{
				StartBlock: &p2pproto.GetBlockHeaders_BlockNumber{
					BlockNumber: 9999,
				},
				Count: 2,
			},
			expected: &p2pproto.BlockHeaders{
				Headers: nil,
			},
		},
		{
			name: "unknown block hash",
			request: &p2pproto.GetBlockHeaders{
				StartBlock: &p2pproto.GetBlockHeaders_BlockHash{
					BlockHash: feltToFieldElement(&felt.Zero),
				},
				Count: 2,
			},
			expected: &p2pproto.BlockHeaders{
				Headers: nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := &p2pproto.Request{
				Request: &p2pproto.Request_GetBlockHeaders{GetBlockHeaders: test.request},
			}

			err, result := simulateStream(t, request, blocksyncserver)

			assert.Nil(t, err)
			assert.Equal(t, test.expected, result.GetBlockHeaders())
		})
	}
}

func TestBlockSyncServer_HandleGetBlockBody(t *testing.T) {
	blockProvider := &memBlockProvider{}
	c := &converter{}
	blocksyncserver := &blockSyncServer{
		blockchain: blockProvider,
		converter:  c,
		log:        utils.NewNopZapLogger(),
	}

	for i := 0; i < 5; i++ {
		if i == 0 {
			blockProvider.blocks = append(blockProvider.blocks, blockGen(nil))
		} else {
			blockProvider.blocks = append(blockProvider.blocks, blockGen(blockProvider.blocks[len(blockProvider.blocks)-1]))
		}
	}

	tests := []struct {
		name     string
		request  *p2pproto.GetBlockBodies
		expected *p2pproto.BlockBodies
	}{
		{
			name: "by hash",
			request: &p2pproto.GetBlockBodies{
				StartBlock: feltToFieldElement(blockProvider.blocks[2].Hash),
				Count:      1,
			},
			expected: &p2pproto.BlockBodies{
				BlockBodies: []*p2pproto.BlockBody{
					convertToProtoBody(c, blockProvider.blocks[2]),
				},
			},
		},
		{
			name: "by hash multiple",
			request: &p2pproto.GetBlockBodies{
				StartBlock: feltToFieldElement(blockProvider.blocks[2].Hash),
				Count:      2,
			},
			expected: &p2pproto.BlockBodies{
				BlockBodies: []*p2pproto.BlockBody{
					convertToProtoBody(c, blockProvider.blocks[2]),
					convertToProtoBody(c, blockProvider.blocks[3]),
				},
			},
		},
		{
			name: "by hash multiple reversed",
			request: &p2pproto.GetBlockBodies{
				StartBlock: feltToFieldElement(blockProvider.blocks[2].Hash),
				Direction:  p2pproto.Direction_BACKWARD,
				Count:      2,
			},
			expected: &p2pproto.BlockBodies{
				BlockBodies: []*p2pproto.BlockBody{
					convertToProtoBody(c, blockProvider.blocks[2]),
					convertToProtoBody(c, blockProvider.blocks[1]),
				},
			},
		},
		{
			name: "unknown block hash",
			request: &p2pproto.GetBlockBodies{
				StartBlock: feltToFieldElement(&felt.Zero),
				Count:      2,
			},
			expected: &p2pproto.BlockBodies{
				BlockBodies: nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := &p2pproto.Request{
				Request: &p2pproto.Request_GetBlockBodies{GetBlockBodies: test.request},
			}

			err, result := simulateStream(t, request, blocksyncserver)

			assert.Nil(t, err)
			assert.Equal(t, test.expected, result.GetBlockBodies())
		})
	}
}

func TestBlockSyncServer_GetStateUpdate(t *testing.T) {
	blockProvider := &memBlockProvider{}
	c := &converter{}
	blocksyncserver := &blockSyncServer{
		blockchain: blockProvider,
		converter:  c,
		log:        utils.NewNopZapLogger(),
	}

	blockProvider.stateUpdates = map[uint64]*core.StateUpdate{}
	for i := 0; i < 5; i++ {
		if i == 0 {
			blockProvider.blocks = append(blockProvider.blocks, blockGen(nil))
		} else {
			blockProvider.blocks = append(blockProvider.blocks, blockGen(blockProvider.blocks[len(blockProvider.blocks)-1]))
		}

		blockProvider.stateUpdates[uint64(i)] = genStateUpdate(uint64(i))
	}

	tests := []struct {
		name     string
		request  *p2pproto.GetStateDiffs
		expected *p2pproto.StateDiffs
	}{
		{
			name: "smoke",
			request: &p2pproto.GetStateDiffs{
				StartBlock: feltToFieldElement(blockProvider.blocks[2].Hash),
				Count:      1,
			},
			expected: &p2pproto.StateDiffs{
				BlockStateUpdates: []*p2pproto.StateDiffs_BlockStateUpdateWithHash{
					coreStateUpdateToProtobufStateUpdate(blockProvider.stateUpdates[2]),
				},
			},
		},
		{
			name: "multiple",
			request: &p2pproto.GetStateDiffs{
				StartBlock: feltToFieldElement(blockProvider.blocks[2].Hash),
				Count:      2,
			},
			expected: &p2pproto.StateDiffs{
				BlockStateUpdates: []*p2pproto.StateDiffs_BlockStateUpdateWithHash{
					coreStateUpdateToProtobufStateUpdate(blockProvider.stateUpdates[2]),
					coreStateUpdateToProtobufStateUpdate(blockProvider.stateUpdates[3]),
				},
			},
		},
		{
			name: "reverse",
			request: &p2pproto.GetStateDiffs{
				StartBlock: feltToFieldElement(blockProvider.blocks[2].Hash),
				Count:      2,
				Direction:  p2pproto.Direction_BACKWARD,
			},
			expected: &p2pproto.StateDiffs{
				BlockStateUpdates: []*p2pproto.StateDiffs_BlockStateUpdateWithHash{
					coreStateUpdateToProtobufStateUpdate(blockProvider.stateUpdates[2]),
					coreStateUpdateToProtobufStateUpdate(blockProvider.stateUpdates[1]),
				},
			},
		},
		{
			name: "unknown blockhash",
			request: &p2pproto.GetStateDiffs{
				StartBlock: feltToFieldElement(&felt.Zero),
				Count:      1,
			},
			expected: &p2pproto.StateDiffs{
				BlockStateUpdates: nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := &p2pproto.Request{
				Request: &p2pproto.Request_GetStateDiffs{GetStateDiffs: test.request},
			}

			err, result := simulateStream(t, request, blocksyncserver)

			assert.Nil(t, err)
			assert.Equal(t, test.expected, result.GetStateDiffs())
		})
	}
}

func TestBlockSyncServer_GetStatus(t *testing.T) {
	blockProvider := &memBlockProvider{}
	c := &converter{}
	blocksyncserver := &blockSyncServer{
		blockchain: blockProvider,
		converter:  c,
		log:        utils.NewNopZapLogger(),
	}

	blockProvider.head = blockGen(nil)

	tests := []struct {
		name     string
		request  *p2pproto.Status
		expected *p2pproto.Status
	}{
		{
			name: "smoke",
			request: &p2pproto.Status{
				Height:  0,
				Hash:    feltToFieldElement(&felt.Zero),
				ChainId: feltToFieldElement(&felt.Zero),
			},
			expected: &p2pproto.Status{
				Height:  0,
				Hash:    feltToFieldElement(blockProvider.head.Hash),
				ChainId: feltToFieldElement(&felt.Zero),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := &p2pproto.Request{
				Request: &p2pproto.Request_Status{Status: test.request},
			}

			err, result := simulateStream(t, request, blocksyncserver)

			assert.Nil(t, err)
			assert.Equal(t, test.expected, result.GetStatus())
		})
	}
}

func genStateUpdate(blockNum uint64) *core.StateUpdate {
	stateUpdate := &core.StateUpdate{
		StateDiff: &core.StateDiff{
			Nonces: map[felt.Felt]*felt.Felt{},
		},
	}

	// I just need something
	key, err := crypto.StarknetKeccak([]byte(strconv.Itoa(int(blockNum))))
	if err != nil {
		panic(err)
	}

	stateUpdate.StateDiff.Nonces[*key] = key

	return stateUpdate
}

func convertToProtoHeader(c *converter, block *core.Block) *p2pproto.BlockHeader {
	header, err := c.coreBlockToProtobufHeader(block)
	if err != nil {
		panic(err)
	}

	return header
}

func convertToProtoBody(c *converter, block *core.Block) *p2pproto.BlockBody {
	body, err := c.coreBlockToProtobufBody(block)
	if err != nil {
		panic(err)
	}

	if len(body.Transactions) == 0 {
		body.Transactions = nil
	}
	if len(body.Receipts) == 0 {
		body.Receipts = nil
	}

	return body
}

type testReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (t *testReadWriteCloser) Close() error {
	return nil
}

var _ io.ReadWriteCloser = &testReadWriteCloser{}

func simulateStream(t *testing.T, request *p2pproto.Request, blocksyncserver *blockSyncServer) (error, *p2pproto.Response) {
	requestBuffer := &bytes.Buffer{}
	err := writeCompressedProtobuf(requestBuffer, request)
	if err != nil {
		t.Fatalf("error writing request to buffer %s", err)
	}

	outBuffer := &bytes.Buffer{}

	err = blocksyncserver.DoHandleBlockSyncStream(&testReadWriteCloser{
		Reader: requestBuffer,
		Writer: outBuffer,
	})
	if err != nil {
		t.Fatalf("error handlign block stream %s", err)
	}

	result := &p2pproto.Response{}
	err = readCompressedProtobuf(outBuffer, result)
	if err != nil {
		t.Fatalf("error handlign block stream %s", err)
	}
	return err, result
}

type memBlockProvider struct {
	head         *core.Block
	blocks       []*core.Block
	stateUpdates map[uint64]*core.StateUpdate
}

func (m *memBlockProvider) BlockByHash(hash *felt.Felt) (*core.Block, error) {
	for _, block := range m.blocks {
		if block.Hash.Equal(hash) {
			return block, nil
		}
	}

	return nil, db.ErrKeyNotFound
}

func (m *memBlockProvider) BlockByNumber(number uint64) (*core.Block, error) {
	for _, block := range m.blocks {
		if block.Number == number {
			return block, nil
		}
	}

	return nil, db.ErrKeyNotFound
}

func (m *memBlockProvider) BlockHeaderByHash(hash *felt.Felt) (*core.Header, error) {
	for _, block := range m.blocks {
		if block.Hash.Equal(hash) {
			return block.Header, nil
		}
	}

	return nil, db.ErrKeyNotFound
}

func (m *memBlockProvider) StateUpdateByNumber(blocknumber uint64) (*core.StateUpdate, error) {
	if update, ok := m.stateUpdates[blocknumber]; ok {
		return update, nil
	}
	return nil, db.ErrKeyNotFound
}

func (m *memBlockProvider) Head() (*core.Block, error) {
	return m.head, nil
}

var _ BlockProvider = &memBlockProvider{}

func blockGen(parent *core.Block) *core.Block {
	var block *core.Block
	if parent == nil {
		block = &core.Block{
			Header: &core.Header{
				Number:          0,
				ParentHash:      &felt.Zero,
				GlobalStateRoot: &felt.Zero,
			},
		}
	} else {
		block = &core.Block{
			Header: &core.Header{
				Number:          parent.Number + 1,
				ParentHash:      parent.Hash,
				GlobalStateRoot: &felt.Zero,
			},
		}
	}

	var err error
	block.Hash, err = core.BlockHash(block, utils.MAINNET, nil)
	if err != nil {
		panic(err)
	}

	return block
}
