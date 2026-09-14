package indexed

import (
	"bytes"
	"iter"

	"github.com/NethermindEth/juno/utils/cbor/v1"
)

type BufferedEncoder struct {
	*bytes.Buffer
	cbor.Encoder
}

func NewBufferedEncoder() BufferedEncoder {
	var buf bytes.Buffer
	return BufferedEncoder{
		Buffer:  &buf,
		Encoder: cbor.NewEncoder(&buf),
	}
}

func (writer BufferedEncoder) WriteSeq[T any](items iter.Seq2[T, error]) ([]int, error) {
	indexes := make([]int, 0)
	for tx, err := range items {
		if err != nil {
			return nil, err
		}

		indexes = append(indexes, writer.Len())
		if err := writer.Encode(tx); err != nil {
			return nil, err
		}
	}

	return indexes, nil
}
