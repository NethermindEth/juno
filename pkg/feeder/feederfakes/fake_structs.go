package feederfakes

// notest
import (
	abi "github.com/NethermindEth/juno/pkg/feeder/abi"
)

//returns ABI with full coverage

func ReturnAbiInfo_Full() *abi.Abi {
	_json := "[{\"inputs\": [{\"name\": \"funct\", \"type\": \"felt\"}], \"name\": \"function-custom\", \"outputs\": [], \"type\": \"function\"}, {\"inputs\": [{\"name\": \"implementation\", \"type\": \"felt\"}], \"name\": \"L1Handler-custom\", \"outputs\": [], \"type\": \"l1_handler\"}, {\"members\": [{\"offset\": 1, \"name\": \"member\", \"type\": \"struct\"}], \"name\": \"Struct-custom\", \"size\": 3, \"type\": \"struct\"}, {\"inputs\": [{\"name\": \"constr\", \"type\": \"felt\"}], \"name\": \"constructor-custom\", \"outputs\": [], \"type\": \"constructor\"}, {\"data\": [{\"name\": \"storage_cells_len\", \"type\": \"felt\"}, {\"name\": \"storage_cells\", \"type\": \"StorageCell*\"}], \"keys\": [], \"name\": \"log_storage_cells\", \"type\": \"event\"}]"
	bjson := []byte(_json)
	var a abi.Abi
	a.UnmarshalAbiJSON(bjson)

	return &a
}

// returns ABI with full coverage
func ReturnAbiInfo_Fail() error {
	_json := "[{\"inputs\": [{\"name\": \"funct\", \"type\": \"felt\"}], \"name\": \"function-custom\", \"outputs\": [], \"type\": \"function\"}, {\"inputs\": [{\"name\": \"implementation\", \"type\": \"unknown\"}], \"name\": \"L1Handler-custom\", \"outputs\": [], \"type\": \"l1_handler\"}, {\"members\": [{\"offset\": 1, \"name\": \"member\", \"type\": \"struct\"}], \"name\": \"Struct-custom\", \"size\": 3, \"type\": \"struct\"}, {\"inputs\": [{\"name\": \"constr\", \"type\": \"felt\"}], \"name\": \"constructor-custom\", \"outputs\": [], \"type\": \"constructor\"}, {\"data\": [{\"name\": \"storage_cells_len\", \"type\": \"felt\"}, {\"name\": \"storage_cells\", \"type\": \"StorageCell*\"}], \"keys\": [], \"name\": \"log_storage_cells\", \"type\": \"unknown\"}]"
	bjson := []byte(_json)
	var a abi.Abi
	err := a.UnmarshalAbiJSON(bjson)

	return err
}
