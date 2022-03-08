package rpc

import (
	"encoding/json"
	"fmt"
)

// TODO: Document.
func StructPrinter(i interface{}) {
	// notest
	b, err := json.Marshal(i)
	if err != nil {
		return
	}
	fmt.Println(string(b))
}
