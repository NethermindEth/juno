// Package ethereum contains all the functions related to Ethereum State and Synchronization
// with Layer 1
package ethereum

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/feeder_gateway"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"io/ioutil"
	"math/big"
	"strings"
)

const latestBlockSynced = "latestBlockSynced"
const blockOfStarknetDeploymentContractMainnet = 13627224
const blockOfStarknetDeploymentContractGoerli = 5853128

// Synchronizer represents the base struct for Ethereum Synchronization
type Synchronizer struct {
	ethereumClient      *ethclient.Client
	feederGatewayClient *feeder_gateway.Client
	db                  *db.Databaser
}

// NewSynchronizer creates a new Synchronizer
func NewSynchronizer(db *db.Databaser) *Synchronizer {
	client, err := ethclient.Dial(config.Runtime.Ethereum.Node)
	if err != nil {
		log.Default.With("Error", err).Fatal("Unable to connect to Ethereum Client")
	}
	feeder := feeder_gateway.NewClient(config.Runtime.Starknet.FeederGateway, "/feeder_gateway", nil)
	return &Synchronizer{
		ethereumClient:      client,
		feederGatewayClient: feeder,
		db:                  db,
	}
}

func (s Synchronizer) initialBlockForStarknetContract() int64 {
	id, err := s.ethereumClient.ChainID(context.Background())
	if err != nil {
		return 0
	}
	if id.Int64() == 1 {
		return blockOfStarknetDeploymentContractMainnet
	}
	return blockOfStarknetDeploymentContractGoerli
}

func (s Synchronizer) latestBlockQueried() (int64, error) {
	get, err := (*s.db).Get([]byte(latestBlockSynced))
	if err != nil {
		return 0, err
	}
	var ret uint64
	buf := bytes.NewBuffer(get)
	err = binary.Read(buf, binary.BigEndian, &ret)
	if err != nil {
		return 0, err
	}
	return int64(ret), nil
}

func (s Synchronizer) updateLatestBlockQueried(block int64) error {
	err := (*s.db).Put([]byte(latestBlockSynced), []byte(fmt.Sprintf("%d", block)))
	if err != nil {
		log.Default.With("Block", block, "Key", latestBlockSynced).
			Info("Couldn't store the latest synced block")
		return err
	}
	return nil
}

func (s Synchronizer) latestBlockOnChain() (uint64, error) {
	number, err := s.ethereumClient.BlockNumber(context.Background())
	if err != nil {
		log.Default.With("Error", err).Error("Couldn't get the latest block")
		return 0, err
	}
	return number, nil
}

func (s Synchronizer) FetchStarknetFact(starknetAddress common.Address, fact chan [32]byte) error {
	log.Default.Info("Starting to update state")
	latestBlockNumber, err := s.ethereumClient.BlockNumber(context.Background())
	if err != nil {
		log.Default.With("Error", err).Error("Couldn't get the latest block")
		return err
	}

	if err != nil {
		log.Default.With("Error", err).Error("Couldn't filter the starknetLogs")
		return err
	}

	starknetAbi, err := loadContract(config.Runtime.Starknet.ContractAbiPathConfig.StarknetAbiPath)
	if err != nil {
		log.Default.With("Error", err).Error("Couldn't get the contracts from the ABI")
		return err
	}

	initialBlock := s.initialBlockForStarknetContract()
	increment := uint64(10_000)
	i := uint64(initialBlock)
	for i < latestBlockNumber {
		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(i)),
			ToBlock:   big.NewInt(int64(i + increment)),
			Addresses: []common.Address{
				starknetAddress,
			},
		}
		starknetLogs, err := s.ethereumClient.FilterLogs(context.Background(), query)
		if err != nil {
			log.Default.With("Error", err, "Initial block", i, "End block", i+increment).
				Info("Couldn't get logs from starknet contract")
			continue
		}
		for _, vLog := range starknetLogs {
			p, err := vLog.MarshalJSON()
			if err != nil {
				log.Default.With("Error", err).Error("Couldn't unmarshal starknet log")
				return err
			}

			log.Default.With("BlockHash", vLog.BlockHash.Hex(), "BlockNumber", vLog.BlockNumber,
				"TxHash", vLog.TxHash.Hex(), "Data", string(p)).Info("Starknet Event Fetched")
			event := map[string]interface{}{}
			err = starknetAbi.UnpackIntoMap(event, "LogStateTransitionFact", vLog.Data)
			if err != nil {
				log.Default.With("Error", err).Info("Couldn't get LogStateTransitionFact from event")
				continue
			}
			str := fmt.Sprintf("%v", event["stateTransitionFact"].([32]byte))
			log.Default.With("Value", str).Info("Got Log State Transaction Fact")
			fact <- event["stateTransitionFact"].([32]byte)
		}
		i += increment
	}
	return nil
}

// UpdateStateRoot keeps updating the Ethereum State Root as a process
func (s Synchronizer) UpdateStateRoot() error {
	log.Default.Info("Starting to update state")

	contractAddresses, err := s.feederGatewayClient.GetContractAddresses()
	if err != nil {
		log.Default.With("Error", err).Info("Couldn't get Contract Address from Feeder Gateway")
		return err
	}

	fact := make(chan [32]byte)
	go func() {

		err = s.FetchStarknetFact(common.HexToAddress(contractAddresses.Starknet), fact)
		if err != nil {
			log.Default.With("Error", err).Info("Couldn't get Fact from Starknet Contract Events")
			close(fact)
		}
	}()

	for {
		select {
		case l, ok := <-fact:
			if !ok {
				return fmt.Errorf("couldn't read fact from starknet")
			}
			log.Default.With("Fact", l).
				Info("Getting Fact from Starknet Contract")
		}
	}
}

func loadContract(abiPath string) (abi.ABI, error) {
	log.Default.With("Contract", abiPath).Info("Loading contract")
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

// Close closes the client for the Layer 1 Ethereum node
func (s Synchronizer) Close(ctx context.Context) {
	// notest
	log.Default.Info("Closing Layer 1 Synchronizer")
	select {
	case <-ctx.Done():
		s.ethereumClient.Close()
	default:
	}
}
