package genesis

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	rpc "github.com/NethermindEth/juno/rpc/v8"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
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

// GenesisStateDiff builds the genesis state given the genesis-config data.
func GenesisStateDiff(
	config *GenesisConfig,
	v vm.VM,
	network *utils.Network,
	maxSteps uint64,
	maxGas uint64,
) (core.StateDiff, map[felt.Felt]core.ClassDefinition, error) {
	initialStateDiff := core.EmptyStateDiff()
	memDB := memory.New()
	genesisState := core.NewPendingStateWriter(
		&initialStateDiff,
		make(map[felt.Felt]core.ClassDefinition, len(config.Classes)),
		core.NewState(memDB.NewIndexedBatch()),
	)

	if err := declareClasses(config, &genesisState); err != nil {
		return core.StateDiff{}, nil, err
	}

	if err := deployContracts(config, v, maxSteps, maxGas, &genesisState); err != nil {
		return core.StateDiff{}, nil, err
	}

	if err := executeFunctionCalls(config, v, maxSteps, maxGas, &genesisState); err != nil {
		return core.StateDiff{}, nil, err
	}

	if err := executeTransactions(config, v, &genesisState); err != nil {
		return core.StateDiff{}, nil, err
	}

	genesisStateDiff, genesisClasses := genesisState.StateDiffAndClasses()
	return genesisStateDiff, genesisClasses, nil
}

func declareClasses(
	config *GenesisConfig,
	genesisState *core.PendingStateWriter,
) error {
	newClasses, err := loadClasses(config.Classes)
	if err != nil {
		return err
	}

	for classHash, class := range newClasses {
		if err := setClass(genesisState, &classHash, class); err != nil {
			return err
		}
	}

	return nil
}

func setClass(
	genesisState *core.PendingStateWriter,
	classHash *felt.Felt,
	class core.ClassDefinition,
) error {
	if err := genesisState.SetContractClass(classHash, class); err != nil {
		return fmt.Errorf("declare v0 class: %v", err)
	}

	if sierraClass, isCairo1 := class.(*core.SierraClass); isCairo1 {
		casmHash := sierraClass.Compiled.Hash(core.HashVersionV1)
		if err := genesisState.SetCompiledClassHash(classHash, &casmHash); err != nil {
			return fmt.Errorf("set compiled class hash: %v", err)
		}
	}
	return nil
}

func deployContracts(
	config *GenesisConfig,
	v vm.VM,
	maxSteps uint64,
	maxGas uint64,
	genesisState *core.PendingStateWriter,
) error {
	constructorSelector, err := new(felt.Felt).SetString(constructorSelector)
	if err != nil {
		return fmt.Errorf("convert string to felt: %v", err)
	}

	blockInfo := vm.BlockInfo{Header: &genesisHeader}

	for address, contractData := range config.Contracts {
		err := deployContract(
			v,
			maxSteps,
			maxGas,
			genesisState,
			address,
			contractData,
			constructorSelector,
			&blockInfo,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func deployContract(
	v vm.VM,
	maxSteps uint64,
	maxGas uint64,
	genesisState *core.PendingStateWriter,
	address felt.Felt,
	contractData GenesisContractData,
	constructorSelector *felt.Felt,
	blockInfo *vm.BlockInfo,
) error {
	classHash := contractData.ClassHash

	if err := genesisState.SetClassHash(&address, &classHash); err != nil {
		return fmt.Errorf("set class hash: %v", err)
	}

	if contractData.ConstructorArgs == nil {
		return nil
	}

	callInfo := &vm.CallInfo{
		ContractAddress: &address,
		ClassHash:       &classHash,
		Selector:        constructorSelector,
		Calldata:        contractData.ConstructorArgs,
	}

	result, err := v.Call(
		callInfo,
		blockInfo,
		genesisState,
		maxSteps,
		maxGas,
		true,
		true,
	)
	if err != nil {
		return fmt.Errorf("execute constructor call: %v", err)
	}

	coreSD := vm2core.AdaptStateDiff(&result.StateDiff)
	genesisState.StateDiff().Merge(&coreSD)
	return nil
}

func executeFunctionCalls(
	config *GenesisConfig,
	v vm.VM,
	maxSteps uint64,
	maxGas uint64,
	genesisState *core.PendingStateWriter,
) error {
	blockInfo := vm.BlockInfo{Header: &genesisHeader}

	for _, fnCall := range config.FunctionCalls {
		contractAddress := fnCall.ContractAddress
		entryPointSelector := fnCall.EntryPointSelector

		classHash, err := genesisState.ContractClassHash(&contractAddress)
		if err != nil {
			return fmt.Errorf("get contract class hash: %v", err)
		}

		callInfo := &vm.CallInfo{
			ContractAddress: &contractAddress,
			ClassHash:       &classHash,
			Selector:        &entryPointSelector,
			Calldata:        fnCall.Calldata,
		}

		result, err := v.Call(
			callInfo,
			&blockInfo,
			genesisState,
			maxSteps,
			maxGas,
			true,
			true,
		)
		if err != nil {
			return fmt.Errorf("execute function call: %v", err)
		}

		coreSD := vm2core.AdaptStateDiff(&result.StateDiff)
		genesisState.StateDiff().Merge(&coreSD)
	}

	return nil
}

func executeTransactions(
	config *GenesisConfig,
	v vm.VM,
	genesisState *core.PendingStateWriter,
) error {
	if len(config.Txns) == 0 {
		return nil
	}

	coreTxns := make([]core.Transaction, len(config.Txns))
	for i := range config.Txns {
		txn := &config.Txns[i]
		switch txn.Type {
		case rpc.TxnInvoke:
			coreTxns[i] = &core.InvokeTransaction{
				TransactionHash:       txn.Hash,
				CallData:              *txn.CallData,
				TransactionSignature:  *txn.Signature,
				MaxFee:                txn.MaxFee,
				ContractAddress:       txn.ContractAddress,
				Version:               (*core.TransactionVersion)(txn.Version),
				EntryPointSelector:    txn.EntryPointSelector,
				Nonce:                 txn.Nonce,
				SenderAddress:         txn.SenderAddress,
				ResourceBounds:        adaptResourceBounds(txn.ResourceBounds),
				Tip:                   txn.Tip.Uint64(),
				PaymasterData:         *txn.PaymasterData,
				AccountDeploymentData: *txn.AccountDeploymentData,
				NonceDAMode:           core.DataAvailabilityMode(*txn.NonceDAMode),
				FeeDAMode:             core.DataAvailabilityMode(*txn.FeeDAMode),
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
			return fmt.Errorf("unsupported transaction type: %v", txn.Type)
		}
	}

	blockInfo := vm.BlockInfo{Header: &genesisHeader}
	executionResults, err := v.Execute(coreTxns, nil, []*felt.Felt{new(felt.Felt).SetUint64(1)},
		&blockInfo, genesisState, true, true, true, true, false, false)
	if err != nil {
		return fmt.Errorf("execute transactions: %v", err)
	}

	for i := range config.Txns {
		traceSD := vm2core.AdaptStateDiff(executionResults.Traces[i].StateDiff)
		genesisSD, _ := genesisState.StateDiffAndClasses()
		genesisSD.Merge(&traceSD)
		genesisState.SetStateDiff(&genesisSD)
	}

	return nil
}

func adaptResourceBounds(rb *rpc.ResourceBoundsMap) map[core.Resource]core.ResourceBounds {
	return map[core.Resource]core.ResourceBounds{
		core.ResourceL1Gas: {
			MaxAmount:       rb.L1Gas.MaxAmount.Uint64(),
			MaxPricePerUnit: rb.L1Gas.MaxPricePerUnit,
		},
		core.ResourceL2Gas: {
			MaxAmount:       rb.L2Gas.MaxAmount.Uint64(),
			MaxPricePerUnit: rb.L2Gas.MaxPricePerUnit,
		},
		core.ResourceL1DataGas: {
			MaxAmount:       rb.L1DataGas.MaxAmount.Uint64(),
			MaxPricePerUnit: rb.L1DataGas.MaxPricePerUnit,
		},
	}
}

func loadClasses(classes []string) (map[felt.Felt]core.ClassDefinition, error) {
	classMap := make(map[felt.Felt]core.ClassDefinition, len(classes))
	for _, classPath := range classes {
		bytes, err := os.ReadFile(classPath)
		if err != nil {
			return nil, fmt.Errorf("read class file: %v", err)
		}

		var response *starknet.ClassDefinition
		if err = json.Unmarshal(bytes, &response); err != nil {
			return nil, fmt.Errorf("unmarshal class: %v", err)
		}

		var class core.ClassDefinition
		if response.DeprecatedCairo != nil {
			if class, err = sn2core.AdaptDeprecatedCairoClass(response.DeprecatedCairo); err != nil {
				return nil, err
			}
		} else {
			var casmClass *starknet.CasmClass
			if casmClass, err = compiler.Compile(response.Sierra); err != nil {
				return nil, err
			}
			if class, err = sn2core.AdaptSierraClass(response.Sierra, casmClass); err != nil {
				return nil, err
			}
		}

		classhash, err := class.Hash()
		if err != nil {
			return nil, fmt.Errorf("calculate class hash: %v", err)
		}
		classMap[classhash] = class
	}
	return classMap, nil
}
