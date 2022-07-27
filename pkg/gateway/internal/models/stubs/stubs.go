// Package stubs implements mock models that are representative of the
// models package.
package stubs

import (
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/gateway/internal/models"
	"github.com/NethermindEth/juno/pkg/types"
)

// Stub represents a mock model.
type Stub struct{}

// block is a mock block that carries some values that appear in the
// first block.
var block = models.Block{
	Header: models.Header{
		Hash:   "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
		Number: 0,
		// XXX: Feeder response is also not prefixed with "0x".
		Root:   "021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6",
		Status: types.BlockStatusAcceptedOnL1,

		// TODO: Include the gas price.
		// Gas:    "0x0",
	},

	Transactions: []any{
		models.Deploy{
			Transaction: models.Transaction{
				Hash: "0xe0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75",
				Type: models.DeployTx,
			},
			Caller:    "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			ClassHash: "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
			Salt:      "0x546c86dc6e40a5e5492b782d8964e9a4274ff6ecb16d31eb09cee45a3564015",
			ConstructorCalldata: []string{
				"0x6cf6c2f36d36b08e591e4489e92ca882bb67b9c39a3afccf011972a8de467f0",
				"0x7ab344d88124307c07b56f6c59c12f4543e9c96398727854a322dea82c73240",
			},
		},
	},
}

// BlockByHash returns a Block corresponding to the hash given. The
// function is agnostic to whether the string has a "0x" prefix.
func (s *Stub) BlockByHash(hash string) (*models.Block, error) {
	if hash == "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943" {
		return &block, nil
	}
	return nil, models.ErrNotFound
}

// BlockByNumber returns a Block corresponding to the height num.
func (s *Stub) BlockByNumber(num uint64) (*models.Block, error) {
	if num == 0 {
		return &block, nil
	}
	return nil, models.ErrNotFound
}

// BlockHashByID returns the hash of the block corresponding to the
// given block number.
func (s *Stub) BlockHashByID(num uint64) (*felt.Felt, error) {
	if num == 0 {
		return new(felt.Felt).SetHex("0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943"), nil
	}
	return nil, models.ErrNotFound
}

// BlockIDByHash returns the block number of the block with the given
// hash.
func (s *Stub) BlockIDByHash(hash *felt.Felt) (uint64, error) {
	genesisHash := new(felt.Felt).SetHex("0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943")
	if hash.Cmp(genesisHash) == 0 {
		return 0, nil
	}
	return 0, models.ErrNotFound
}
