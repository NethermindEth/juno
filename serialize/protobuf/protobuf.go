package protobuf

import "google.golang.org/protobuf/proto"

func (t *TestMarshal) Marshal() ([]byte, error) {
	encode, err := proto.Marshal(t)

	return encode, err
}

func (t *TestMarshal) Unmarshal(b []byte) error {
	err := proto.Unmarshal(b, t)

	return err
}
