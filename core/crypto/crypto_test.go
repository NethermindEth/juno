package crypto

import "testing"

func TestHash(t *testing.T) {
	res, err := hash(
		"0x03d937c035c878245caf64531a5756109c53068da139362728feb561405371cb",
		"0x0208a0a10250e382e1e4bbe2880906c2791bf6275695e02fbbc6aeff9cd8b31a")
	if err != nil {
		t.Errorf("expected no error but got %s.", err)
	}

	expectedHash := "0x030e480bed5fe53fa909cc0f8c4d99b8f9f2c016be4c41e13a4848797979c662"
	if res != expectedHash {
		t.Errorf("Hash error: expected %s but got %s.", expectedHash, res)
	}
}

func TestGetPublicKey(t *testing.T) {
	res, err := publicKey(
		"0x03c1e9550e66958296d11b60f8e8e7a7ad990d07fa65d5f7652c4a6c87d4e3cc")
	if err != nil {
		t.Errorf("expected no error but got %s.", err)
	}

	expectedKey := "0x077a3b314db07c45076d11f62b6f9e748a39790441823307743cf00d6597ea43"
	if res != expectedKey {
		t.Errorf("GetPublicKey error: expected %s but got %s.", expectedKey, res)
	}

	res, err = publicKey("0x12")
	if err != nil {
		t.Errorf("expected no error but got %s.", err)
	}

	expectedKey = "0x019661066e96a8b9f06a1d136881ee924dfb6a885239caa5fd3f87a54c6b25c4"
	if res != expectedKey {
		t.Errorf("GetPublicKey error: expected %s but got %s.", expectedKey, res)
	}
}

func TestVerify(t *testing.T) {
	res, err := verify(
		"0x1ef15c18599971b7beced415a40f0c7deacfd9b0d1819e03d723d8bc943cfca",
		"0x2",
		"0x411494b501a98abd8262b0da1351e17899a0c4ef23dd2f96fec5ba847310b20",
		"0x405c3191ab3883ef2b763af35bc5f5d15b3b4e99461d70e84c654a351a7c81b")
	if err != nil {
		t.Errorf("expected no error but got %s.", err)
	}
	if !(res) {
		t.Errorf("Verify error: valid message was not verified correctly.")
	}

	res, err = verify(
		"0x077a4b314db07c45076d11f62b6f9e748a39790441823307743cf00d6597ea43",
		"0x0397e76d1667c4454bfb83514e120583af836f8e32a516765497823eabe16a3f",
		"0x0173fd03d8b008ee7432977ac27d1e9d1a1f6c98b1a2f05fa84a21c84c44e882",
		"0x01f2c44a7798f55192f153b4c48ea5c1241fbb69e6132cc8a0da9c5b62a4286e")
	if err != nil {
		t.Errorf("expected no error but got %s.", err)
	}
	if res {
		t.Errorf("Verify error: invalid message was not rejected.")
	}
}

func TestSign(t *testing.T) {
	r, s, err := sign("0x1", "0x2", "0x3")
	if err != nil {
		t.Errorf("expected no error but got %s.", err)
	}
	key, err := publicKey("0x1")
	if err != nil {
		t.Errorf("expected no error but got %s.", err)
	}
	res, err := verify(key, "0x2", r, s)
	if err != nil {
		t.Errorf("expected no error but got %s.", err)
	}
	if !(res) {
		t.Errorf("Sign error: signature rejected by verification.")
	}
}
