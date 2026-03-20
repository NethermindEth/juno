package p2p2consensus

import (
	"errors"

	p2pconsensus "github.com/starknet-io/starknet-p2p-specs/p2p/proto/consensus/consensus"
)

func validateProposalInit(p *p2pconsensus.ProposalInit) error {
	if p == nil {
		return errors.New("proposal init is nil")
	}
	if p.Proposer == nil {
		return errors.New("proposer must be set")
	}
	return nil
}

func validateBlockInfo(b *p2pconsensus.BlockInfo) error {
	if b == nil {
		return errors.New("block info is nil")
	}
	if b.Builder == nil {
		return errors.New("builder must be set")
	}
	if b.L2GasPriceFri == nil {
		return errors.New("l2_gas_price_fri must be set")
	}
	if b.L1GasPriceWei == nil {
		return errors.New("l1_gas_price_wei must be set")
	}
	if b.L1DataGasPriceWei == nil {
		return errors.New("l1_data_gas_price_wei must be set")
	}
	if b.EthToStrkRate == nil {
		return errors.New("eth_to_strk_rate must be set")
	}
	return nil
}

func validateConsensusTransaction(tx *p2pconsensus.ConsensusTransaction) error {
	if tx == nil {
		return errors.New("consensus transaction is nil")
	}
	// Todo: consider further validation here
	if tx.Txn == nil {
		return errors.New("txn must be set to a valid variant")
	}
	if tx.TransactionHash == nil {
		return errors.New("transaction_hash must be set")
	}
	return nil
}

func validateProposalCommitment(p *p2pconsensus.ProposalCommitment) error { //nolint:gocyclo // simple, repetative code
	if p == nil {
		return errors.New("proposal commitment is nil")
	}
	if p.ParentCommitment == nil {
		return errors.New("parent_commitment must be set")
	}
	if p.Builder == nil {
		return errors.New("builder must be set")
	}
	if p.OldStateRoot == nil {
		return errors.New("old_state_root must be set")
	}
	if p.VersionConstantCommitment == nil {
		return errors.New("version_constant_commitment must be set")
	}
	if p.StateDiffCommitment == nil {
		return errors.New("state_diff_commitment must be set")
	}
	if p.TransactionCommitment == nil {
		return errors.New("transaction_commitment must be set")
	}
	if p.EventCommitment == nil {
		return errors.New("event_commitment must be set")
	}
	if p.ReceiptCommitment == nil {
		return errors.New("receipt_commitment must be set")
	}
	if p.ConcatenatedCounts == nil {
		return errors.New("concatenated_counts must be set")
	}
	if p.L1GasPriceFri == nil {
		return errors.New("l1_gas_price_fri must be set")
	}
	if p.L1DataGasPriceFri == nil {
		return errors.New("l1_data_gas_price_fri must be set")
	}
	if p.L2GasPriceFri == nil {
		return errors.New("l2_gas_price_fri must be set")
	}
	if p.L2GasUsed == nil {
		return errors.New("l2_gas_used must be set")
	}
	if p.NextL2GasPriceFri == nil {
		return errors.New("next_l2_gas_price_fri must be set")
	}
	return nil
}

func validateProposalFin(p *p2pconsensus.ProposalFin) error {
	if p == nil {
		return errors.New("proposal fin is nil")
	}
	if p.ProposalCommitment == nil {
		return errors.New("proposal_commitment must be set")
	}
	return nil
}
