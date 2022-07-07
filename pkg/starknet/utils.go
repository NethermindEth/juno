package starknet

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"

	"github.com/NethermindEth/juno/internal/db"
	dbAbi "github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/feeder"
	feederAbi "github.com/NethermindEth/juno/pkg/feeder/abi"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// feederErr describes an error from the feeder gateway API response.
type feederErr struct {
	response []byte
}

func (e *feederErr) Error() string {
	return string(e.response)
}

// newTrie returns a new Trie
func newTrie(database db.DatabaseOperations, prefix string) trie.Trie {
	return nil
	//store := db.NewKeyValueStore(database, prefix)
	//return trie.New(store, 251)
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
// leaf of the Merkle Patricia Tree that represent the state in StarkNet
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
	stateDiff.NewRoot = update.NewRoot
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
		if db.IsNotFound(err) {
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
		log.Default.With("Value", value, "Key", key).
			Info("Couldn't store the kv-pair on the database")
		return err
	}
	return nil
}

// updateState is a pure function (besides logging) that applies the
// `update` StateDiff to the database transaction `txn`.
func updateState(
	txn db.DatabaseOperations,
	contractHashMap map[string]*big.Int,
	update *starknetTypes.StateDiff,
	stateRoot string,
	sequenceNumber uint64,
) (string, error) {
	//log.Default.With("Block Number", sequenceNumber).Info("Processing block")
	//
	//stateTrie := newTrie(txn, "state_trie_")
	//
	//log.Default.With("Block Number", sequenceNumber).Info("Processing deployed contracts")
	//for _, deployedContract := range update.DeployedContracts {
	//	contractHash, ok := new(big.Int).SetString(remove0x(deployedContract.ContractHash), 16)
	//	if !ok {
	//		// notest
	//		log.Default.Panic("Couldn't get contract hash")
	//	}
	//	storageTrie := newTrie(txn, remove0x(deployedContract.Address))
	//	storageRoot := storageTrie.Commitment()
	//	address, ok := new(big.Int).SetString(remove0x(deployedContract.Address), 16)
	//	if !ok {
	//		// notest
	//		log.Default.With("Address", deployedContract.Address).
	//			Panic("Couldn't convert Address to Big.Int ")
	//	}
	//	contractStateValue := contractState(contractHash, storageRoot)
	//	stateTrie.Put(address, contractStateValue)
	//}
	//
	//log.Default.With("Block Number", sequenceNumber).Info("Processing storage diffs")
	//for k, v := range update.StorageDiffs {
	//	formattedAddress := remove0x(k)
	//	storageTrie := newTrie(txn, formattedAddress)
	//	for _, storageSlots := range v {
	//		key, ok := new(big.Int).SetString(remove0x(storageSlots.Key), 16)
	//		if !ok {
	//			// notest
	//			log.Default.With("Storage Slot Key", storageSlots.Key).
	//				Panic("Couldn't get the ")
	//		}
	//		val, ok := new(big.Int).SetString(remove0x(storageSlots.Value), 16)
	//		if !ok {
	//			// notest
	//			log.Default.With("Storage Slot Value", storageSlots.Value).
	//				Panic("Couldn't get the contract Hash")
	//		}
	//		storageTrie.Put(key, val)
	//	}
	//	storageRoot := storageTrie.Commitment()
	//
	//	address, ok := new(big.Int).SetString(formattedAddress, 16)
	//	if !ok {
	//		// notest
	//		log.Default.With("Address", formattedAddress).
	//			Panic("Couldn't convert Address to Big.Int ")
	//	}
	//	contractHash := contractHashMap[formattedAddress]
	//	contractStateValue := contractState(contractHash, storageRoot)
	//
	//	stateTrie.Put(address, contractStateValue)
	//}
	//
	//stateCommitment := remove0x(stateTrie.Commitment().Text(16))
	//
	//if stateRoot != "" && stateCommitment != remove0x(stateRoot) {
	//	// notest
	//	log.Default.With("state Commitment", stateCommitment, "state Root from API", remove0x(stateRoot)).
	//		Panic("stateRoot not equal to the one provided")
	//}
	//log.Default.With("state Root", stateCommitment).
	//	Info("Got state commitment")
	//
	//return stateCommitment, nil
	return "", nil
}

// byteCodeToStateCode convert an array of strings to the Code
func byteCodeToStateCode(bytecode []string) *state.Code {
	code := state.Code{}

	for _, bCode := range bytecode {
		code.Code = append(code.Code, types.HexToFelt(bCode).Bytes())
	}

	return &code
}

// feederTransactionToDBTransaction convert the feeder TransactionInfo to the transaction stored in DB
func feederTransactionToDBTransaction(info *feeder.TransactionInfo) types.IsTransaction {
	calldata := make([]types.Felt, 0)
	for _, data := range info.Transaction.Calldata {
		calldata = append(calldata, types.HexToFelt(data))
	}

	if info.Transaction.Type == "INVOKE" {
		signature := make([]types.Felt, 0)
		for _, data := range info.Transaction.Signature {
			signature = append(signature, types.HexToFelt(data))
		}
		return &types.TransactionInvoke{
			Hash:               types.HexToTransactionHash(info.Transaction.TransactionHash),
			ContractAddress:    types.HexToAddress(info.Transaction.ContractAddress),
			EntryPointSelector: types.HexToFelt(info.Transaction.EntryPointSelector),
			CallData:           calldata,
			Signature:          signature,
			MaxFee:             types.Felt{},
		}
	}

	// Is a DEPLOY Transaction
	return &types.TransactionDeploy{
		Hash:                types.HexToTransactionHash(info.Transaction.TransactionHash),
		ContractAddress:     types.HexToAddress(info.Transaction.ContractAddress),
		ConstructorCallData: calldata,
	}
}

// feederBlockToDBBlock convert the feeder block to the block stored in the database
func feederBlockToDBBlock(b *feeder.StarknetBlock) *types.Block {
	txnsHash := make([]types.TransactionHash, 0)
	for _, data := range b.Transactions {
		txnsHash = append(txnsHash, types.TransactionHash(types.HexToFelt(data.TransactionHash)))
	}
	status, _ := types.BlockStatusValue[b.Status]
	return &types.Block{
		BlockHash:   types.HexToBlockHash(b.BlockHash),
		BlockNumber: uint64(b.BlockNumber),
		ParentHash:  types.HexToBlockHash(b.ParentBlockHash),
		Status:      status,
		Sequencer:   types.HexToAddress(b.SequencerAddress),
		NewRoot:     types.HexToFelt(b.StateRoot),
		OldRoot:     types.HexToFelt(b.OldStateRoot),
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

// getFullContract retrieves the compiled contract at address (with 0x
// prefix) from the feeder gateway and returns an error otherwise. Note
// that the feeder gateway client cannot be used here because it returns
// Go objects and what is required here is the raw JSON response.
func getFullContract(baseURL *url.URL, address string) ([]byte, error) {
	url, err := url.Parse(baseURL.String() + "/feeder_gateway/get_full_contract")
	if err != nil {
		return nil, err
	}

	vals := url.Query()
	vals.Add("contractAddress", address)
	url.RawQuery = vals.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Ensure that the feeder gateway did not return an error.
	if bytes.Contains(body, []byte(`{"code": "StarkErrorCode.`)) {
		return nil, &feederErr{body}
	}

	return body, nil
}
