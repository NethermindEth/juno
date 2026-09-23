package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/starknet"
)

const preConfirmedStatus = "PRE_CONFIRMED"

type blockHeader struct {
	Version          json.RawMessage `json:"starknet_version,omitempty"`
	Timestamp        json.RawMessage `json:"timestamp,omitempty"`
	SequencerAddress json.RawMessage `json:"sequencer_address,omitempty"`
	L1GasPrice       json.RawMessage `json:"l1_gas_price,omitempty"`
	L2GasPrice       json.RawMessage `json:"l2_gas_price,omitempty"`
	L1DataGasPrice   json.RawMessage `json:"l1_data_gas_price,omitempty"`
	L1DAMode         json.RawMessage `json:"l1_da_mode,omitempty"`
}

type retained struct {
	blockHeader
	Transactions []json.RawMessage `json:"transactions"`
	Receipts     []json.RawMessage `json:"transaction_receipts"`
}

type confirmedBlock struct {
	Hash string `json:"block_hash"`
	retained
}

type round struct {
	Changed         bool    `json:"changed"`
	BlockIdentifier string  `json:"block_identifier"`
	BlockNumber     *uint64 `json:"block_number,omitempty"`
	Status          string  `json:"status,omitempty"`
	retained
	StateDiffs []fgwStateDiff `json:"transaction_state_diffs"`
}

func buildRound(number uint64, block *confirmedBlock, traces []tracedTransaction) ([]byte, error) {
	body, err := composeRound(block, traces)
	if err != nil {
		return nil, fmt.Errorf("block %d: %w", number, err)
	}

	if err := validateRound(body); err != nil {
		return nil, fmt.Errorf("block %d: %w", number, err)
	}

	return gzipBytes(body)
}

func composeRound(block *confirmedBlock, traces []tracedTransaction) ([]byte, error) {
	if len(traces) != len(block.Transactions) {
		return nil, fmt.Errorf(
			"trace has %d transactions, block has %d",
			len(traces),
			len(block.Transactions),
		)
	}

	round := round{
		Changed:         true,
		BlockIdentifier: block.Hash,
		Status:          preConfirmedStatus,
		retained:        block.retained,
		StateDiffs:      make([]fgwStateDiff, 0, len(traces)),
	}

	for index, trace := range traces {
		hash, err := transactionHash(block.Transactions[index])
		if err != nil {
			return nil, err
		}

		if trace.TransactionHash != hash {
			return nil, fmt.Errorf("trace tx %d is %s, block has %s", index, trace.TransactionHash, hash)
		}

		round.StateDiffs = append(round.StateDiffs, toFGWStateDiff(trace.TraceRoot.StateDiff))
	}

	return json.Marshal(round)
}

func transactionHash(transaction json.RawMessage) (string, error) {
	var fields struct {
		Hash string `json:"transaction_hash"`
	}

	err := json.Unmarshal(transaction, &fields)
	return fields.Hash, err
}

func validateRound(body []byte) error {
	envelope, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(body))
	if err != nil {
		return err
	}

	if err := envelope.Validate(); err != nil {
		return err
	}

	if _, ok := envelope.Update.(starknet.PreConfirmedBlock); !ok {
		return fmt.Errorf("round decodes as %T, not a full pre-confirmed block", envelope.Update)
	}

	return nil
}

func decodeRound(gz []byte) (*round, error) {
	body, err := gunzip(gz)
	if err != nil {
		return nil, err
	}

	var round round
	if err := json.Unmarshal(body, &round); err != nil {
		return nil, err
	}

	transactions := len(round.Transactions)
	receipts, diffs := len(round.Receipts), len(round.StateDiffs)
	if transactions != receipts || transactions != diffs {
		return nil, fmt.Errorf(
			"%d transactions, %d receipts, %d state diffs",
			transactions,
			receipts,
			diffs,
		)
	}

	round.Transactions = orEmpty(round.Transactions)
	round.Receipts = orEmpty(round.Receipts)
	round.StateDiffs = orEmpty(round.StateDiffs)
	return &round, nil
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}

	return list
}

func (round *round) reply(known, shown uint64, blockNumber *uint64) ([]byte, error) {
	reply := *round
	reply.BlockNumber = blockNumber
	if known > 0 {
		reply.Status = ""
		reply.blockHeader = blockHeader{}
	}
	reply.slice(known, shown)
	return reply.encode()
}

func (round *round) slice(from, to uint64) {
	round.Transactions = round.Transactions[from:to]
	round.Receipts = round.Receipts[from:to]
	round.StateDiffs = round.StateDiffs[from:to]
}

func (round *round) encode() ([]byte, error) {
	body, err := json.Marshal(round)
	if err != nil {
		return nil, err
	}

	return gzipBytes(body)
}

type (
	fgwStateDiff struct {
		StorageDiffs         map[string][]fgwEntry `json:"storage_diffs"`
		Nonces               map[string]string     `json:"nonces"`
		DeployedContracts    []fgwDeployedContract `json:"deployed_contracts"`
		OldDeclaredContracts []string              `json:"old_declared_contracts"`
		DeclaredClasses      []fgwDeclaredClass    `json:"declared_classes"`
		ReplacedClasses      []fgwReplacedClass    `json:"replaced_classes"`
		MigratedClasses      []fgwDeclaredClass    `json:"migrated_compiled_classes"`
	}
	fgwEntry struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	fgwDeployedContract struct {
		Address   string `json:"address"`
		ClassHash string `json:"class_hash"`
	}
	fgwDeclaredClass struct {
		ClassHash         string `json:"class_hash"`
		CompiledClassHash string `json:"compiled_class_hash"`
	}
	fgwReplacedClass struct {
		Address   string `json:"address"`
		ClassHash string `json:"class_hash"`
	}
)

func toFGWStateDiff(diff *rpcStateDiff) fgwStateDiff {
	if diff == nil {
		diff = &rpcStateDiff{}
	}

	converted := fgwStateDiff{
		StorageDiffs:         make(map[string][]fgwEntry, len(diff.StorageDiffs)),
		Nonces:               make(map[string]string, len(diff.Nonces)),
		DeployedContracts:    make([]fgwDeployedContract, 0, len(diff.DeployedContracts)),
		OldDeclaredContracts: append([]string{}, diff.DeprecatedDeclaredClasses...),
		DeclaredClasses:      make([]fgwDeclaredClass, 0, len(diff.DeclaredClasses)),
		ReplacedClasses:      make([]fgwReplacedClass, 0, len(diff.ReplacedClasses)),
		MigratedClasses:      make([]fgwDeclaredClass, 0, len(diff.MigratedCompiledClasses)),
	}

	for _, storage := range diff.StorageDiffs {
		entries := make([]fgwEntry, 0, len(storage.StorageEntries))
		for _, entry := range storage.StorageEntries {
			entries = append(entries, fgwEntry(entry))
		}

		converted.StorageDiffs[storage.Address] = entries
	}

	for _, nonce := range diff.Nonces {
		converted.Nonces[nonce.ContractAddress] = nonce.Nonce
	}

	for _, deployed := range diff.DeployedContracts {
		converted.DeployedContracts = append(converted.DeployedContracts, fgwDeployedContract(deployed))
	}

	for _, declared := range diff.DeclaredClasses {
		converted.DeclaredClasses = append(converted.DeclaredClasses, fgwDeclaredClass(declared))
	}

	for _, replaced := range diff.ReplacedClasses {
		converted.ReplacedClasses = append(converted.ReplacedClasses, fgwReplacedClass{
			Address:   replaced.ContractAddress,
			ClassHash: replaced.ClassHash,
		})
	}

	for _, migrated := range diff.MigratedCompiledClasses {
		converted.MigratedClasses = append(converted.MigratedClasses, fgwDeclaredClass(migrated))
	}

	return converted
}
