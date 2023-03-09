package feeder

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

type ContractCode struct {
	Abi     interface{} `json:"abi"`
	Program *Program    `json:"program"`
}

func ProgramHash(contractDefinition *ClassDefinition) (*felt.Felt, error) {
	program := contractDefinition.Program

	// make debug info None
	program.DebugInfo = nil

	// Cairo 0.8 added "accessible_scopes" and "flow_tracking_data" attribute fields, which were
	// not present in older contracts. They present as null/empty for older contracts deployed
	// prior to adding this feature and should not be included in the hash calculation in these cases.
	//
	// We, therefore, check and remove them from the definition before calculating the hash.
	if program.Attributes != nil {
		attributes := program.Attributes.([]interface{})
		if len(attributes) == 0 {
			program.Attributes = nil
		} else {
			for key, attribute := range attributes {
				attributeInterface := attribute.(map[string]interface{})
				if attributeInterface["accessible_scopes"] == nil || len(attributeInterface["accessible_scopes"].([]interface{})) == 0 {
					delete(attributeInterface, "accessible_scopes")
				}

				if attributeInterface["flow_tracking_data"] == nil || len(attributeInterface["flow_tracking_data"].(map[string]interface{})) == 0 {
					delete(attributeInterface, "flow_tracking_data")
				}

				attributes[key] = attributeInterface
			}

			program.Attributes = attributes
		}
	}

	contractCode := new(ContractCode)
	contractCode.Abi = contractDefinition.Abi
	contractCode.Program = &program

	programBytes, err := contractCode.Marshal()
	if err != nil {
		return nil, err
	}

	programKeccak, err := crypto.StarknetKeccak(programBytes)
	if err != nil {
		return nil, err
	}

	return programKeccak, nil
}

// Marshal is a custom json marshaler for ContractCode
func (c ContractCode) Marshal() ([]byte, error) {
	contractCodeBytes, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	// reformat hints
	hint := c.Program.Hints
	newHint := reformatHint(hint)

	contractCodeString := string(contractCodeBytes)
	hintsIndex := strings.Index(contractCodeString, `"hints":`)            // find location of "hints" in contractCodeString
	identifierIndex := strings.Index(contractCodeString, `"identifiers":`) // find location of "identifiers" in contractCodeString
	// replace json "hints" with reformatted "hints"
	contractCodeString = contractCodeString[:(hintsIndex+8)] + newHint + contractCodeString[identifierIndex-1:]
	// reformat contractCodeString with
	contractCodeString = strings.ReplaceAll(contractCodeString, `,`, `, `)
	contractCodeString = strings.ReplaceAll(contractCodeString, `,  `, `, `)
	contractCodeString = strings.ReplaceAll(contractCodeString, `:`, `: `)
	contractCodeString = strings.ReplaceAll(contractCodeString, `:  `, `: `)

	buf := bytes.NewBufferString(contractCodeString)

	return buf.Bytes(), nil
}

// json marshals map with keys sorted alphabetically. This function
// reformats hints to be sorted numerically.
func reformatHint(hints Hints) string {
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)

	// sort hints
	keys := make([]uint64, 0, len(hints))
	for key := range hints {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	buf.WriteString(`{`)
	for key, hint := range keys {
		buf.WriteString(`"`)
		buf.WriteString(strconv.FormatUint(hint, 10))
		buf.WriteString(`":`)
		if err := encoder.Encode(hints[hint]); err != nil {
			panic(err)
		}
		// remove trailing newline
		buf.Truncate(buf.Len() - 1)
		if int(key) != len(keys)-1 {
			buf.WriteString(",")
		}
	}
	buf.WriteString(`}`)

	return buf.String()
}
