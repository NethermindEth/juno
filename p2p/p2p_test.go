package p2p

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
)

func TestP2PImpl_Smoke(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := pebble.NewMem()
	if err != nil {
		t.Fatalf("Error opening memdb %s", err)
	}

	bc := blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger())
	pimpl, err := New(bc, "", "", utils.NewNopZapLogger())
	if err != nil {
		t.Fatalf("Error settinp up p2p %s", err)
	}

	err = pimpl.Run(ctx)
	if err != nil {
		t.Fatalf("Error starting p2p %s", err)
	}
}
