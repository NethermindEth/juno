package starknet

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"

	"github.com/NethermindEth/juno/internal/db"
	dbAbi "github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/db/state"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/feeder"
	feederAbi "github.com/NethermindEth/juno/pkg/feeder/abi"
	"github.com/NethermindEth/juno/pkg/felt"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// newTrie returns a new Trie
func newTrie(database db.DatabaseOperations, prefix string) trie.Trie {
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
func contractState(contractHash, storageRoot *felt.Felt) *felt.Felt {
	// Is defined as:
	// h(h(h(contract_hash, storage_root), 0), 0).
	val := pedersen.Digest(contractHash, storageRoot)
	val = pedersen.Digest(val, new(felt.Felt).SetZero())
	val = pedersen.Digest(val, new(felt.Felt).SetZero())
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
func getNumericValueFromDB(database db.Database, key string) (uint64, error) {
	value, err := database.Get([]byte(key))
	if err != nil {
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if value == nil {
		// notest
		return 0, nil
	}
	var ret uint64
	buf := bytes.NewBuffer(value)
	err = binary.Read(buf, binary.BigEndian, &ret)
	if err != nil {
		return 0, err
	}
	return ret, nil
}

// updateNumericValueFromDB update the value in the database for a key increasing the value in 1
func updateNumericValueFromDB(database db.DatabaseOperations, key string, value uint64) error {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, value+1)
	err := database.Put([]byte(key), b)
	if err != nil {
		Logger.With("Value", value, "Key", key).
			Info("Couldn't store the kv-pair on the database")
		return err
	}
	return nil
}

// updateState is a pure function (besides logging) that applies the
// `update` StateDiff to the database transaction `txn`.
func updateState(
	txn db.DatabaseOperations,
	contractHashMap map[string]*felt.Felt,
	update *starknetTypes.StateDiff,
	stateRoot string,
	sequenceNumber uint64,
) (string, error) {
	Logger.With("Block Number", sequenceNumber).Info("Processing block")

	stateTrie := newTrie(txn, "state_trie_")

	Logger.With("Block Number", sequenceNumber).Info("Processing deployed contracts")
	for _, deployedContract := range update.DeployedContracts {
		contractHash := new(felt.Felt).SetHex(deployedContract.ContractHash)
		storageTrie := newTrie(txn, remove0x(deployedContract.Address))
		storageRoot := storageTrie.Commitment()
		address := new(felt.Felt).SetHex(deployedContract.Address)
		contractStateValue := contractState(contractHash, storageRoot)
		stateTrie.Put(address, contractStateValue)
	}

	Logger.With("Block Number", sequenceNumber).Info("Processing storage diffs")
	for k, v := range update.StorageDiffs {
		formattedAddress := remove0x(k)
		storageTrie := newTrie(txn, formattedAddress)
		for _, storageSlots := range v {
			key := new(felt.Felt).SetHex(storageSlots.Key)
			val := new(felt.Felt).SetHex(storageSlots.Value)
			storageTrie.Put(key, val)
		}
		storageRoot := storageTrie.Commitment()

		address := new(felt.Felt).SetHex(formattedAddress)
		contractHash := contractHashMap[formattedAddress]
		contractStateValue := contractState(contractHash, storageRoot)

		stateTrie.Put(address, contractStateValue)
	}

	stateCommitment := remove0x(stateTrie.Commitment().Text(16))

	if stateRoot != "" && stateCommitment != remove0x(stateRoot) {
		// notest
		Logger.With("State Commitment", stateCommitment, "State Root from API", remove0x(stateRoot)).
			Panic("stateRoot not equal to the one provided")
	}
	Logger.With("State Root", stateCommitment).
		Info("Got State commitment")

	return stateCommitment, nil
}

// byteCodeToStateCode convert an array of strings to the Code
func byteCodeToStateCode(bytecode []string) *state.Code {
	code := state.Code{}

	for _, bCode := range bytecode {
		code.Code = append(code.Code, new(felt.Felt).SetHex(bCode).ByteSlice())
	}

	return &code
}

// feederTransactionToDBTransaction convert the feeder TransactionInfo to the transaction stored in DB
func feederTransactionToDBTransaction(info *feeder.TransactionInfo) types.IsTransaction {
	calldata := make([]*felt.Felt, 0)
	for _, data := range info.Transaction.Calldata {
		calldata = append(calldata, new(felt.Felt).SetHex(data))
	}

	if info.Transaction.Type == "INVOKE" {
		signature := make([]*felt.Felt, 0)
		for _, data := range info.Transaction.Signature {
			signature = append(signature, new(felt.Felt).SetHex(data))
		}
		return &types.TransactionInvoke{
			Hash:               new(felt.Felt).SetHex(info.Transaction.TransactionHash),
			ContractAddress:    new(felt.Felt).SetHex(info.Transaction.ContractAddress),
			EntryPointSelector: new(felt.Felt).SetHex(info.Transaction.EntryPointSelector),
			CallData:           calldata,
			Signature:          signature,
			MaxFee:             new(felt.Felt),
		}
	}

	// Is a DEPLOY Transaction
	return &types.TransactionDeploy{
		Hash:                new(felt.Felt).SetHex(info.Transaction.TransactionHash),
		ContractAddress:     new(felt.Felt).SetHex(info.Transaction.ContractAddress),
		ConstructorCallData: calldata,
	}
}

// feederBlockToDBBlock convert the feeder block to the block stored in the database
func feederBlockToDBBlock(b *feeder.StarknetBlock) *types.Block {
	txnsHash := make([]*felt.Felt, 0)
	for _, data := range b.Transactions {
		txnsHash = append(txnsHash, new(felt.Felt).SetHex(data.TransactionHash))
	}
	status := types.BlockStatusValue[b.Status]
	return &types.Block{
		BlockHash:   new(felt.Felt).SetHex(b.BlockHash),
		BlockNumber: uint64(b.BlockNumber),
		ParentHash:  new(felt.Felt).SetHex(b.ParentBlockHash),
		Status:      status,
		Sequencer:   new(felt.Felt).SetHex(b.SequencerAddress),
		NewRoot:     new(felt.Felt).SetHex(b.StateRoot),
		OldRoot:     new(felt.Felt).SetHex(b.OldStateRoot),
		TimeStamp:   b.Timestamp,
		TxCount:     uint64(len(b.Transactions)),
		TxHashes:    txnsHash,
	}
}

func toDbAbi(abi feederAbi.Abi) *dbAbi.Abi {
	marshal, err := json.Marshal(abi)
	if err != nil {
		// notest
		return nil
	}
	var abiResponse dbAbi.Abi

	err = json.Unmarshal(marshal, &abiResponse)
	if err != nil {
		// notest
		return nil
	}

	for i, str := range abi.Structs {
		abiResponse.Structs[i].Fields = make([]*dbAbi.Struct_Field, len(str.Members))
		for j, field := range str.Members {
			abiResponse.Structs[i].Fields[j] = &dbAbi.Struct_Field{
				Name:   field.Name,
				Type:   field.Type,
				Offset: uint32(field.Offset),
			}
		}
	}

	return &abiResponse
}
