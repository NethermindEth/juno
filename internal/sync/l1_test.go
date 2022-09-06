package sync

import (
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"os"

	"github.com/NethermindEth/juno/internal/sync/contracts"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
)

type chainConfig struct {
	operatorKey     *ecdsa.PrivateKey
	operatorAddress common.Address
}

func newChainConfig(operatorKey *ecdsa.PrivateKey) *chainConfig {
	return &chainConfig{
		operatorKey:     operatorKey,
		operatorAddress: crypto.PubkeyToAddress(operatorKey.PublicKey),
	}
}

type backend struct {
	db        ethdb.Database
	simulated *backends.SimulatedBackend
	config    *chainConfig
}

func newBackend(config *chainConfig) *backend {
	max := new(big.Int).SetUint64(math.MaxUint64)
	balance := new(big.Int).Mul(max, max)

	alloc := core.GenesisAlloc{
		config.operatorAddress: {
			Balance: balance,
		},
	}
	database := rawdb.NewMemoryDatabase()
	return &backend{
		config:    config,
		db:        database,
		simulated: backends.NewSimulatedBackendWithDatabase(database, alloc, 2e15),
	}
}

func (b *backend) makeChain(generate func(int, *core.BlockGen)) error {
	chain := b.simulated.Blockchain()
	blocks, receipts := core.GenerateChain(chain.Config(), chain.Genesis(), chain.Engine(), b.db, 2, generate)

	if failedBlock, err := chain.InsertChain(blocks); err != nil {
		return fmt.Errorf("block %d: failed to insert block: %w", failedBlock, err)
	}
	if failedBlock, err := chain.InsertReceiptChain(blocks, receipts, 0); err != nil {
		return fmt.Errorf("block %d: failed to insert transactions and receipts: %w", failedBlock, err)
	}

	return nil
}

func hexesToAddresses(hexes []string) []common.Address {
	addresses := make([]common.Address, len(hexes))
	for i, hex := range hexes {
		addresses[i] = common.HexToAddress(hex)
	}
	return addresses
}

// Returns: deployTxs, cpuVerifierAddress, error
func initCpuVerifierContract(opts *bind.TransactOpts, backend *backends.SimulatedBackend, memoryPageFactRegistryAddress common.Address) ([]*types.Transaction, *common.Address, error) {
	var err error
	deployTxs := make([]*types.Transaction, 25)
	auxPolynomialsAddresses := make([]common.Address, 7)

	if auxPolynomialsAddresses[0], deployTxs[0], _, err = contracts.DeployCpuConstraintPoly(opts, backend); err != nil {
		return nil, nil, err
	}
	if auxPolynomialsAddresses[1], deployTxs[1], _, err = contracts.DeployPedersenHashPointsXColumn(opts, backend); err != nil {
		return nil, nil, err
	}
	if auxPolynomialsAddresses[2], deployTxs[2], _, err = contracts.DeployPedersenHashPointsYColumn(opts, backend); err != nil {
		return nil, nil, err
	}
	if auxPolynomialsAddresses[3], deployTxs[3], _, err = contracts.DeployEcdsaPointsXColumn(opts, backend); err != nil {
		return nil, nil, err
	}
	if auxPolynomialsAddresses[4], deployTxs[4], _, err = contracts.DeployEcdsaPointsYColumn(opts, backend); err != nil {
		return nil, nil, err
	}

	var merkleStatementAddress common.Address
	merkleStatementAddress, deployTxs[5], _, err = contracts.DeployMerkleStatement(opts, backend)

	var cpuOodsAddress common.Address
	cpuOodsAddress, deployTxs[6], _, err = contracts.DeployCpuOods(opts, backend)

	var friStatementAddress common.Address
	friStatementAddress, deployTxs[7], _, err = contracts.DeployFriStatement(opts, backend)

	var cpuVerifierContractAddress common.Address
	cpuVerifierContractAddress, deployTxs[8], _, err = contracts.DeployCairoVerifier(opts, backend, auxPolynomialsAddresses, cpuOodsAddress, memoryPageFactRegistryAddress, merkleStatementAddress, friStatementAddress, big.NewInt(80), new(big.Int))
	if err != nil {
		return nil, nil, err
	}
	return deployTxs, &cpuVerifierContractAddress, nil
}

func generator(b *backend) (*L1ChainConfig, func(int, *core.BlockGen), error) {
	opts := &bind.TransactOpts{
		From: b.config.operatorAddress,
		Signer: func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			return types.SignTx(tx, types.LatestSigner(b.simulated.Blockchain().Config()), b.config.operatorKey)
		},
	}
	deployTxs := make([]*types.Transaction, 0)

	memoryPageFactRegistryAddress, memoryPageFactRegistryDeployTx, memoryPageFactRegistry, err := contracts.DeployMemoryPageFactRegistry(opts, b.simulated)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize contract: %w", err)
	}
	deployTxs = append(deployTxs, memoryPageFactRegistryDeployTx)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize contract: %w", err)
	}
	numCpuVerifiers := 7
	cpuVerifierAddresses := make([]common.Address, 0)
	for i := 0; i < numCpuVerifiers; i++ {
		deployTx, address, err := initCpuVerifierContract(opts, b.simulated, memoryPageFactRegistryAddress)
		if err != nil {
			return nil, nil, err
		}
		cpuVerifierAddresses = append(cpuVerifierAddresses, *address)
		deployTxs = append(deployTxs, deployTx...)
	}

	cairoBootLoaderProgramAddress, cairoBootLoaderProgramDeployTx, _, err := contracts.DeployCairoBootloaderProgram(opts, b.simulated)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize contract: %w", err)
	}
	deployTxs = append(deployTxs, cairoBootLoaderProgramDeployTx)

	gpsStatementVerifierAddress, gpsStatementVerifierDeployTx, gpsStatementVerifier, err := contracts.DeployGpsStatementVerifier(opts, b.simulated, cairoBootLoaderProgramAddress, memoryPageFactRegistryAddress, cpuVerifierAddresses)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize contract: %w", err)
	}
	deployTxs = append(deployTxs, gpsStatementVerifierDeployTx)

	starknetAddress, starknetDeployTx, starknet, err := contracts.DeployStarknet(opts, b.simulated)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize contract: %w", err)
	}
	deployTxs = append(deployTxs, starknetDeployTx)

	l1Config := &L1ChainConfig{
		starknet: ContractConfig{
			address:         starknetAddress,
			deploymentBlock: 1,
		},
		gpsStatementVerifier: ContractConfig{
			address:         gpsStatementVerifierAddress,
			deploymentBlock: 1,
		},
		memoryPageFactRegistry: ContractConfig{
			address:         memoryPageFactRegistryAddress,
			deploymentBlock: 1,
		},
	}
	return l1Config, func(blockNum int, gen *core.BlockGen) {
		switch blockNum {
		case 1:
			for _, tx := range deployTxs {
				gen.AddTx(tx)
			}
		case 2:
			tx, err := memoryPageFactRegistry.MemoryPageFactRegistryTransactor.RegisterContinuousMemoryPage()
			if err != nil {
				panic(fmt.Errorf("failed registerContinuousMemoryPage transaction to memory page fact registry: %w", err))
			}
			gen.AddTx(tx)
		case 3:
			tx, err := gpsStatementVerifier.GpsStatementVerifierTransactor.VerifyProofAndRegister()
			if err != nil {
				panic(fmt.Errorf("failed verifyProofAndRegister transaction to gps statement verifier: %w", err))
			}
			gen.AddTx(tx)
		case 4:
			tx, err := starknet.StarknetTransactor.UpdateState()
			if err != nil {
				panic(fmt.Errorf("failed verifyProofAndRegister transaction to gps statement verifier: %w", err))
			}
			gen.AddTx(tx)
		}
	}, nil
}

func setupBackend() (*backend, error) {
	operatorKey, err := crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	if err != nil {
		return nil, fmt.Errorf("failed to create operator key: %w", err)
	}
	config := newChainConfig(operatorKey)
	backend := newBackend(config)

	return backend, nil
}

func main() {
	backend, err := setupBackend()
	if err != nil {
		exit(fmt.Errorf("failed to set up backend: %w", err))
	}
	_, generate, err := generator(backend)
	if err != nil {
		exit(fmt.Errorf("failed to initialize generator: %w", err))
	}
	if err := backend.makeChain(generate); err != nil {
		exit(fmt.Errorf("failed to make chain: %w", err))
	}
}

func exit(err error) {
	fmt.Fprint(os.Stderr, err.Error())
	os.Exit(1)
}
