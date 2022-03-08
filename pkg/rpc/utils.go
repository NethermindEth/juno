package rpc

import (
	"encoding/json"
	"fmt"
)

func StructPrinter(i interface{}) {
	// notest
	b, err := json.Marshal(i)
	if err != nil {
		return
	}
	fmt.Println(string(b))
}
