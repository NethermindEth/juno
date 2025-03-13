package genesis

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/NethermindEth/juno/vm"
)

// The constructor entrypoint for cairo contracts
const constructorSelector = "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194"

var genesisHeader = core.Header{
	Number:    0,
	Timestamp: 0,
}

type GenesisConfig struct {
	Classes           []string                          `json:"classes"`            // []path-to-class.json
	Contracts         map[felt.Felt]GenesisContractData `json:"contracts"`          // address -> {classHash, constructorArgs}
	FunctionCalls     []FunctionCall                    `json:"function_calls"`     // list of functionCalls to Call()
	BootstrapAccounts []Account                         `json:"bootstrap_accounts"` // accounts to prefund with strk token
	Txns              []rpc.Transaction                 `json:"transactions"`       // Declare NOT supported
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
	var aux struct {
		Classes           []string                       `json:"classes"`            // []path-to-class.json
		Contracts         map[string]GenesisContractData `json:"contracts"`          // address -> {classHash, constructorArgs}
		FunctionCalls     []FunctionCall                 `json:"function_calls"`     // list of functionCalls to Call()
		BootstrapAccounts []Account                      `json:"bootstrap_accounts"` // accounts to prefund with strk token
		Txns              []rpc.Transaction              `json:"transactions"`       // declare NOT supported
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	g.Classes = aux.Classes
	g.FunctionCalls = aux.FunctionCalls
	g.BootstrapAccounts = aux.BootstrapAccounts
	g.Contracts = make(map[felt.Felt]GenesisContractData, len(aux.Contracts))
	for classHash, constructorArgs := range aux.Contracts {
		key, err := new(felt.Felt).SetString(classHash)
		if err != nil {
			return err
		}
		g.Contracts[*key] = constructorArgs
	}
	g.Txns = aux.Txns
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
	return validator.Validator().Struct(g)
}

// GenesisState builds the genesis state given the genesis-config data.
func GenesisStateDiff( //nolint:funlen,gocyclo
	config *GenesisConfig,
	v vm.VM,
	network *utils.Network,
	maxSteps uint64,
) (*core.StateDiff, map[felt.Felt]core.Class, error) {
	newClasses, err := loadClasses(config.Classes)
	if err != nil {
		return nil, nil, err
	}
	blockInfo := vm.BlockInfo{
		Header: &genesisHeader,
	}
	genesisState := sync.NewPendingStateWriter(core.EmptyStateDiff(), make(map[felt.Felt]core.Class),
		core.NewState(db.NewMemTransaction()))

	classhashToSierraVersion := make(map[felt.Felt]string, len(newClasses))
	contractAddressToSierraVersion := make(map[felt.Felt]string, len(config.Contracts))
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
		classhashToSierraVersion[classHash] = class.SierraVersion()
	}

	constructorSelector, err := new(felt.Felt).SetString(constructorSelector)
	if err != nil {
		return nil, nil, fmt.Errorf("convert string to felt: %v", err)
	}

	for address, contractData := range config.Contracts {
		classHash := contractData.ClassHash
		contractAddressToSierraVersion[address] = classhashToSierraVersion[classHash]
		if err = genesisState.SetClassHash(&address, &classHash); err != nil {
			return nil, nil, fmt.Errorf("set class hash: %v", err)
		}
		if contractData.ConstructorArgs != nil {
			callInfo := &vm.CallInfo{
				ContractAddress: &address,
				ClassHash:       &classHash,
				Selector:        constructorSelector,
				Calldata:        contractData.ConstructorArgs,
			}
			// Call the constructors
			result, err := v.Call(callInfo, &blockInfo, genesisState, network, maxSteps, classhashToSierraVersion[classHash], true, true)
			if err != nil {
				return nil, nil, fmt.Errorf("execute function call: %v", err)
			}
			var coreSD core.StateDiff
			vm2core.AdaptStateDiff(&result.StateDiff, &coreSD)
			genesisState.StateDiff().MergeStateDiffs(&coreSD)
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
			Header: &genesisHeader,
		}
		result, err := v.Call(callInfo, &blockInfo, genesisState, network, maxSteps, contractAddressToSierraVersion[contractAddress], true, true)
		if err != nil {
			return nil, nil, fmt.Errorf("execute function call: %v", err)
		}
		var coreSD core.StateDiff
		vm2core.AdaptStateDiff(&result.StateDiff, &coreSD)
		genesisState.StateDiff().MergeStateDiffs(&coreSD)
	}

	if len(config.Txns) != 0 {
		coreTxns := make([]core.Transaction, len(config.Txns))
		for i, txn := range config.Txns {
			switch txn.Type {
			case rpc.TxnInvoke:
				coreTxns[i] = &core.InvokeTransaction{
					TransactionHash:      txn.Hash,
					CallData:             *txn.CallData,
					TransactionSignature: *txn.Signature,
					MaxFee:               txn.MaxFee,
					ContractAddress:      txn.ContractAddress,
					Version:              (*core.TransactionVersion)(txn.Version),
					EntryPointSelector:   txn.EntryPointSelector,
					Nonce:                txn.Nonce,
					SenderAddress:        txn.SenderAddress,
				}
			case rpc.TxnDeployAccount:
				coreTxns[i] = &core.DeployAccountTransaction{
					DeployTransaction: core.DeployTransaction{
						TransactionHash:     txn.Hash,
						ContractAddressSalt: txn.ContractAddressSalt,
						ContractAddress:     txn.SenderAddress,
						ClassHash:           txn.ClassHash,
						ConstructorCallData: *txn.ConstructorCallData,
						Version:             (*core.TransactionVersion)(txn.Version),
					},
					MaxFee:               txn.MaxFee,
					TransactionSignature: *txn.Signature,
					Nonce:                txn.Nonce,
				}
			default:
				return nil, nil, fmt.Errorf("unsupported transaction type: %v", txn.Type)
			}
		}
		executionResults, err := v.Execute(coreTxns, nil, []*felt.Felt{new(felt.Felt).SetUint64(1)},
			&blockInfo, genesisState, network, true, false, true, true)
		if err != nil {
			return nil, nil, fmt.Errorf("execute function call: %v", err)
		}
		for i := range config.Txns {
			var traceSD core.StateDiff
			vm2core.AdaptStateDiff(executionResults.Traces[i].StateDiff, &traceSD)
			genesisSD, _ := genesisState.StateDiffAndClasses()
			genesisSD.Merge(&traceSD)
			genesisState.SetStateDiff(genesisSD)
		}
	}
	genesisStateDiff, genesisClasses := genesisState.StateDiffAndClasses()
	return genesisStateDiff, genesisClasses, nil
}

func loadClasses(classes []string) (map[felt.Felt]core.Class, error) {
	classMap := make(map[felt.Felt]core.Class, len(classes))
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
		} else {
			var compiledClass *starknet.CompiledClass
			if compiledClass, err = compiler.Compile(response.V1); err != nil {
				return nil, err
			}
			if coreClass, err = sn2core.AdaptCairo1Class(response.V1, compiledClass); err != nil {
				return nil, err
			}
		}

		classhash, err := coreClass.Hash()
		if err != nil {
			return nil, fmt.Errorf("calculate class hash: %v", err)
		}
		classMap[*classhash] = coreClass
	}
	return classMap, nil
}
