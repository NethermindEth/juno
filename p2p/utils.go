package p2p

import (
	"bufio"
	"bytes"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/grpcclient"
	"github.com/klauspost/compress/zstd"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-varint"
	"google.golang.org/protobuf/proto"
	"io"
)

func readCompressedProtobuf(stream network.Stream, request proto.Message) error {
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

func writeCompressedProtobuf(stream network.Stream, resp proto.Message) error {
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

	_, err = io.Copy(stream, buffer)
	if err != nil {
		return err
	}

	return nil
}

func fieldElementToFelt(field *grpcclient.FieldElement) *felt.Felt {
	if field == nil {
		return nil
	}
	thefelt := felt.Zero
	thefelt.SetBytes(field.Elements)
	return &thefelt
}

func feltToFieldElement(felt *felt.Felt) *grpcclient.FieldElement {
	if felt == nil {
		return nil
	}
	return &grpcclient.FieldElement{Elements: felt.Marshal()}
}

func feltsToFieldElements(felts []*felt.Felt) []*grpcclient.FieldElement {
	fieldElements := make([]*grpcclient.FieldElement, len(felts))
	for i, f := range felts {
		fieldElements[i] = feltToFieldElement(f)
	}

	return fieldElements
}

func fieldElementsToFelts(fieldElements []*grpcclient.FieldElement) []*felt.Felt {
	felts := make([]*felt.Felt, len(fieldElements))
	for i, fe := range fieldElements {
		felts[i] = fieldElementToFelt(fe)
	}

	return felts
}
