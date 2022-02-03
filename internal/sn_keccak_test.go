package internal

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"testing"
)

func TestSnKeccak(t *testing.T) {
	derivedHash := hexutil.Encode(SnKeccak([]byte("hello")))
	// Original value: 0x1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8
	expectedHash := "0x008aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8"
	if derivedHash != expectedHash {
		t.Errorf("Got %q, expected %q", derivedHash, expectedHash)
	}

	derivedHash2 := hexutil.Encode(SnKeccak([]byte("hello world")))
	// Original value: 0x47173285a8d7341e5e972fc677286384f802f8ef42a5ec5f03bbfa254cb01fad
	expectedHash2 := "0x03173285a8d7341e5e972fc677286384f802f8ef42a5ec5f03bbfa254cb01fad"
	if derivedHash2 != expectedHash2 {
		t.Errorf("Got %q, expected %q", derivedHash2, expectedHash2)
	}

	derivedHash3 := hexutil.Encode(SnKeccak([]byte("constructor")))
	// Original value: 0x968ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194
	expectedHash3 := "0x028ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194"
	if derivedHash3 != expectedHash3 {
		t.Errorf("Got %q, expected %q", derivedHash3, expectedHash3)
	}
}
