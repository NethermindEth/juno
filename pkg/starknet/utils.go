package starknet

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"io/ioutil"
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
