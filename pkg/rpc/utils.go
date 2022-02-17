package rpc

import (
	"encoding/json"
	"fmt"
)

func StructPrinter(i interface{}) {
	b, err := json.Marshal(i)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
}
