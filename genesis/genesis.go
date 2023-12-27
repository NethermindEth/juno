package genesis

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/NethermindEth/juno/vm"
)

type GenesisConfig struct {
	ChainID       string                         `json:"chain_id" validate:"required"`
	Classes       []string                       `json:"classes"`        // []path-to-class.json
	Contracts     map[string]GenesisContractData `json:"contracts"`      // address -> {classHash, constructorArgs}
	FunctionCalls []FunctionCall                 `json:"function_calls"` // list of functionCalls to Call()
}

type GenesisContractData struct {
	ClassHash       felt.Felt   `json:"class_hash"`
	ConstructorArgs []felt.Felt `json:"constructor_args"`
}

type FunctionCall struct {
	ContractAddress    felt.Felt   `json:"contract_address"`
	EntryPointSelector felt.Felt   `json:"entry_point_selector"`
	Calldata           []felt.Felt `json:"calldata"`
}

func (g *GenesisConfig) Validate() error {
	validate := validator.Validator()
	return validate.Struct(g)
}

// GenesisState builds the genesis state given the genesis-config data.
func GenesisStateDiff(
	config *GenesisConfig,
	v vm.VM,
	network *utils.Network,
) (*core.StateDiff, map[felt.Felt]core.Class, error) {
	blockTimestamp := uint64(time.Now().Unix())

	newClasses, err := loadClasses(config.Classes)
	if err != nil {
		return nil, nil, err
	}

	genesisState := blockchain.NewPendingState(core.EmptyStateDiff(), make(map[felt.Felt]core.Class), core.NewState(db.NewMemTransaction()))

	for classHash, class := range newClasses {
		switch class.Version() {
		case 0:
			// Sets pending.newClasses, DeclaredV0Classes, (not DeclaredV1Classes)
			if err = genesisState.SetContractClass(&classHash, class); err != nil {
				return nil, nil, fmt.Errorf("declare v0 class: %v", err)
			}
		default:
			return nil, nil, errors.New("only cairo v0 contracts are supported for genesis state initialisation")
		}
	}

	constructorSelector, err := new(felt.Felt).SetString("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194")
	if err != nil {
		return nil, nil, fmt.Errorf("convert string to felt: %v", err)
	}

	for address, contractData := range config.Contracts {
		addressFelt, err := new(felt.Felt).SetString(address)
		if err != nil {
			return nil, nil, fmt.Errorf("convert string to felt: %v", err)
		}
		classHash := contractData.ClassHash
		if err = genesisState.SetClassHash(addressFelt, &classHash); err != nil {
			return nil, nil, fmt.Errorf("set class hash: %v", err)
		}
		callInfo := &vm.CallInfo{
			ContractAddress: addressFelt,
			ClassHash:       &classHash,
			Selector:        constructorSelector,
			Calldata:        contractData.ConstructorArgs,
		}
		blockInfo := vm.BlockInfo{
			Header: &core.Header{
				Number:    0,
				Timestamp: blockTimestamp,
			},
		}
		maxSteps := uint64(100000) //nolint:gomnd
		// Call the constructors
		if _, err = v.Call(callInfo, &blockInfo, genesisState, network, maxSteps, false); err != nil {
			return nil, nil, fmt.Errorf("execute function call: %v", err)
		}
	}

	for _, fnCall := range config.FunctionCalls {
		contractAddress := fnCall.ContractAddress
		entryPointSelector := fnCall.EntryPointSelector
		classHash, err := genesisState.ContractClassHash(&contractAddress)
		if err != nil {
			return nil, nil, fmt.Errorf("get contract class hash: %v", err)
		}
		callInfo := &vm.CallInfo{
			ContractAddress: &contractAddress,
			ClassHash:       classHash,
			Selector:        &entryPointSelector,
			Calldata:        fnCall.Calldata,
		}
		blockInfo := vm.BlockInfo{
			Header: &core.Header{
				Number:    0,
				Timestamp: blockTimestamp,
			},
		}
		maxSteps := uint64(100000) //nolint:gomnd
		if _, err = v.Call(callInfo, &blockInfo, genesisState, network, maxSteps, false); err != nil {
			return nil, nil, fmt.Errorf("execute function call: %v", err)
		}
	}
	return genesisState.StateDiff(), newClasses, nil
}

func loadClasses(classes []string) (map[felt.Felt]core.Class, error) {
	classMap := make(map[felt.Felt]core.Class)
	for _, classPath := range classes {
		bytes, err := os.ReadFile(classPath)
		if err != nil {
			return nil, fmt.Errorf("read class file: %v", err)
		}

		var response *starknet.Cairo0Definition
		if err = json.Unmarshal(bytes, &response); err != nil {
			return nil, fmt.Errorf("unmarshal class: %v", err)
		}

		coreClass, err := sn2core.AdaptCairo0Class(response)
		if err != nil {
			return nil, err
		}

		classhash, err := coreClass.Hash()
		if err != nil {
			return nil, fmt.Errorf("calculate class hash: %v", err)
		}
		classMap[*classhash] = coreClass
	}
	return classMap, nil
}
