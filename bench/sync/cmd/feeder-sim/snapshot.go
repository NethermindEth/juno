package main

import (
	"strconv"
	"time"
)

type prepared struct {
	stored []byte
	round  *round
}

type snapshot struct {
	advance
	start  time.Time
	blocks map[uint64]prepared
	latest [][]byte
}

func newSnapshot(advance advance) *snapshot {
	return &snapshot{
		advance: advance,
		start:   time.Now(),
		blocks:  make(map[uint64]prepared),
	}
}

func (snapshot *snapshot) bounds(config *config) (lo, hi uint64) {
	lo = config.from
	if snapshot.tip >= config.from+config.keep {
		lo = snapshot.tip - config.keep
	}

	return lo, min(config.to, snapshot.tip+config.lead)
}

func (snapshot *snapshot) phase(now time.Time, stages uint64) uint64 {
	if snapshot.interval <= 0 {
		return stages
	}

	elapsed := max(now.Sub(snapshot.start), 0)
	return min(stages, uint64(elapsed)*(stages+1)/uint64(snapshot.interval))
}

func (snapshot *snapshot) resolveBlockNumber(
	blockNumber string,
	config *config,
) (number uint64, queriedLatest bool, err error) {
	lo, hi := snapshot.bounds(config)
	switch blockNumber {
	case "":
		return 0, false, malformedf("Field blockNumber is required.")
	case latestBlock:
		if hi <= snapshot.tip {
			return 0, false, notFoundf("No pre-confirmed block.")
		}

		return hi, true, nil
	}

	number, err = strconv.ParseUint(blockNumber, 10, 64)
	if err != nil {
		return 0, false, malformedf("%s: %v", preConfirmedBlock.name, err)
	}

	if number < lo || number > hi {
		return 0, false, notFoundf("Pre-confirmed block with number %d was not found.", number)
	}

	return number, false, nil
}

func revealed(total, stages, phase uint64) uint64 {
	if stages == 0 {
		return total
	}

	return total * phase / stages
}
