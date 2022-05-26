package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/block"
	"google.golang.org/protobuf/proto"
	"math/big"
	"testing"
)

func TestService(t *testing.T) {
	blocks := []*block.Block{
		{
			Hash:             fromHexString("43950c9e3565cba1f2627b219d4863380f93a8548818ce26019d1bd5eebb0fb"),
			BlockNumber:      2175,
			ParentBlockHash:  fromHexString("f8fe26de3ce9ee4d543b1152deb2ce549e589524d79598227761d6006b74a9"),
			Status:           "ACCEPTED_ON_L2",
			SequencerAddress: fromHexString("0"),
			GlobalStateRoot:  fromHexString("6a42d697b5b735eef03bb71841ed5099d57088f7b5eec8e356fe2601d5ba08f"),
			OldRoot:          fromHexString("1d932dcf7da6c4f7605117cf514d953147161ab2d8f762dcebbb6dad427e519"),
			AcceptedTime:     1652492749,
			TimeStamp:        1652488132,
			TxCount:          2,
			TxCommitment:     fromHexString("0"),
			EventCount:       19,
			EventCommitment:  fromHexString("0"),
			TxHashes: [][]byte{
				fromHexString("5ce76214481ebb29f912cb5d31abdff34fd42217f5ece9dda76d9fcfd62dc73"),
				fromHexString("4ff16b7673da1f4c4b114d28e0e1a366bd61b702eca3e21882da6c8939e60a2"),
			},
		},
	}
	BlockService.Setup(db.NewKeyValueDb(t.TempDir(), 0))
	err := BlockService.Run()
	if err != nil {
		t.Errorf("error starting the service: %s", err)
	}
	for _, b := range blocks {
		key := b.Hash
		BlockService.StoreBlock(key, b)
		returnedBlock := BlockService.GetBlock(key)
		if returnedBlock == nil {
			t.Errorf("unexpected nil after search for block with hash %s", hex.EncodeToString(b.Hash))
		}
		if !equalData(t, b, returnedBlock) {
			t.Errorf("b")
		}
	}
	BlockService.Close(context.Background())
}

func equalData(t *testing.T, a, b *block.Block) bool {
	aData, err := proto.Marshal(a)
	if err != nil {
		t.Errorf("marshal error: %s", err)
	}
	bData, err := proto.Marshal(b)
	if err != nil {
		t.Errorf("marshal error: %s", err)
	}
	return bytes.Compare(aData, bData) == 0
}

func fromHexString(s string) []byte {
	number, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic(any(fmt.Errorf("error pasing hex-string: %s", s)))
	}
	return number.Bytes()
}
