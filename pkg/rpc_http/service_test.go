package rpcnew

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

type A struct {
	A int64 `json:"aa"`
	B int64
}

func TestObjectParam_Decode(t *testing.T) {
	params := []byte("{\"aa\":1}")
	objectParams := ObjectParam(params)
	a := A{}
	aT := reflect.TypeOf(a)
	param, err := objectParams.Decode(aT)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%T: %v\n", param, param)
}

func Rpc1(ctx context.Context, param *A) (interface{}, error) {
	fmt.Println(param)
	return param, nil
}

func TestRegisterFunction(t *testing.T) {
	f, err := NewRpcFunction(Rpc1)
	if err != nil {
		t.Error(err)
	}
	params := []byte("{\"aa\":1}")
	objectParams := ObjectParam(params)
	fmt.Println(f.Call(objectParams))
}
