package indexed

import (
	"bytes"
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/encoder"
)

type BufferedEncoder struct {
	*bytes.Buffer
	encoder.Encoder
}

func NewBufferedEncoder() BufferedEncoder {
	var buf bytes.Buffer
	return BufferedEncoder{
		Buffer:  &buf,
		Encoder: encoder.NewEncoder(&buf),
	}
}

var _ IndexedWriter[any] = (*BufferedEncoder)(nil)

type IndexedWriter[T any] interface {
	Len() int
	Encode(T) error
}

func Write[T any](
	writer IndexedWriter[T],
	items iter.Seq2[T, error],
	expectedCount int,
) ([]int, error) {
	indexes := make([]int, expectedCount)
	index := 0
	for tx, err := range items {
		if err != nil {
			return nil, err
		}

		if index >= expectedCount {
			return nil, fmt.Errorf("got more than expected %d items", expectedCount)
		}

		indexes[index] = writer.Len()
		index++
		if err := writer.Encode(tx); err != nil {
			return nil, err
		}
	}

	if index < expectedCount {
		return nil, fmt.Errorf("got less than expected %d items", expectedCount)
	}

	return indexes, nil
}
