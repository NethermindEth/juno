package vm

import (
	"encoding/json"

	"github.com/NethermindEth/juno/core"
)

func marshalContractInfo(declaredClass core.Class) (json.RawMessage, error) {
	contractInfo := struct {
		ABILength           uint32 `json:"abi_length"`
		SierraProgramLength uint32 `json:"sierra_program_length"`
	}{ABILength: 0, SierraProgramLength: 0}

	switch c := (declaredClass).(type) {
	case *core.Cairo0Class:
		contractInfo.ABILength = uint32(len(string(c.Abi)))
	case *core.Cairo1Class:
		contractInfo.ABILength = uint32(len(c.Abi))
		contractInfo.SierraProgramLength = uint32(len(c.Program))
	}

	result, err := json.Marshal(contractInfo)
	if err != nil {
		return nil, err
	}
	return result, nil
}
