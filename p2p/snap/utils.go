package snap

import (
	"bufio"
	"bytes"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/p2p/snap/p2pproto"
	"github.com/klauspost/compress/zstd"
	"github.com/multiformats/go-varint"
	"google.golang.org/protobuf/proto"
	"io"
)

func readCompressedProtobuf(stream io.Reader, request proto.Message) error {
	reader := bufio.NewReader(stream)
	_, err := varint.ReadUvarint(reader)
	if err != nil {
		return err
	}

	decoder, err := zstd.NewReader(reader)
	if err != nil {
		return err
	}

	msgBuff, err := io.ReadAll(decoder)
	if err != nil {
		return err
	}

	err = proto.Unmarshal(msgBuff, request)
	if err != nil {
		return err
	}

	return nil
}

func writeCompressedProtobuf(stream io.Writer, resp proto.Message) error {
	buff, err := proto.Marshal(resp)
	if err != nil {
		return err
	}

	buffer := bytes.NewBuffer(make([]byte, 0))
	compressed, err := zstd.NewWriter(buffer)
	if err != nil {
		return err
	}

	_, err = compressed.Write(buff)
	if err != nil {
		return err
	}
	err = compressed.Close()
	if err != nil {
		return err
	}

	uvariantBuffer := varint.ToUvarint(uint64(buffer.Len()))
	_, err = stream.Write(uvariantBuffer)
	if err != nil {
		return err
	}

	bwriter := bufio.NewWriter(stream)
	_, err = io.Copy(bwriter, buffer)
	if err != nil {
		return err
	}

	err = bwriter.Flush()
	if err != nil {
		return err
	}

	return nil
}

func fieldElementToFelt(field *p2pproto.FieldElement) *felt.Felt {
	if field == nil {
		return nil
	}
	thefelt := felt.Zero
	thefelt.SetBytes(field.Elements)
	return &thefelt
}

func feltToFieldElement(flt *felt.Felt) *p2pproto.FieldElement {
	if flt == nil {
		return nil
	}
	return &p2pproto.FieldElement{Elements: flt.Marshal()}
}

func bitsetToProto(bset *trie.Key) *p2pproto.Path {
	// TODO: do this properly
	buff := bytes.NewBuffer(make([]byte, 0))
	_, err := bset.WriteTo(buff)
	if err != nil {
		panic(err)
	}

	return &p2pproto.Path{
		Length:  uint32(bset.Len()),
		Element: buff.Bytes(),
	}
}

func protoToBitset(path *p2pproto.Path) *trie.Key {
	// TODO: do this properly
	k := trie.NewKey(0, make([]byte, 0))
	err := k.UnmarshalBinary(path.Element)
	if err != nil {
		panic(err)
	}

	return &k
}

func feltsToFieldElements(felts []*felt.Felt) []*p2pproto.FieldElement {
	return toProtoMapArray(felts, feltToFieldElement)
}

func fieldElementsToFelts(fieldElements []*p2pproto.FieldElement) []*felt.Felt {
	felts := make([]*felt.Felt, len(fieldElements))
	for i, fe := range fieldElements {
		felts[i] = fieldElementToFelt(fe)
	}

	return felts
}

func toProtoMapArray[F any, T any](from []F, mapper func(F) T) (to []T) {
	if len(from) == 0 {
		// protobuf does not distinguish between nil array or empty array. But we put it here for testing reason
		// as when deserializing it always deserialize empty array as nil
		return nil
	}

	toArray := make([]T, len(from))

	for i, f := range from {
		toArray[i] = mapper(f)
	}

	return toArray
}
