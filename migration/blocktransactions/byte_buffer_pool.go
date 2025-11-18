package blocktransactions

import (
	"bytes"
	"sync"
)

type byteBufferPool struct {
	sync.Pool
	reusableSize int
}

func newByteBufferPool(reusableSize int) *byteBufferPool {
	return &byteBufferPool{
		Pool: sync.Pool{
			New: func() any {
				var buffer bytes.Buffer
				buffer.Grow(reusableSize)
				return &buffer
			},
		},
		reusableSize: reusableSize,
	}
}

func (b *byteBufferPool) get() *bytes.Buffer {
	return b.Pool.Get().(*bytes.Buffer)
}

func (b *byteBufferPool) put(buffer *bytes.Buffer) {
	if buffer.Cap() > b.reusableSize {
		return
	}
	buffer.Reset()
	b.Pool.Put(buffer)
}
