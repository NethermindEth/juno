//go:build amd64 || arm64

package jsonbench

import "github.com/bytedance/sonic"

// sonicLib is only available on amd64/arm64 where sonic's JIT compiler works.
var sonicLib = &JSONLibrary{
	Name:      "sonic",
	Marshal:   sonic.Marshal,
	Unmarshal: sonic.Unmarshal,
}
