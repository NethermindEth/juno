package health

import (
	"testing"
)

func TestGetLastBlock(t *testing.T) {
	hash, err := GetLastBlock()
	if err != nil {
		// notest
		panic(err)
	}
	// notest
	println(hash)
}
