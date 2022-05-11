package starknet

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"io/ioutil"
	"math/big"
	"strings"
)

// newTrie returns a new Trie
func newTrie(database *db.Databaser, prefix string) trie.Trie {
	store := db.NewKeyValueStore(database, prefix)
	return trie.New(store, 251)
}

// loadContractInfo loads a contract ABI and set the events' thar later we are going yo use
func loadContractInfo(contractAddress, abiPath, logName string, contracts map[common.Address]ContractInfo) error {
	contractAddressHash := common.HexToAddress(contractAddress)
	contractFromAbi, err := loadAbiOfContract(abiPath)
	if err != nil {
		return err
	}
	contracts[contractAddressHash] = ContractInfo{
		contract:  contractFromAbi,
		eventName: logName,
	}
	return nil
}

// loadAbiOfContract loads the ABI of the contract from the
func loadAbiOfContract(abiPath string) (abi.ABI, error) {
	log.Default.With("ContractInfo", abiPath).Info("Loading contract")
	b, err := ioutil.ReadFile(abiPath)
	if err != nil {
		return abi.ABI{}, err
	}
	contractAbi, err := abi.JSON(strings.NewReader(string(b)))
	if err != nil {
		return abi.ABI{}, err
	}
	return contractAbi, nil
}

// contractState define the function that calculates the values stored in the
// leaf of the Merkle Patricia Tree that represent the State in StarkNet
func contractState(contractHash, storageRoot *big.Int) *big.Int {
	// Is defined as:
	// h(h(h(contract_hash, storage_root), 0), 0).
	val, err := pedersen.Digest(contractHash, storageRoot)
	if err != nil {
		log.Default.With("Error", err, "ContractInfo Hash", contractHash.String(),
			"Storage Commitment", storageRoot.String(),
			"Function", "h(contract_hash, storage_root)").
			Panic("Couldn't calculate the digest")
	}
	val, err = pedersen.Digest(val, big.NewInt(0))
	if err != nil {
		log.Default.With("Error", err, "ContractInfo Hash", contractHash.String(),
			"Storage Commitment", storageRoot.String(),
			"Function", "h(h(contract_hash, storage_root), 0)",
			"First Hash", val.String()).
			Panic("Couldn't calculate the digest")
	}
	val, err = pedersen.Digest(val, big.NewInt(0))
	if err != nil {
		log.Default.With("Error", err, "ContractInfo Hash", contractHash.String(),
			"Storage Commitment", storageRoot.String(),
			"Function", "h(h(h(contract_hash, storage_root), 0), 0)",
			"Second Hash", val.String()).
			Panic("Couldn't calculate the digest")
	}
	return val
}

// removeOx remove the initial zeros and x at the beginning of the string
func remove0x(s string) string {
	answer := ""
	found := false
	for _, char := range s {
		found = found || (char != '0' && char != 'x')
		if found {
			answer = answer + string(char)
		}
	}
	if len(answer) == 0 {
		return "0"
	}
	return answer
}

// stateUpdateResponseToStateDiff convert the input feeder.StateUpdateResponse to StateDiff
func stateUpdateResponseToStateDiff(update feeder.StateUpdateResponse) StateDiff {
	var stateDiff StateDiff
	stateDiff.DeployedContracts = make([]DeployedContract, 0)
	stateDiff.StorageDiffs = make(map[string][]KV)
	for _, v := range update.StateDiff.DeployedContracts {
		deployedContract := DeployedContract{
			Address:      v.Address,
			ContractHash: v.ContractHash,
		}
		stateDiff.DeployedContracts = append(stateDiff.DeployedContracts, deployedContract)
	}
	for addressDiff, keyVals := range update.StateDiff.StorageDiffs {
		address := addressDiff
		kvs := make([]KV, 0)
		for _, kv := range keyVals {
			kvs = append(kvs, KV{
				Key:   kv.Key,
				Value: kv.Value,
			})
		}
		stateDiff.StorageDiffs[address] = kvs
	}

	return stateDiff
}

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func getGpsVerifierContractAddress(ethereumClient *ethclient.Client) string {
	id, err := ethereumClient.ChainID(context.Background())
	if err != nil {
		return "0xa739B175325cCA7b71fcB51C3032935Ef7Ac338F"
	}
	if id.Int64() == 1 {
		return "0xa739B175325cCA7b71fcB51C3032935Ef7Ac338F"
	}
	return "0x5EF3C980Bf970FcE5BbC217835743ea9f0388f4F"
}

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func getMemoryPagesContractAddress(ethereumClient *ethclient.Client) string {
	id, err := ethereumClient.ChainID(context.Background())
	if err != nil {
		return "0x96375087b2F6eFc59e5e0dd5111B4d090EBFDD8B"
	}
	if id.Int64() == 1 {
		return "0x96375087b2F6eFc59e5e0dd5111B4d090EBFDD8B"
	}
	return "0x743789ff2fF82Bfb907009C9911a7dA636D34FA7"
}

func initialBlockForStarknetContract(ethereumClient *ethclient.Client) int64 {
	id, err := ethereumClient.ChainID(context.Background())
	if err != nil {
		return 0
	}
	if id.Int64() == 1 {
		return blockOfStarknetDeploymentContractMainnet
	}
	return blockOfStarknetDeploymentContractGoerli
}

func latestBlockQueried(database *db.Databaser) (int64, error) {
	blockNumber, err := (*database).Get([]byte(latestBlockSynced))
	if err != nil {
		return 0, err
	}
	if blockNumber == nil {
		return 0, nil
	}
	var ret uint64
	buf := bytes.NewBuffer(blockNumber)
	err = binary.Read(buf, binary.BigEndian, &ret)
	if err != nil {
		return 0, err
	}
	return int64(ret), nil
}

func updateLatestBlockQueried(database *db.Databaser, block int64) error {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(block))
	err := (*database).Put([]byte(latestBlockSynced), b)
	if err != nil {
		log.Default.With("Block", block, "Key", latestBlockSynced).
			Info("Couldn't store the latest synced block")
		return err
	}
	return nil
}
