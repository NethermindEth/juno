package main

import (
	"fmt"
	"net/url"
	"time"
)

func (server *server) preConfirmedReply(requestURL *url.URL) ([]byte, error) {
	values := requestURL.Query()
	if !values.Has("blockIdentifier") {
		return nil, malformedf("Field blockIdentifier is required.")
	}

	query, err := decode[preConfirmedQuery](values)
	if err != nil {
		return nil, malformedf("%s: %v", preConfirmedBlock.name, err)
	}

	snapshot := server.window.current.Load()
	number, queriedLatest, err := snapshot.resolveBlockNumber(query.BlockNumber, server.config)
	if err != nil {
		return nil, err
	}

	block, ok := snapshot.blocks[number]
	if !ok {
		return nil, fmt.Errorf("block %d: not prepared", number)
	}

	phase := resolvePhase(snapshot, server.config, number)
	total := uint64(len(block.round.Transactions))
	shown := revealed(total, server.config.stages, phase)

	sameBlock := query.BlockIdentifier == block.round.BlockIdentifier
	known := query.KnownTransactionCount
	if !sameBlock {
		known = 0
	}
	fromScratch := known == 0

	switch {
	case sameBlock && known >= shown:
		return server.window.noChange, nil
	case fromScratch && queriedLatest:
		return snapshot.latest[phase], nil
	case fromScratch && shown == total:
		return block.stored, nil
	}

	var blockNumber *uint64
	if queriedLatest {
		blockNumber = &number
	}

	return block.round.reply(known, shown, blockNumber)
}

func resolvePhase(snapshot *snapshot, config *config, number uint64) uint64 {
	isLatestBlock := number == snapshot.tip+config.lead
	if !isLatestBlock {
		return config.stages
	}

	return snapshot.phase(time.Now(), config.stages)
}
