package rpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/testsource"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	bc := blockchain.New(pebble.NewMemTest(), utils.MAINNET)
	handler := rpc.NewHandler(bc, utils.MAINNET.ChainId())

	t.Run("starknet_chainId", func(t *testing.T) {
		cId, err := handler.ChainId()
		require.Nil(t, err)
		assert.Equal(t, utils.MAINNET.ChainId(), cId)
	})

	t.Run("empty bc - starknet_blockNumber", func(t *testing.T) {
		_, err := handler.BlockNumber()
		assert.Equal(t, rpc.ErrNoBlock, err)
	})
	t.Run("empty bc - starknet_blockHashAndNumber", func(t *testing.T) {
		_, err := handler.BlockNumberAndHash()
		assert.Equal(t, rpc.ErrNoBlock, err)
	})

	log := utils.NewNopZapLogger()
	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()
	synchronizer := sync.NewSynchronizer(bc, gw, log)
	ctx, canceler := context.WithCancel(context.Background())

	syncNodeChan := make(chan struct{})
	go func() {
		synchronizer.Run(ctx)
		close(syncNodeChan)
	}()

	time.Sleep(time.Second)

	t.Run("starknet_blockNumber", func(t *testing.T) {
		num, err := handler.BlockNumber()
		assert.Nil(t, err)
		assert.Equal(t, true, num > 0)
	})

	t.Run("starknet_blockHashAndNumber", func(t *testing.T) {
		hashAndNum, err := handler.BlockNumberAndHash()
		assert.Nil(t, err)
		assert.Equal(t, true, hashAndNum.Number > 0)
		gwBlock, gwErr := gw.BlockByNumber(ctx, hashAndNum.Number)
		assert.NoError(t, gwErr)
		assert.Equal(t, gwBlock.Hash, hashAndNum.Hash)
	})

	canceler()
	<-syncNodeChan
}
