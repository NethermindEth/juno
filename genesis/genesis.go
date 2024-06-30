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
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/validator"
	"github.com/NethermindEth/juno/vm"
)

type GenesisConfig struct {
	Classes       []string                          `json:"classes"`        // []path-to-class.json
	Contracts     map[felt.Felt]GenesisContractData `json:"contracts"`      // address -> {classHash, constructorArgs}
	FunctionCalls []FunctionCall                    `json:"function_calls"` // list of functionCalls to Call()
	BootstrapAccounts []Account // accounts to prefund with strk token
}

type Account struct {
	PubKey felt.Felt
	PrivKey felt.Felt
	Address felt.Felt
}

func Read(path string) (*GenesisConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config GenesisConfig
	if err = json.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	return &config, err
}

type GenesisContractData struct {
	ClassHash       felt.Felt   `json:"class_hash"`
	ConstructorArgs []felt.Felt `json:"constructor_args"`
}

type FunctionCall struct {
	ContractAddress    felt.Felt   `json:"contract_address"`
	EntryPointSelector felt.Felt   `json:"entry_point_selector"`
	Calldata           []felt.Felt `json:"calldata"`
	CallerAddress      felt.Felt `json:"caller_address"`
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
			maxSteps := uint64(100000) //nolint:gomnd
			// Call the constructors
			if _, err = v.Call(callInfo, &blockInfo, genesisState, network, maxSteps, false); err != nil {
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
			CallerAddress:   &fnCall.CallerAddress,
		}
		blockInfo := vm.BlockInfo{
			Header: &core.Header{
				Number:    0,
				Timestamp: 0,
			},
		}
		maxSteps := uint64(100000) //nolint:gomnd
		if _, err = v.Call(callInfo, &blockInfo, genesisState, network, maxSteps, false); err != nil {
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
		} else if compiledClass, cErr := starknet.Compile(response.V1); cErr != nil {
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



func GenesisConfigAccountsTokens(initMintAmnt felt.Felt, classes []string) GenesisConfig {

	strToFelt := func(feltStr string) felt.Felt{
		felt,_:= new(felt.Felt).SetString(feltStr)		
		return *felt
	}

	// accounts to deploy and prefund
	accounts:=[]Account{
		{
			PubKey: strToFelt("0x16d03d341717ab11083a481f53278d8e54f610af815cbdab4035b2df283fcc0"),
			PrivKey: strToFelt("0x2bff1b26236b72d8a930be1dfbee09f79a536a49482a4c8b8f1030e2ab3bf1b"),
			Address: strToFelt("0x101"),
		},
		{
			PubKey: strToFelt("0x3bc7ab4ca475e24a0053db47c3e5a2a53264a30639d8b2bd0a08407da3ca0c"),
			PrivKey: strToFelt("0x43d8de30e55ed83b4436aea47e7517d4a52d06912938e2887cb1d33518daef1"),
			Address: strToFelt("0x102"),
		},
	}
	// strk params
	whyIsThisNeeded := new(felt.Felt).SetUint64(0) // Buffer for self parameter??
	permissionedMinter := strToFelt("0x123456")
	initialSupply:=new(felt.Felt).Mul(&initMintAmnt,new(felt.Felt).SetUint64(uint64(len(accounts))+100))
	strkAddress := strToFelt("0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d") 
	strkClassHash :=strToFelt("0x04ad3c1dc8413453db314497945b6903e1c766495a1e60492d44da9c2a986e4b")	
	strkConstrcutorArgs := []felt.Felt{
		strToFelt("0x537461726b6e657420546f6b656e"), 	// 1 name, felt
		strToFelt("0x5354524b"), 						// 2 symbol, felt
		strToFelt("0x12"), 								// 3 decimals, u8
		*initialSupply,									// 4 initial_supply, u256
		permissionedMinter, 							// 5 recipient, ContractAddress
		permissionedMinter, 							// 6 permitted_minter, ContractAddress
		permissionedMinter,	 							// 7 provisional_governance_admin, ContractAddress
		strToFelt("0x1"), 								// 8 upgrade_delay, u128
		*whyIsThisNeeded, 								// Todo: ? ref self: ContractState ?
	} 

	// account params
	simpleAccountClassHash := strToFelt("0x04c6d6cf894f8bc96bb9c525e6853e5483177841f7388f74a46cfda6f028c755") 

	genesisConfig := GenesisConfig{
		Classes: classes,
		Contracts: map[felt.Felt]GenesisContractData{
			strkAddress: {
				ClassHash: strkClassHash,
				ConstructorArgs: strkConstrcutorArgs,
			},
		},	
	}


	// deploy accounts
	for _, acnt:=range accounts{			
		genesisConfig.Contracts[acnt.Address]=GenesisContractData{
				ClassHash: simpleAccountClassHash,
				ConstructorArgs: []felt.Felt{acnt.PubKey},
			}
	}

	// fund accounts with strk token
	for _, acnt:=range accounts{			
		genesisConfig.FunctionCalls = append(genesisConfig.FunctionCalls,
			FunctionCall{
				ContractAddress:    strkAddress,
				EntryPointSelector: strToFelt("0x0083afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"), // transfer
				Calldata:           []felt.Felt{acnt.Address, initMintAmnt, *whyIsThisNeeded},       			 // todo
				CallerAddress: permissionedMinter,
		})			
	}

	genesisConfig.BootstrapAccounts=accounts

	return genesisConfig
}