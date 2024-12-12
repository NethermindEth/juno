package genesis

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/NethermindEth/juno/vm"
)

type GenesisConfig struct {
	Classes           []string                          `json:"classes"`            // []path-to-class.json
	Contracts         map[felt.Felt]GenesisContractData `json:"contracts"`          // address -> {classHash, constructorArgs}
	FunctionCalls     []FunctionCall                    `json:"function_calls"`     // list of functionCalls to Call()
	BootstrapAccounts []Account                         `json:"bootstrap_accounts"` // accounts to prefund with strk token
}

type Account struct {
	Address felt.Felt `json:"address"`
	PubKey  felt.Felt `json:"publicKey"`
	PrivKey felt.Felt `json:"privateKey"`
}

func Read(path string) (*GenesisConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config GenesisConfig
	if err = config.UnmarshalJSON(file); err != nil {
		return nil, err
	}
	return &config, err
}

func (g *GenesisConfig) UnmarshalJSON(data []byte) error {
	type auxConfig struct {
		Classes           []string                       `json:"classes"`            // []path-to-class.json
		Contracts         map[string]GenesisContractData `json:"contracts"`          // address -> {classHash, constructorArgs}
		FunctionCalls     []FunctionCall                 `json:"function_calls"`     // list of functionCalls to Call()
		BootstrapAccounts []Account                      `json:"bootstrap_accounts"` // accounts to prefund with strk token
	}
	var aux auxConfig
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	g.Classes = aux.Classes
	g.FunctionCalls = aux.FunctionCalls
	g.BootstrapAccounts = aux.BootstrapAccounts
	g.Contracts = make(map[felt.Felt]GenesisContractData)
	for k, v := range aux.Contracts {
		key, err := new(felt.Felt).SetString(k)
		if err != nil {
			return err
		}
		g.Contracts[*key] = v
	}
	return nil
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
	maxSteps uint64,
) (*core.StateDiff, map[felt.Felt]core.Class, error) {
	newClasses, err := loadClasses(config.Classes)
	if err != nil {
		return nil, nil, err
	}

	genesisState := blockchain.NewPendingStateWriter(core.EmptyStateDiff(), make(map[felt.Felt]core.Class),
		core.NewState(db.NewMemTransaction()))

	for classHash, class := range newClasses {
		// Sets pending.newClasses, DeclaredV0Classes, (not DeclaredV1Classes)
		if err = genesisState.SetContractClass(&classHash, class); err != nil {
			return nil, nil, fmt.Errorf("declare v0 class: %v", err)
		}

		if cairo1Class, isCairo1 := class.(*core.Cairo1Class); isCairo1 {
			if err = genesisState.SetCompiledClassHash(&classHash, cairo1Class.Compiled.Hash()); err != nil {
				return nil, nil, fmt.Errorf("set compiled class hash: %v", err)
			}
		}
	}

	constructorSelector, err := new(felt.Felt).SetString("0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194")
	if err != nil {
		return nil, nil, fmt.Errorf("convert string to felt: %v", err)
	}
	for addressFelt, contractData := range config.Contracts {
		classHash := contractData.ClassHash
		if err = genesisState.SetClassHash(&addressFelt, &classHash); err != nil {
			return nil, nil, fmt.Errorf("set class hash: %v", err)
		}
		if contractData.ConstructorArgs != nil {
			callInfo := &vm.CallInfo{
				ContractAddress: &addressFelt,
				ClassHash:       &classHash,
				Selector:        constructorSelector,
				Calldata:        contractData.ConstructorArgs,
			}
			blockInfo := vm.BlockInfo{
				Header: &core.Header{
					Number:    0,
					Timestamp: 0,
				},
			}
			// Call the constructors
			if _, err = v.Call(callInfo, &blockInfo, genesisState, network, maxSteps); err != nil {
				return nil, nil, fmt.Errorf("execute function call: %v", err)
			}
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
				Timestamp: 0,
			},
		}
		if _, err = v.Call(callInfo, &blockInfo, genesisState, network, maxSteps); err != nil {
			return nil, nil, fmt.Errorf("execute function call: %v", err)
		}
	}

	genesisStateDiff, genesisClasses := genesisState.StateDiffAndClasses()
	return genesisStateDiff, genesisClasses, nil
}

func loadClasses(classes []string) (map[felt.Felt]core.Class, error) {
	classMap := make(map[felt.Felt]core.Class)
	for _, classPath := range classes {
		bytes, err := os.ReadFile(classPath)
		if err != nil {
			return nil, fmt.Errorf("read class file: %v", err)
		}

		var response *starknet.ClassDefinition
		if err = json.Unmarshal(bytes, &response); err != nil {
			return nil, fmt.Errorf("unmarshal class: %v", err)
		}

		var coreClass core.Class
		if response.V0 != nil {
			if coreClass, err = sn2core.AdaptCairo0Class(response.V0); err != nil {
				return nil, err
			}
		} else if compiledClass, cErr := compiler.Compile(response.V1); cErr != nil {
			return nil, cErr
		} else if coreClass, err = sn2core.AdaptCairo1Class(response.V1, compiledClass); err != nil {
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
