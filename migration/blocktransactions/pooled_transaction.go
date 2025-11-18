package blocktransactions

import (
	"bytes"
	"errors"
)

type pooledTransaction struct {
	*bytes.Buffer
}

func (b pooledTransaction) MarshalCBOR() ([]byte, error) {
	if b.Buffer == nil {
		return nil, errors.New("buffer is nil")
	}
	return b.Bytes(), nil
}

type pooledTransactionSerializer struct{}

func (pooledTransactionSerializer) Marshal(value *pooledTransaction) ([]byte, error) {
	return value.MarshalCBOR()
}

func (pooledTransactionSerializer) Unmarshal(data []byte, value *pooledTransaction) error {
	if value.Buffer == nil {
		value.Buffer = transactionBufferPool.get()
	}
	_, err := value.Write(data)
	return err
}

var transactionBufferPool = newByteBufferPool(initialTxSize)
