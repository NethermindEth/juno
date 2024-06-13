package sync

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClassRange(t *testing.T) {

	var d db.DB
	d, _ = pebble.New("/home/amirul/fastworkscratch3/juno_db/juno_mainnet", 128000000, 128, utils.NewNopZapLogger())
	bc := blockchain.New(d, &utils.Mainnet) // Needed because class loader need encoder to be registered

	_, err := utils.NewZapLogger(utils.DEBUG, false)
	assert.NoError(t, err)

	b, err := bc.Head()
	assert.NoError(t, err)

	fmt.Printf("headblock %d\n", b.Number)

	stateRoot := b.GlobalStateRoot

	server := &snapServer{
		blockchain: bc,
	}

	// err = syncer.Run(context.Background())
	assert.NoError(t, err)

	startRange := (&felt.Felt{}).SetUint64(0)

	server.GetClassRange(context.Background(),
		&spec.ClassRangeRequest{
			Root:           core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptHash(startRange),
			ChunksPerProof: 100,
		})(func(result *ClassRangeStreamingResult, err error) bool {
		if err != nil {
			fmt.Printf("err %s\n", err)
		}

		return true
	})

}
