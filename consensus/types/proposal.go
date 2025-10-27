package types

import (
	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type Transaction struct {
	Transaction core.Transaction
	Class       core.ClassDefinition
	PaidFeeOnL1 *felt.Felt
}

type ProposalFin felt.Felt

type ValueID felt.Felt

type BlockInfo struct {
	BlockNumber       uint64
	Builder           felt.Felt
	Timestamp         uint64
	L2GasPriceFRI     felt.Felt
	L1GasPriceWEI     felt.Felt
	L1DataGasPriceWEI felt.Felt
	EthToStrkRate     felt.Felt
	L1DAMode          core.L1DAMode
}

type ProposalInit struct {
	BlockNum   Height
	Round      Round
	ValidRound Round
	Proposer   felt.Felt
}

type ProposalCommitment struct {
	BlockNumber uint64
	Builder     felt.Felt

	// We must set these by hand. They will be compared against ProposalCommitment
	ParentCommitment felt.Felt
	Timestamp        uint64
	ProtocolVersion  semver.Version

	// These also need set by hand. However, we would need to update the DB
	// and blockchain Reader interface, so they are ignored for now.
	OldStateRoot              felt.Felt
	VersionConstantCommitment felt.Felt
	NextL2GasPriceFRI         felt.Felt // If empty proposal, use last value

	// These values may be zero for empty proposals
	StateDiffCommitment   felt.Felt
	TransactionCommitment felt.Felt
	EventCommitment       felt.Felt
	ReceiptCommitment     felt.Felt
	ConcatenatedCounts    felt.Felt
	L1GasPriceFRI         felt.Felt
	L1DataGasPriceFRI     felt.Felt
	L2GasPriceFRI         felt.Felt
	L2GasUsed             felt.Felt
	L1DAMode              core.L1DAMode
}
