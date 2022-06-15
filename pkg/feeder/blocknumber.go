package feeder

import (
	"encoding/json"
	"fmt"
	"strings"
)

const pending = "pending"

// BlockNumber represents the `block_number` field in the StarkNet block
// structure. The BlockNumber is an int64, but in StarkNet the value can be an
// integer or the string "pending", so to check if the BlockNumber is pending
// the BlockNumber type implements the boolean function IsPending()
//
// See https://docs.starknet.io/docs/Blocks/header#block-header
type BlockNumber int64

func (x *BlockNumber) IsPending() bool {
	return int64(*x) == -1
}

func (x *BlockNumber) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	// Try to unmarshal as "pending"
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if strings.Compare(pending, s) != 0 {
			return fmt.Errorf("unexpected string value %s as a BlockNumber", s)
		}
		*x = BlockNumber(-1)
	} else {
		// Try to unmarshal as number
		var value int64
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if value < 0 {
			return fmt.Errorf("the block number must be greater than zero")
		}
		*x = BlockNumber(value)
	}
	return nil
}
