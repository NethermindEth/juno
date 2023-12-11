package blockbuilder

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/starknet"
)

// todo : move this logic when ready.

// BuildGenesisState loads the genesis state from the given path,
// extracts the classes and applys them to core.StateDiff.
func BuildGenesisStateDiff(path string) (*core.StateDiff, error) {
	var config GenesisConfig
	stateDiff := core.StateDiff{}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Errorf("read genesis file: %v", err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		fmt.Errorf("unmarshal genesis: %v", err)
	}

	// Extact the classes from config
	classes, err := loadNewClasses(config.Classes)
	if err != nil {
		fmt.Errorf("load classes: %v", err)
	}

	// Insert classes into the state diff
	for _, class := range classes {
		if class.Version() == 0 {
			classHash, err := class.Hash()
			if err != nil {
				fmt.Errorf("class.Hash(): %v", err)
			}
			stateDiff.DeclaredV0Classes = append(stateDiff.DeclaredV0Classes, classHash) // Doesn't do anything?
		}
		if class.Version() == 1 {
			classHash, err := class.Hash()
			if err != nil {
				fmt.Errorf("class.Hash(): %v", err)
			}

			compiledClassHash, err := class.CompiledClassHash() // Todo
			if err != nil {
				fmt.Errorf("get compiled class hash: %v", err)
			}
			stateDiff.DeclaredV1Classes[*classHash] = compiledClassHash // Doesn't do anything?
		}
	}

	// Calculate deployedContracts (need to calculate the address from calling the constructor)..

	// Calculate Storage Diffs when StateWriter is ready

	panic("to finish")
	return nil, nil

}

// Need this for b.Store() and to calculate StateDiff
func loadNewClasses(classPaths []string) ([]core.Class, error) {
	classes := make([]core.Class, len(classPaths))
	for _, classPath := range classPaths {
		data, err := ioutil.ReadFile(classPath)
		if err != nil {
			fmt.Errorf("load class: %v", err)
		}

		class, err := adaptFeederClass2Core(data)
		if err != nil {
			fmt.Errorf("adapt class to core type: %v", err)
		}
		classes = append(classes, class)
	}
	return classes, nil
}

func adaptFeederClass2Core(declaredClass json.RawMessage) (core.Class, error) {
	var feederClass starknet.ClassDefinition
	err := json.Unmarshal(declaredClass, &feederClass)
	if err != nil {
		return nil, err
	}

	switch {
	case feederClass.V1 != nil:
		return sn2core.AdaptCairo1Class(feederClass.V1, nil)
	case feederClass.V0 != nil:
		return sn2core.AdaptCairo0Class(feederClass.V0)
	default:
		return nil, errors.New("empty class")
	}
}
