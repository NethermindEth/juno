package internal

import "testing"

func TestEncodeChainId(t *testing.T) {
	mainnetChainId := EncodeChainId("SN_MAIN")
	expectedMainnetChainId := "0x534e5f4d41494e"
	if mainnetChainId != expectedMainnetChainId {
			t.Errorf("Got %q, expected %q", mainnetChainId, expectedMainnetChainId)
	}

	goerliChainId := EncodeChainId("SN_GOERLI")
	expectedGoerliChainId := "0x534e5f474f45524c49"
	if goerliChainId != expectedGoerliChainId {
			t.Errorf("Got %q, expected %q", goerliChainId, expectedGoerliChainId)
	}
}
