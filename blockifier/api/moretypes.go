package api

import "github.com/NethermindEth/juno/core/felt"

// BlockContext contains the context for block execution.
type BlockContext struct {
	BlockInfo          BlockInfo
	ChainInfo          ChainInfo
	VersionedConstants VersionedConstants
	BouncerConfig      BouncerConfig
}

// BlockInfo contains information about the current block.
type BlockInfo struct {
	BlockNumber      uint64
	BlockTimestamp   uint64
	SequencerAddress felt.Felt
	GasPrices        GasPrices
	// UseKZGDA indicates whether to use KZG for data availability
	UseKZGDA bool
}

type ChainInfo struct {
	ChainID     string
	ETHAddress  felt.Felt
	STRKAddress felt.Felt
}

// BouncerWeights specifies weights for transaction resource limits.
type BouncerWeights struct {
	// Todo
}

// BouncerConfig contains configuration for the transaction bouncer.
type BouncerConfig struct {
	// BlockMaxCapacity specifies the maximum capacity per block
	BlockMaxCapacity BouncerWeights
}

// VersionedConstants contains protocol constants specific to versions.
type VersionedConstants struct {
	// (Big) Todo
}
