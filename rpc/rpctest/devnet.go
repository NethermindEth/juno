package rpctest

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/sequencer"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/stretchr/testify/require"
)

const (
	sequencerAddress = 0x5e9
	l1HandlerAddress = 0x11
)

var (
	// SequencerAddress is the address of the devnet sequencer, which gets the transaction fees.
	SequencerAddress = felt.FromUint64[felt.Address](sequencerAddress)
	// L1HandlerAddress is the address of a devnet contract with the L1 handler
	// deposit(from_address, user, amount).
	L1HandlerAddress = felt.FromUint64[felt.Address](l1HandlerAddress)
	// L1HandlerClassHash is the class of the contract at L1HandlerAddress: a Cairo 0 class from
	// the Sepolia fixtures, with no constructor.
	L1HandlerClassHash = felt.UnsafeFromString[felt.ClassHash](
		"0x28d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43",
	)
)

const mempoolLimit = 1024

// Devnet is a Node on a copy of the genesis chain. Only MakeBlock makes a block.
type Devnet struct {
	*Node
	sequencer *sequencer.Sequencer
	network   *networks.Network
	accounts  []genesis.Account
}

type genesisChain struct {
	database *memory.Database
	accounts []genesis.Account
}

// repoRoot returns the directory of the first go.mod in or above the working directory. A test runs
// in its package directory. With -trimpath, runtime.Caller gives no file system path.
func repoRoot() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := workDir; ; dir = filepath.Dir(dir) {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if dir == filepath.Dir(dir) {
			return "", fmt.Errorf("no go.mod in %s or above", workDir)
		}
	}
}

// genesisBase builds the genesis chain one time for each test binary: the genesis file of the
// repository, plus the L1 handler contract.
var genesisBase = sync.OnceValues(func() (genesisChain, error) {
	root, err := repoRoot()
	if err != nil {
		return genesisChain{}, err
	}
	config, err := genesis.Read(filepath.Join(root, "genesis", "genesis_prefund_accounts.json"))
	if err != nil {
		return genesisChain{}, err
	}
	for i := range config.Classes {
		config.Classes[i] = filepath.Join(root, config.Classes[i])
	}
	config.Classes = append(config.Classes, filepath.Join(
		root, "clients", "feeder", "testdata", "sepolia", "class", L1HandlerClassHash.String()+".json",
	))
	config.Contracts[L1HandlerAddress] = genesis.GenesisContractData{ClassHash: L1HandlerClassHash}

	network := &networks.Sepolia
	logger := log.NewNopZapLogger()
	diff, classes, err := genesis.GenesisStateDiff(
		context.Background(),
		config,
		vm.New(chainInfo(network), false, logger),
		network,
		vm.DefaultMaxSteps,
		vm.DefaultMaxGas,
		statetestutils.UseNewState(),
		compiler.NewUnsafe(),
	)
	if err != nil {
		return genesisChain{}, err
	}

	database := memory.New()
	chain := blockchain.New(database, network, blockchain.WithNewState(statetestutils.UseNewState()))
	if err := chain.StoreGenesis(&diff, classes); err != nil {
		return genesisChain{}, err
	}
	return genesisChain{database: database, accounts: config.BootstrapAccounts}, nil
})

// NewDevnet returns a Devnet in sequencer mode, with fees and validation on. It has the genesis
// block, block 1 with gas prices and an empty pre_confirmed block.
func NewDevnet(t testing.TB) *Devnet {
	t.Helper()
	base, err := genesisBase()
	require.NoError(t, err)

	network := &networks.Sepolia
	logger := log.NewNopZapLogger()
	database := base.database.Copy()
	chain := blockchain.New(database, network, blockchain.WithNewState(statetestutils.UseNewState()))

	txPool := mempool.New(database, chain, mempoolLimit, logger)
	t.Cleanup(txPool.Close)

	nodeVM := vm.New(chainInfo(network), false, logger)
	executor := builder.NewExecutor(chain, nodeVM, logger, false, false)
	blockBuilder := builder.New(chain, executor)
	signingKey, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	seq := sequencer.New(&blockBuilder, txPool, &SequencerAddress, signingKey, time.Second, logger)

	handler := newHandler(chain, &seq, network, logger).WithMempool(txPool)
	devnet := &Devnet{
		Node:      newNode(t, chain, handler),
		sequencer: &seq,
		network:   network,
		accounts:  base.accounts,
	}
	devnet.MakeBlock(t)
	return devnet
}

// MakeBlock makes a block from the transactions in the mempool and returns its header.
func (d *Devnet) MakeBlock(t testing.TB) *core.Header {
	t.Helper()
	header, err := d.sequencer.RunOnce()
	require.NoError(t, err)
	return header
}

// Submit adds the transactions to the mempool through the v0.10 server.
func (d *Devnet) Submit(t testing.TB, txns ...Txn) {
	t.Helper()
	for _, txn := range txns {
		var method, param string
		switch txn.fields["type"] {
		case "INVOKE":
			method, param = "starknet_addInvokeTransaction", "invoke_transaction"
		case "DEPLOY_ACCOUNT":
			method, param = "starknet_addDeployAccountTransaction", "deploy_account_transaction"
		default:
			require.FailNow(t, "cannot submit transaction type", txn.fields["type"])
		}
		result := d.Result(t, Version0_10, method, `{"`+param+`":`+txn.String()+`}`)
		var added struct {
			TransactionHash felt.TransactionHash `json:"transaction_hash"`
		}
		require.NoError(t, json.Unmarshal(result, &added))
		require.Equal(t, txn.Hash, added.TransactionHash)
	}
}
