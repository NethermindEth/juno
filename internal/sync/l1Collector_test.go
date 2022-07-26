package sync

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/internal/sync/abi"

	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
	ethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
)

// newTestBackend creates a fake chain and returns an associated node.
func newTestBackend(t *testing.T, alloc core.GenesisAlloc, txs ...*ethtypes.Transaction) *node.Node {
	// Create node
	n, err := node.New(&node.Config{})
	if err != nil {
		t.Fatalf("can't create new node: %v", err)
	}
	// Create Ethereum Service
	config := &ethconfig.Config{
		Genesis: &core.Genesis{
			Alloc:     alloc,
			Timestamp: 9000,
			ExtraData: []byte("test genesis"),
			BaseFee:   big.NewInt(params.InitialBaseFee),
			Config:    params.AllEthashProtocolChanges,
		},
		Ethash: ethash.Config{
			PowMode: ethash.ModeFake,
		},
	}
	ethservice, err := eth.New(n, config)
	if err != nil {
		t.Fatalf("can't create new ethereum service: %v", err)
	}
	// Import the test chain.
	if err := n.Start(); err != nil {
		t.Fatalf("can't start test node: %v", err)
	}
	blocks := generateTestChain(config.Genesis, txs...)
	if _, err := ethservice.BlockChain().InsertChain(blocks[1:]); err != nil {
		t.Fatalf("can't import test blocks: %v", err)
	}
	return n
}

// generateTestChain generates a test chain from the given transactions.
func generateTestChain(genesis *core.Genesis, txs ...*ethtypes.Transaction) []*ethtypes.Block {
	db := rawdb.NewMemoryDatabase()
	generate := func(i int, g *core.BlockGen) {
		g.OffsetTime(5)
		g.SetExtra([]byte("test"))
		if i == 1 {
			// Test transactions are included in block #2.
			for _, tx := range txs {
				g.AddTx(tx)
			}
		}
	}
	gblock := genesis.ToBlock(db)
	engine := ethash.NewFaker()
	blocks, _ := core.GenerateChain(genesis.Config, gblock, engine, db, 2, generate)
	blocks = append([]*ethtypes.Block{gblock}, blocks...)
	return blocks
}

// newMockEthclient returns a mock eth client. The close methods for the
// backend and rpc client are meant to be defered immediately.
func newMockEthclient(
	t *testing.T,
	addresses []common.Address,
	txs ...*ethtypes.Transaction,
) (func() error, func(), *ethclient.Client) {
	alloc := make(core.GenesisAlloc, len(addresses))
	for _, addr := range addresses {
		alloc[addr] = core.GenesisAccount{Balance: big.NewInt(2e18)}
	}
	backend := newTestBackend(t, alloc, txs...)
	rpcClient, _ := backend.Attach()
	return backend.Close, rpcClient.Close, ethclient.NewClient(rpcClient)
}

func TestGetFactInfo(t *testing.T) {
	contractAbi, err := loadAbiOfContract(abi.StarknetAbi)
	if err != nil {
		t.Errorf("Could not finish test: failed to load contract ABI")
	}
	tests := [...]struct {
		logs              []ethtypes.Log
		abi               ethAbi.ABI
		fact              common.Hash
		latestBlockSynced uint64
		description       string
	}{
		{
			logs: []ethtypes.Log{
				{
					BlockNumber: 1,
					BlockHash:   common.Hash{1},
					TxHash:      common.Hash{2},
					Data:        []byte("067aa4a01cc374131818ab8aaaed7b7609448c922fe8956d07a9420cc5bb0bf500000000000000000000000000000000000000000000000000000000000009a5"),
				},
			},
			fact:              common.HexToHash("1"),
			latestBlockSynced: 7148728157378602549,
			description:       "basic test",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			want := &types.Fact{
				StateRoot:      new(felt.Felt).SetHex("0x3036376161346130316363333734313331383138616238616161656437623736"),
				SequenceNumber: 7148728157378602549,
				Value:          test.fact,
			}

			l := &l1Collector{
				service: service{
					logger: Logger.Named("l1Collector"),
				},
				starknetABI: contractAbi,
				facts:       types.NewDictionary(),
			}
			l.facts.Add(test.fact, want)
			res, err := l.getFactInfo(test.logs, test.fact, common.Hash{2})
			if err != nil {
				t.Errorf("Error while searching for fact: %v", err)
			} else if res.Value != want.Value || res.SequenceNumber != want.SequenceNumber || res.StateRoot.Cmp(want.StateRoot) != 0 {
				t.Errorf("Incorrect fact:\n%+v\n, want\n%+v", res, want)
			}
		})
	}
}

func TestProcessPagesHashes(t *testing.T) {
	// Instantiate pagesHashes and memoryContract
	memoryContract, err := loadAbiOfContract(abi.MemoryPagesAbi)
	if err != nil {
		t.Errorf("Could not finish test: failed to load contract ABI")
	}
	hash := common.HexToHash("0x2f8c1a2e8c9c550d1a62b8d42543f3dac973ea8d30e52dcb49d2e1c787007203")
	pagesHashes := make([][32]byte, 1)
	copy(pagesHashes[0][:], hash.Bytes())

	to := common.HexToAddress("0x96375087b2f6efc59e5e0dd5111b4d090ebfdd8b")
	r, _ := new(big.Int).SetString("a681faea68ff81d191169010888bbbe90ec3eb903e31b0572cd34f13dae281b9", 16)
	s, _ := new(big.Int).SetString("3f59b0fa5ce6cf38aff2cfeb68e7a503ceda2a72b4442c7e2844d63544383e3", 16)
	// See https://api.etherscan.io/api?module=proxy&action=eth_getTransactionByHash&txhash=0xbc78ab8a9e9a0bca7d0321a27b2c03addeae08ba81ea98b03cd3dd237eabed44
	tx := &ethtypes.DynamicFeeTx{
		ChainID:    params.AllEthashProtocolChanges.ChainID, // for the mock geth backend
		Nonce:      0,                                       // First transaction in the fake chain
		GasTipCap:  big.NewInt(1000000000),
		GasFeeCap:  big.NewInt(135000000000),
		Gas:        53896,
		To:         &to,
		Value:      big.NewInt(7165918000000000),
		Data:       common.Hex2Bytes("5578ceae0000000000000000000000000000000000000000000000000000000004ddf9d100000000000000000000000000000000000000000000000000000000000000a0075575fe6501ef399f6ae493918c4f4baf7958116ba67c45d70c40f88865835c04e3bcc23e0d0e792a8697ce7167a42aece29db8b431e44639acdcc95e8711280800000000000011000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000001a02bae43711f26ae111ab92461ec41ea93031f031b2b5bc943db0d863c77176b9066644b5899132aeedb2617607aa8aa91028780e8056eba13aaff13716275a9804ecafe9423cfac4498b51d375f7ed2d330246c6be6a7c8f46ccaad87ae5a8db069a9ba8bd284ed6f6c530f1524b0b4d9cd28d92c3e7854b1678b8c7a9f91010000000000000000000000000000000000000000000000000000000000007036d000000000000000000000000000000000000000000000000000000000000001f000000000000000000000000000000000000000000000000000000000000003f00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000037e11c9c9817ce9f3cb31ed7f00491478a7689bb8442b9ff37596ac35f4168003db9d83c9328d408e4b27f5f8e6e97ba4dc3d0f751331c273645ce39eaaf3250000000000000000000000000000000000000000000000017ffffdf653b3fc7c000000000000000000000000a350122a590fc6c8bee981a06039436fff79c02a02c77fdc97759f654a74b6ba9845219df635f53ffa877bdfc69f4dbf7028885a0000000000000000000000000000000000000000000000007ffffe47458b28cf02ac7d20744d8eaa13caa5465eef3d392f83d0e22d3c19c17bedf4f61e38978f032187f7d3db76378152d69d5ff22a5761496596ff4e91708d72d0c939d8282500000000000000000000000000000000000000006437795b80000000023a828f066364750c682c034cd551ea39126bd2f5d85396f85882d09ff5c8cdecebdc0f00081aa49c199a78e0a085fc5552cc9fa460b111a730d2e4cd37ac8dd58ba1170000000000000000000000000000000000000000501c31ad800ef931baebc3f00353716ea5c217d72631de338cba3bd27918e1b1386432f96ee0f34a0e232c8902f3ca9ce08216101aa1dc5cf0bf34f4fceb54f09ec3304263124e10b6e807eb"),
		AccessList: ethtypes.AccessList{},
		// Signature values
		V: new(big.Int),
		R: r,
		S: s,
	}
	// Get ethclient mock
	addresses := []common.Address{
		common.HexToAddress("0x4cecc7cf83d99cf6d80dd94a6917d38df9bef4e5"), // gpsVerifier
		common.HexToAddress("0x211b9e844ee92de0b2ac38760a5bb004b2637796"), // fromAddr
		common.HexToAddress("0x3eb3ef36108789c2993117cfcd8c05e879f54284"),
		common.HexToAddress("0x1Fb17006d0B4FeC8592dF10c76B48437d85A5E2f"),
	}
	finalTx := ethtypes.NewTx(tx)
	backendClose, rpcClose, ec := newMockEthclient(t, addresses, finalTx)
	defer backendClose()
	defer rpcClose()

	l := &l1Collector{
		l1client:       ec,
		memoryPageHash: types.NewDictionary(),
	}
	l.memoryPageHash.Add(hash, types.TxnHash{Hash: finalTx.Hash()})

	pages, _ := l.processPagesHashes(pagesHashes, memoryContract)

	wantPagesStrings := [][]string{
		// The value of the `values` parameter in the call to `registerContinuousMemoryPage`
		{
			"1234834334069480936175124601702828278893792771629334247197322562958869493433",
			"2894569705096436988202241476149584659847938276142739122446927701442245909144",
			"2227441395874722835147724348170629283854842796424696304210142707193993341147",
			"2987045859350866997138844199610827213625393450826172110487146920194004815888",
			"459629",
			"31",
			"63",
			"0",
			"4",
			"2",
			"0",
			"0",
			"1579684045770090003724053845854453617637241937193113478989231702540446602880",
			"1744965180054291264381444568283831268850780558100531687196082006490436268837",
			"27670113869995703420",
			"932351137691557682564734418947000262521442123818",
			"1257110731982283622291230165442656184053050769315263543083595628318768531546",
			"9223370143940946127",
			"1209386985568188099804813167263172730555815921883803981760275761521692809103",
			"1416182916062304243995279836981199463161606839130308014079194832082314995749",
			"31015564996434821448619164303",
			"2889488281167698836374527637140732432385986136984344531276120871781089926159",
			"14318659793747172997107906456918529274357618201697323149265026796065890583",
			"24792885305128787157755610096",
			"1504369732514099191184182768871423158015883187366932377192092039478771920009",
			"1335367916064879623813631211461025580911802680380024233738971738356754941931",
		},
	}
	wantPages := make([][]*big.Int, len(wantPagesStrings))
	for i, page := range wantPagesStrings {
		wantPages[i] = make([]*big.Int, len(page))
		for j, x := range page {
			wantPages[i][j], _ = new(big.Int).SetString(x, 10)
		}
	}

	assert.DeepEqual(t, pages, wantPages, cmp.Comparer(func(x, y *big.Int) bool { return x.Cmp(y) == 0 }))
}

func TestParsePages(t *testing.T) {
	pages := [][]int64{
		// First page: should be removed
		{
			0,
		},
		{
			// Deployed contracts
			4, // Number of memory cells with deployed contract info
			2, // Contract address
			3, // Contract hash
			1, // Number of constructor arguments
			2, // Constructor argument
			// State diffs
			1, // Number of diffs
			3, // Contract address
			1, // Number of updates
			3, // Key (Cairo memory address)
			4, // Value
		},
	}

	wantDiff := &types.StateDiff{
		DeployedContracts: []types.DeployedContract{
			{
				Address:             new(felt.Felt).SetHex("02"),               // Contract address
				Hash:                new(felt.Felt).SetHex("03"),               // Contract hash
				ConstructorCallData: []*felt.Felt{new(felt.Felt).SetUint64(2)}, // Constructor argument
			},
		},
		StorageDiff: types.StorageDiff{
			new(felt.Felt).SetHex("3").String(): { // Contract address
				{
					Address: new(felt.Felt).SetHex("3"), // Cairo memory address
					Value:   new(felt.Felt).SetHex("04"),
				},
			},
		},
	}

	data := make([][]*big.Int, len(pages))
	for i, page := range pages {
		data[i] = make([]*big.Int, len(page))
		for j, x := range page {
			data[i][j] = big.NewInt(x)
		}
	}

	stateDiff := parsePages(data)

	assert.DeepEqual(t, stateDiff, wantDiff, cmp.Comparer(func(x *felt.Felt, y *felt.Felt) bool { return x.CmpCompat(y) == 0 }))
}

func TestUpdateBlockOnChain(t *testing.T) {
	starknetAbi, err := loadAbiOfContract(abi.StarknetAbi)
	if err != nil {
		t.Errorf("loading contract: %x", err)
	}

	tests := [...]struct {
		data        string
		want        int64
		description string
	}{
		// See https://etherscan.io/tx/0x7e4543c64f058d15035febcc39ff52dc871c0c591b486ef5498c2fe39fc19f11#eventlog
		{
			data:        "04b954914f21ba9beab17ce2a067a7f0d177ba7876b98abbcc10fd37833f1ee20000000000000000000000000000000000000000000000000000000000000cfb",
			want:        3323,
			description: "starknet block 3323",
		},
		{
			data:        "0",
			want:        0, // should not update
			description: "malformed input",
		},
	}

	for _, test := range tests {
		// Set up a new l1collector on every test
		l := l1Collector{
			starknetABI: starknetAbi,
		}
		l.logger = Logger

		t.Run(test.description, func(t *testing.T) {
			l.updateBlockOnChain(common.Hex2Bytes(test.data))
			assert.Equal(t, l.latestBlockOnChain, test.want, fmt.Sprintf("got %d, want %d", l.latestBlockOnChain, test.want))
		})
	}
}

func TestRemoveFactTree(t *testing.T) {
	tests := [...]struct {
		factToRemove     *types.Fact
		gpsVerifier      map[common.Hash]types.PagesHash
		pageHashRegistry map[common.Hash]types.TxnHash
		facts            map[string]types.Fact
		description      string
	}{
		{
			factToRemove: &types.Fact{
				Value:          common.HexToHash("0x0"),
				SequenceNumber: 0,
			},
			gpsVerifier: map[common.Hash]types.PagesHash{
				common.HexToHash("0x0"): {Bytes: [][32]byte{{0}}},
			},
			pageHashRegistry: map[common.Hash]types.TxnHash{
				common.HexToHash("0x0"): {},
			},
			facts: map[string]types.Fact{
				"0": {},
			},
			description: "simple sanity test",
		},
	}

	for _, test := range tests {
		// Set up a new l1collector on every test
		facts := types.NewDictionary()
		for k, v := range test.facts {
			facts.Add(k, v)
		}
		gpsVerifier := types.NewDictionary()
		for k, v := range test.gpsVerifier {
			gpsVerifier.Add(k, v)
		}
		pageHashRegistry := types.NewDictionary()
		for k, v := range test.pageHashRegistry {
			pageHashRegistry.Add(k, v)
		}

		l := l1Collector{
			facts:          facts,
			gpsVerifier:    gpsVerifier,
			memoryPageHash: pageHashRegistry,
		}
		l.logger = Logger

		t.Run(test.description, func(t *testing.T) {
			l.removeFactTree(test.factToRemove)
			assert.Equal(t, l.facts.Size(), 0, "facts dictionary not empty")
			assert.Equal(t, l.gpsVerifier.Size(), 0, "gpsVerifier dictionary not empty")
			assert.Equal(t, l.memoryPageHash.Size(), 0, "memoryPageHash dictionary not empty")
		})
	}
}
