package genesis

import (
	"encoding/json"
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
	if err != nil {
		return nil, nil, err
	}

	for classHash, class := range newClasses {
		switch class.Version() {
		case 0:
			// Sets pending.newClasses, DeclaredV0Classes, (not DeclaredV1Classes)
			if err = genesisState.SetContractClass(&classHash, class); err != nil {
				return nil, nil, fmt.Errorf("failed to set cairo v0 contract class : %v", err)
			}
		default:
			return nil, nil, fmt.Errorf("only cairo v 0 contracts are supported for genesis state initialisation")
		}
	}

	constructorSelector, err := new(felt.Felt).SetString("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194")
	if err != nil {
		return nil, nil, err
	}

	for address, contractData := range config.Contracts {
		addressFelt, err := new(felt.Felt).SetString(address)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to set contract address as felt %s", err)
		}
		classHash := contractData.ClassHash
		err = genesisState.SetClassHash(addressFelt, &classHash) // Sets DeployedContracts, ReplacedClasses
		if err != nil {
			return nil, nil, fmt.Errorf("failed to set contract class hash %s", err)
		}

		// Call the constructors
		_, err = v.Call(addressFelt, &classHash, constructorSelector,
			contractData.ConstructorArgs, 0, blockTimestamp, genesisState, *network)
		if err != nil {
			return nil, nil, err
		}
	}

	for _, fnCall := range config.FunctionCalls {
		contractAddress := fnCall.ContractAddress
		entryPointSelector := fnCall.EntryPointSelector
		classHash, err := genesisState.ContractClassHash(&contractAddress)
		if err != nil {
			return nil, nil, err
		}
		_, err = v.Call(&contractAddress, classHash, &entryPointSelector, fnCall.Calldata, 0, blockTimestamp, genesisState, *network)
		if err != nil {
			return nil, nil, err
		}
	}
	return genesisState.StateDiff(), newClasses, nil
}

func loadClasses(classes []string) (map[felt.Felt]core.Class, error) {
	classMap := make(map[felt.Felt]core.Class)
	for _, classPath := range classes {
		bytes, err := os.ReadFile(classPath)
		if err != nil {
			return nil, err
		}

		var response *starknet.Cairo0Definition
		if err = json.Unmarshal(bytes, &response); err != nil {
			return nil, err
		}

		coreClass, err := sn2core.AdaptCairo0Class(response)
		if err != nil {
			return nil, err
		}

		classhash, err := coreClass.Hash()
		if err != nil {
			return nil, err
		}
		classMap[*classhash] = coreClass
	}
	return classMap, nil
}
