package starknet

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/NethermindEth/juno/internal/db"
	dbAbi "github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/db/transaction"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services"
	commonLocal "github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/feeder"
	feederAbi "github.com/NethermindEth/juno/pkg/feeder/abi"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// newTrie returns a new Trie
func newTrie(database db.Databaser, prefix string) trie.Trie {
	store := db.NewKeyValueStore(database, prefix)
	return trie.New(store, 251)
}

// loadContractInfo loads a contract ABI and set the events that later we are going to use
func loadContractInfo(contractAddress, abiValue, logName string, contracts map[common.Address]starknetTypes.ContractInfo) error {
	contractAddressHash := common.HexToAddress(contractAddress)
	contractFromAbi, err := loadAbiOfContract(abiValue)
	if err != nil {
		return err
	}
	contracts[contractAddressHash] = starknetTypes.ContractInfo{
		Contract:  contractFromAbi,
		EventName: logName,
	}
	return nil
}

// loadAbiOfContract loads the ABI of the contract from the
func loadAbiOfContract(abiVal string) (abi.ABI, error) {
	contractAbi, err := abi.JSON(strings.NewReader(abiVal))
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
	val := pedersen.Digest(contractHash, storageRoot)
	val = pedersen.Digest(val, big.NewInt(0))
	val = pedersen.Digest(val, big.NewInt(0))
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
func stateUpdateResponseToStateDiff(update feeder.StateUpdateResponse) starknetTypes.StateDiff {
	var stateDiff starknetTypes.StateDiff
	stateDiff.DeployedContracts = make([]starknetTypes.DeployedContract, len(update.StateDiff.DeployedContracts))
	for i, v := range update.StateDiff.DeployedContracts {
		stateDiff.DeployedContracts[i] = starknetTypes.DeployedContract{
			Address:      v.Address,
			ContractHash: v.ContractHash,
		}
	}
	stateDiff.StorageDiffs = make(map[string][]starknetTypes.KV)
	for addressDiff, keyVals := range update.StateDiff.StorageDiffs {
		address := addressDiff
		kvs := make([]starknetTypes.KV, 0)
		for _, kv := range keyVals {
			kvs = append(kvs, starknetTypes.KV{
				Key:   kv.Key,
				Value: kv.Value,
			})
		}
		stateDiff.StorageDiffs[address] = kvs
	}

	return stateDiff
}

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func getGpsVerifierContractAddress(id int64) string {
	if id == 1 {
		return starknetTypes.GpsVerifierContractAddressMainnet
	}
	return starknetTypes.GpsVerifierContractAddressGoerli
}

// getGpsVerifierAddress returns the address of the GpsVerifierStatement in the current chain
func getMemoryPagesContractAddress(id int64) string {
	if id == 1 {
		return starknetTypes.MemoryPagesContractAddressMainnet
	}
	return starknetTypes.MemoryPagesContractAddressGoerli
}

// initialBlockForStarknetContract Returns the first block that we need to start to fetch the facts from l1
func initialBlockForStarknetContract(id int64) int64 {
	if id == 1 {
		return starknetTypes.BlockOfStarknetDeploymentContractMainnet
	}
	return starknetTypes.BlockOfStarknetDeploymentContractGoerli
}

// getNumericValueFromDB get the value associated to a key and convert it to integer
func getNumericValueFromDB(database db.Databaser, key string) (uint64, error) {
	value, err := database.Get([]byte(key))
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, nil
	}
	var ret uint64
	buf := bytes.NewBuffer(value)
	err = binary.Read(buf, binary.BigEndian, &ret)
	if err != nil {
		return 0, err
	}
	return uint64(ret), nil
}

// updateNumericValueFromDB update the value in the database for a key increasing the value in 1
func updateNumericValueFromDB(database db.Databaser, key string, value uint64) error {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, value+1)
	err := database.Put([]byte(key), b)
	if err != nil {
		log.Default.With("Value", value, "Key", key).
			Info("Couldn't store the kv-pair on the database")
		return err
	}
	return nil
}

// updateState is a pure function (besides logging) that applies the
// `update` StateDiff to the database transaction `txn`.
func updateState(
	txn db.Transaction,
	hashService *services.ContractHashService,
	update *starknetTypes.StateDiff,
	stateRoot string,
	sequenceNumber uint64,
) (string, error) {
	log.Default.With("Block Number", sequenceNumber).Info("Processing block")

	stateTrie := newTrie(txn, "state_trie_")

	log.Default.With("Block Number", sequenceNumber).Info("Processing deployed contracts")
	for _, deployedContract := range update.DeployedContracts {
		contractHash, ok := new(big.Int).SetString(remove0x(deployedContract.ContractHash), 16)
		if !ok {
			// notest
			log.Default.Panic("Couldn't get contract hash")
		}
		hashService.StoreContractHash(remove0x(deployedContract.Address), contractHash)
		storageTrie := newTrie(txn, remove0x(deployedContract.Address))
		storageRoot := storageTrie.Commitment()
		address, ok := new(big.Int).SetString(remove0x(deployedContract.Address), 16)
		if !ok {
			// notest
			log.Default.With("Address", deployedContract.Address).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractStateValue := contractState(contractHash, storageRoot)
		stateTrie.Put(address, contractStateValue)
	}

	log.Default.With("Block Number", sequenceNumber).Info("Processing storage diffs")
	for k, v := range update.StorageDiffs {
		formattedAddress := remove0x(k)
		storageTrie := newTrie(txn, formattedAddress)
		for _, storageSlots := range v {
			key, ok := new(big.Int).SetString(remove0x(storageSlots.Key), 16)
			if !ok {
				// notest
				log.Default.With("Storage Slot Key", storageSlots.Key).
					Panic("Couldn't get the ")
			}
			val, ok := new(big.Int).SetString(remove0x(storageSlots.Value), 16)
			if !ok {
				// notest
				log.Default.With("Storage Slot Value", storageSlots.Value).
					Panic("Couldn't get the contract Hash")
			}
			storageTrie.Put(key, val)
		}
		storageRoot := storageTrie.Commitment()

		address, ok := new(big.Int).SetString(formattedAddress, 16)
		if !ok {
			// notest
			log.Default.With("Address", formattedAddress).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractHash := hashService.GetContractHash(formattedAddress)
		contractStateValue := contractState(contractHash, storageRoot)

		stateTrie.Put(address, contractStateValue)
	}

	stateCommitment := remove0x(stateTrie.Commitment().Text(16))

	if stateRoot != "" && stateCommitment != remove0x(stateRoot) {
		// notest
		log.Default.With("State Commitment", stateCommitment, "State Root from API", remove0x(stateRoot)).
			Panic("stateRoot not equal to the one provided")
	}
	log.Default.With("State Root", stateCommitment).
		Info("Got State commitment")

	return stateCommitment, nil
}

// byteCodeToStateCode convert an array of strings to the Code
func byteCodeToStateCode(bytecode []string) *state.Code {
	code := state.Code{}

	for _, bCode := range bytecode {
		code.Code = append(code.Code, commonLocal.HexToFelt(bCode).Bytes())
	}

	return &code
}

// feederTransactionToDBTransaction convert the feeder TransactionInfo to the transaction stored in DB
func feederTransactionToDBTransaction(info *feeder.TransactionInfo) *transaction.Transaction {
	calldata := make([][]byte, 0)
	for _, data := range info.Transaction.Calldata {
		calldata = append(calldata, commonLocal.HexToFelt(data).Bytes())
	}

	if info.Transaction.Type == "INVOKE" {
		signature := make([][]byte, 0)
		for _, data := range info.Transaction.Signature {
			signature = append(signature, commonLocal.HexToFelt(data).Bytes())
		}
		return &transaction.Transaction{
			Hash: commonLocal.HexToFelt(info.Transaction.TransactionHash).Bytes(),
			Tx: &transaction.Transaction_Invoke{Invoke: &transaction.InvokeFunction{
				ContractAddress:    commonLocal.HexToFelt(info.Transaction.ContractAddress).Bytes(),
				EntryPointSelector: commonLocal.HexToFelt(info.Transaction.EntryPointSelector).Bytes(),
				CallData:           calldata,
				Signature:          signature,
			}},
		}
	}

	// Is a DEPLOY Transaction
	return &transaction.Transaction{
		Hash: commonLocal.HexToFelt(info.Transaction.TransactionHash).Bytes(),
		Tx: &transaction.Transaction_Deploy{Deploy: &transaction.Deploy{
			ContractAddressSalt: commonLocal.HexToFelt(info.Transaction.ContractAddressSalt).Bytes(),
			ConstructorCallData: calldata,
		}},
	}
}

// feederBlockToDBBlock convert the feeder block to the block stored in the database
func feederBlockToDBBlock(b *feeder.StarknetBlock) *block.Block {
	txnsHash := make([][]byte, 0)
	for _, data := range b.Transactions {
		txnsHash = append(txnsHash, commonLocal.HexToFelt(data.TransactionHash).Bytes())
	}
	return &block.Block{
		Hash:             commonLocal.HexToFelt(b.BlockHash).Bytes(),
		BlockNumber:      uint64(b.BlockNumber),
		ParentBlockHash:  commonLocal.HexToFelt(b.ParentBlockHash).Bytes(),
		Status:           string(b.Status),
		SequencerAddress: commonLocal.HexToFelt(b.SequencerAddress).Bytes(),
		GlobalStateRoot:  commonLocal.HexToFelt(b.StateRoot).Bytes(),
		OldRoot:          commonLocal.HexToFelt(b.OldStateRoot).Bytes(),
		TimeStamp:        b.Timestamp,
		TxCount:          uint64(len(b.Transactions)),
		TxHashes:         txnsHash,
	}
}

func toDbAbi(abi feederAbi.Abi) *dbAbi.Abi {
	marshal, err := json.Marshal(abi)
	if err != nil {
		return nil
	}
	var abiResponse dbAbi.Abi

	err = json.Unmarshal(marshal, &abiResponse)
	if err != nil {
		return nil
	}
	return &abiResponse
}
