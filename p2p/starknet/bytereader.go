package starknet

import (
	"io"

	"google.golang.org/protobuf/encoding/protodelim"
)

var _ protodelim.Reader = (*byteReader)(nil)

type byteReader struct {
	io.Reader
}

func (r *byteReader) ReadByte() (byte, error) {
	var b [1]byte
	if _, err := r.Read(b[:]); err != nil {
		return 0, err
	}
	return b[0], nil
}
