package serialize

import (
	"bytes"
	"encoding/gob"
	"log"
)

func MarshalGob(in any) bytes.Buffer {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(in)
	if err != nil {
		log.Fatal("encode error:", err)
	}

	return b
}

func UnMarshalGob[T any](b bytes.Buffer) T {
	dec := gob.NewDecoder(&b)
	var t T
	err := dec.Decode(&t)
	if err != nil {
		log.Fatal("decode error:", err)
	}

	return t
}
