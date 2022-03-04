package rpc

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func StructPrinter(i interface{}) {
	// notest
	b, err := json.Marshal(i)
	if err != nil {
		return
	}
	fmt.Println(string(b))
}
