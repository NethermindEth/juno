package rpc

import (
	"encoding/json"
	"fmt"
)

type Dispatcher struct {
	handlers []HandleParamsResulter
}

func (us *Dispatcher) MethodName(h HandleParamsResulter) string {
	return h.Name()
}

func (us *Dispatcher) Handlers() []HandleParamsResulter {
	return us.handlers
}

func NewRPCDispatcher(handlers []HandleParamsResulter) *Dispatcher {
	return &Dispatcher{handlers: handlers}
}

func StructPrinter(i interface{}) {
	b, err := json.Marshal(i)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(b))
}
