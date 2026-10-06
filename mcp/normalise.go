package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/core/crypto"
)

// normaliser rewrites a lenient argument value into the form the JSON-RPC method expects. Values
// it doesn't recognise are returned unchanged, for the JSON-RPC method to validate.
type normaliser func(json.RawMessage) json.RawMessage

// entryPointName matches the name of a Cairo function, which is never a valid felt.
var entryPointName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// normaliseBlockID accepts, on top of the JSON-RPC block ids, a block number given as a JSON
// number or a decimal string, a block hash given as a 0x-prefixed string, and the "pending" tag
// of older JSON-RPC versions.
func normaliseBlockID(raw json.RawMessage) json.RawMessage {
	var id any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&id); err != nil {
		return raw
	}

	switch id := id.(type) {
	case json.Number:
		return blockNumber(id.String(), raw)
	case string:
		switch {
		case id == "pending":
			return json.RawMessage(`"pre_confirmed"`)
		case strings.HasPrefix(id, "0x"):
			blockHash, err := json.Marshal(map[string]string{"block_hash": id})
			if err != nil {
				return raw
			}
			return blockHash
		default:
			return blockNumber(id, raw)
		}
	default:
		return raw
	}
}

func blockNumber(number string, raw json.RawMessage) json.RawMessage {
	n, err := strconv.ParseUint(number, 10, 64)
	if err != nil {
		return raw
	}
	return fmt.Appendf(nil, `{"block_number":%d}`, n)
}

// normaliseSelector replaces an entry point given by name with its selector, which models can't
// compute themselves.
func normaliseSelector(raw json.RawMessage) json.RawMessage {
	var name string
	if err := json.Unmarshal(raw, &name); err != nil || !entryPointName.MatchString(name) {
		return raw
	}
	selector := crypto.StarknetKeccak([]byte(name))
	return json.RawMessage(strconv.Quote(selector.String()))
}

// fieldNormaliser returns a normaliser applying normalise to the given fields of a JSON object.
func fieldNormaliser(normalise normaliser, fields ...string) normaliser {
	return func(raw json.RawMessage) json.RawMessage {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return raw
		}
		for _, field := range fields {
			if value, ok := object[field]; ok {
				object[field] = normalise(value)
			}
		}
		normalised, err := json.Marshal(object)
		if err != nil {
			return raw
		}
		return normalised
	}
}
