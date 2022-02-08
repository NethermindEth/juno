package chainid

import "testing"

func TestEncodeMainnetChainId(t *testing.T) {
	mainnetChainId := EncodeChainId("SN_MAIN")
	expectedMainnetChainId := "0x534e5f4d41494e"
	if mainnetChainId != expectedMainnetChainId {
		t.Errorf("Got %q, expected %q", mainnetChainId, expectedMainnetChainId)
	}
}

func TestEncodeGoerliId(t *testing.T) {
	goerliChainId := EncodeChainId("SN_GOERLI")
	expectedGoerliChainId := "0x534e5f474f45524c49"
	if goerliChainId != expectedGoerliChainId {
		t.Errorf("Got %q, expected %q", goerliChainId, expectedGoerliChainId)
	}
}

func BenchmarkEncodeMainnetChainId(b *testing.B) {
	for i := 0; i < b.N; i++ {
		EncodeChainId("SN_MAIN")
	}
}

func BenchmarkEncodeGoerliChainId(t *testing.B) {
	goerliChainId := EncodeChainId("SN_GOERLI")
	expectedGoerliChainId := "0x534e5f474f45524c49"
	if goerliChainId != expectedGoerliChainId {
		t.Errorf("Got %q, expected %q", goerliChainId, expectedGoerliChainId)
	}
}
