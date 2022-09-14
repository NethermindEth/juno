package types

import (
	"encoding/json"
	"errors"
	"math/big"
	"sort"

	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/NethermindEth/juno/internal/utils"

	"github.com/NethermindEth/juno/pkg/felt"
)

type ContractClass struct {
	Program           string
	EntryPointsByType EntryPoints
	Abi               string
}

func NewContractClass(program json.RawMessage, entryPoints EntryPoints, abi json.RawMessage) (*ContractClass, error) {
	gzipProgram, err := utils.CompressGzipB64(program)
	if err != nil {
		return nil, err
	}
	gzipAbi, err := utils.CompressGzipB64(abi)
	if err != nil {
		return nil, err
	}
	c := ContractClass{
		Program:           gzipProgram,
		EntryPointsByType: entryPoints,
		Abi:               gzipAbi,
	}
	c.EntryPointsByType.Sort()
	return &c, nil
}

func NewContractClassFromFeeder(fullContract *feeder.FullContract) (*ContractClass, error) {
	constructor, err := NewEntryPointListFromFeeder(fullContract.Entrypoints.Constructor)
	if err != nil {
		return nil, err
	}
	external, err := NewEntryPointListFromFeeder(fullContract.Entrypoints.External)
	if err != nil {
		return nil, err
	}
	l1Handler, err := NewEntryPointListFromFeeder(fullContract.Entrypoints.L1Handler)
	if err != nil {
		return nil, err
	}
	entryPoints := EntryPoints{
		Constructor: constructor,
		External:    external,
		L1Handler:   l1Handler,
	}
	return NewContractClass(fullContract.Program, entryPoints, fullContract.Abi)
}

type EntryPoints struct {
	Constructor EntryPoiintList `json:"CONSTRUCTOR"`
	External    EntryPoiintList `json:"EXTERNAL"`
	L1Handler   EntryPoiintList `json:"L1_HANDLER"`
}

func (e *EntryPoints) Sort() {
	e.Constructor.Sort()
	e.External.Sort()
	e.L1Handler.Sort()
}

type EntryPoiintList []*ContractEntryPoint

func NewEntryPointListFromFeeder(entryPoints []feeder.EntryPoint) (EntryPoiintList, error) {
	contractEntryPoints := make([]*ContractEntryPoint, 0, len(entryPoints))
	for _, entryPoint := range entryPoints {
		offset, ok := new(big.Int).SetString(remove0x(entryPoint.Offset), 16)
		if !ok {
			return nil, errors.New("error parsing entry point offset")
		}
		contractEntryPoints = append(contractEntryPoints, &ContractEntryPoint{
			Offset:   offset,
			Selector: new(felt.Felt).SetHex(entryPoint.Selector),
		})
	}
	return contractEntryPoints, nil
}

func (e EntryPoiintList) Sort() {
	sort.Slice(e, func(i, j int) bool {
		return e[i].Selector.Cmp(e[j].Selector) < 0
	})
}

type ContractEntryPoint struct {
	Offset   *big.Int   `json:"offset"`
	Selector *felt.Felt `json:"selector"`
}

func (c *ContractEntryPoint) MarshalJSON() ([]byte, error) {
	data := make(map[string]interface{}, 2)
	if c.Offset != nil {
		data["offset"] = "0x" + c.Offset.Text(16)
	}
	if c.Selector != nil {
		data["selector"] = "0x" + c.Selector.Text(16)
	}
	return json.Marshal(data)
}

func remove0x(s string) string {
	if len(s) > 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}
