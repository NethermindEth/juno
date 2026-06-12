package reedsolomon

import (
	"errors"
	"fmt"

	"github.com/klauspost/reedsolomon"
)

// EncodeData generates the coding shards usign Reed-Solomon
// erasure codes. Receives the data, amount of shards  and parity number.
// It will return the Reed Solomon encoding where the first `numDataShards`
// `[]byte` slices will be occupied by the original data. The remaining `parity`
// `[]byte` slices will contain the coding shards.
func EncodeData(
	data []byte,
	numDataShards,
	parity int,
) ([][]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("received empty data")
	}

	encoder, err := reedsolomon.New(numDataShards, parity)
	if err != nil {
		return nil, fmt.Errorf("creating Reed-Solomon encoder: %w", err)
	}

	split, err := encoder.Split(data)
	if err != nil {
		return nil, fmt.Errorf("splitting the data into shards: %w", err)
	}

	err = encoder.Encode(split)
	if err != nil {
		return nil, fmt.Errorf("encoding the data shards: %w", err)
	}

	return split, nil
}

// RecoverData restores the missing data using Reed-Solomon erasure codes.
// There cannot be more than `parity` shards missing otherwise the recover will fail.
// Data that is considered missing needs to be marked as `nil`. Returns the recovered data.
// The input data shards well be modified in place.
func RecoverData(
	shards [][]byte,
	numDataShards,
	parity int,
) ([][]byte, error) {
	if len(shards) == 0 {
		return nil, errors.New("no data shards provided")
	}

	// todo(rdr): numDataShards can be inferred by getting the length of the shards
	decoder, err := reedsolomon.New(numDataShards, parity)
	if err != nil {
		return nil, fmt.Errorf("creating Reed-Solomon decoder: %w", err)
	}

	// todo(rdr): this is a slow approach where we are reconstructing parity shards as
	// well. This is safe because at the end we can verify that it is correct. We might
	// want to speed this up using `ReconstructData` with no `Verify` which should be 3x faster.
	err = decoder.Reconstruct(shards)
	if err != nil {
		return nil, fmt.Errorf("recovering the data shards: %w", err)
	}

	correct, err := decoder.Verify(shards)
	if err != nil {
		return nil, fmt.Errorf("verifying the data shards: %w", err)
	}

	if !correct {
		return nil, errors.New("data shard failed verification")
	}

	return shards, nil
}
