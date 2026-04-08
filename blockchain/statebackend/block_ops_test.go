package statebackend

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyBlockSuccession(t *testing.T) {
	h1 := felt.NewRandom[felt.Felt]()

	t.Run("empty chain rejects non-zero block number", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{Number: 10}}
		assert.EqualError(t, verifyBlockSuccession(memDB, block), "expected block #0, got block #10")
	})

	t.Run("empty chain rejects non-zero parent hash", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{ParentHash: h1}}
		assert.EqualError(t, verifyBlockSuccession(memDB, block),
			"block's parent hash does not match head block hash")
	})

	t.Run("rejects invalid version string", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{ProtocolVersion: "notasemver", ParentHash: &felt.Zero}}
		assert.Error(t, verifyBlockSuccession(memDB, block))
	})

	t.Run("rejects unsupported version that needs padding", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{ProtocolVersion: "99.0", ParentHash: &felt.Zero}}
		assert.ErrorContains(t, verifyBlockSuccession(memDB, block), "unsupported block version")
	})

	t.Run("rejects unsupported version that needs truncating", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{ProtocolVersion: "99.0.0.0", ParentHash: &felt.Zero}}
		assert.ErrorContains(t, verifyBlockSuccession(memDB, block), "unsupported block version")
	})

	t.Run("rejects version greater than supported", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{ProtocolVersion: "99.0.0", ParentHash: &felt.Zero}}
		assert.ErrorContains(t, verifyBlockSuccession(memDB, block), "unsupported block version")
	})

	t.Run("patch version mismatch is ignored", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{
			ProtocolVersion: core.LatestVer.IncPatch().String(),
			ParentHash:      &felt.Zero,
		}}
		assert.NoError(t, verifyBlockSuccession(memDB, block))
	})

	t.Run("rejects minor version mismatch", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{
			ProtocolVersion: core.LatestVer.IncMinor().String(),
			ParentHash:      &felt.Zero,
		}}
		assert.ErrorContains(t, verifyBlockSuccession(memDB, block), "unsupported block version")
	})

	t.Run("rejects major version mismatch", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{
			ProtocolVersion: core.LatestVer.IncMajor().String(),
			ParentHash:      &felt.Zero,
		}}
		assert.ErrorContains(t, verifyBlockSuccession(memDB, block), "unsupported block version")
	})

	t.Run("accepts empty version string", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{ParentHash: &felt.Zero}}
		assert.NoError(t, verifyBlockSuccession(memDB, block))
	})

	t.Run("empty chain accepts block 0 with zero parent hash", func(t *testing.T) {
		memDB := memory.New()
		block := &core.Block{Header: &core.Header{ParentHash: &felt.Zero}}
		assert.NoError(t, verifyBlockSuccession(memDB, block))
	})

	t.Run("rejects wrong block number after head", func(t *testing.T) {
		memDB := memory.New()
		storeTestHeader(t, memDB, &core.Header{Number: 0, Hash: h1, ParentHash: &felt.Zero})

		block := &core.Block{Header: &core.Header{Number: 10}}
		assert.EqualError(t, verifyBlockSuccession(memDB, block), "expected block #1, got block #10")
	})

	t.Run("rejects wrong parent hash after head", func(t *testing.T) {
		memDB := memory.New()
		storeTestHeader(t, memDB, &core.Header{Number: 0, Hash: h1, ParentHash: &felt.Zero})

		wrongParent := felt.NewRandom[felt.Felt]()
		block := &core.Block{Header: &core.Header{Number: 1, ParentHash: wrongParent}}
		assert.EqualError(t, verifyBlockSuccession(memDB, block),
			"block's parent hash does not match head block hash")
	})

	t.Run("accepts correct successor", func(t *testing.T) {
		memDB := memory.New()
		storeTestHeader(t, memDB, &core.Header{Number: 0, Hash: h1, ParentHash: &felt.Zero})

		block := &core.Block{Header: &core.Header{Number: 1, ParentHash: h1}}
		assert.NoError(t, verifyBlockSuccession(memDB, block))
	})
}

func storeTestHeader(t *testing.T, database *memory.Database, header *core.Header) {
	t.Helper()
	batch := database.NewBatch()

	require.NoError(t, core.WriteBlockHeader(batch, header))
	require.NoError(t, core.WriteChainHeight(batch, header.Number))
	require.NoError(t, batch.Write())
}
