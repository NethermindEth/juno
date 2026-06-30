package db

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/batchrepr"
)

const valueLenBytes = 5

// Encode Pebble's batchrepr format directly so replay can parse WAL records
// with ReadHeader and Read.
func encodeBatch[V types.Hashable[H], H types.Hash, A types.Addr](
	records []walRecordEnvelope[V, H, A],
	seqNum uint64,
	encodedBatch []byte,
) ([]byte, error) {
	if len(records) > math.MaxUint32 {
		return nil, errors.New("encodeBatch: too many WAL records in single flush")
	}

	const (
		kindBytes             = 1
		keyLenBytes           = 1
		keyBytes              = 4
		estimatedPayloadBytes = 128
		keyLenOffset          = kindBytes
		keyOffset             = keyLenOffset + keyLenBytes
		valueLenOffset        = keyOffset + keyBytes
		recordHeaderBytes     = valueLenOffset + valueLenBytes
		estimatedRecordBytes  = recordHeaderBytes + estimatedPayloadBytes
	)
	estimatedBatchBytes := batchrepr.HeaderLen + len(records)*estimatedRecordBytes
	if cap(encodedBatch) < estimatedBatchBytes {
		encodedBatch = make([]byte, batchrepr.HeaderLen, estimatedBatchBytes)
	} else {
		encodedBatch = encodedBatch[:batchrepr.HeaderLen]
	}
	// batchrepr header: seq num (8 bytes) followed by record count (4 bytes).
	binary.LittleEndian.PutUint64(encodedBatch[:8], seqNum)
	binary.LittleEndian.PutUint32(encodedBatch[8:batchrepr.HeaderLen], uint32(len(records)))

	for i := range records {
		record := &records[i]
		// Each batchrepr entry starts with the kind byte, the encoded key, and a
		// value length prefix backfilled once the payload size is known.
		encodedBatch = slices.Grow(encodedBatch, recordHeaderBytes)
		headerIndex := len(encodedBatch)
		valueLenIndex := headerIndex + valueLenOffset
		valueIndex := headerIndex + recordHeaderBytes
		encodedBatch = encodedBatch[:valueIndex]
		encodedBatch[headerIndex] = byte(pebble.InternalKeyKindSet)
		encodedBatch[headerIndex+keyLenOffset] = keyBytes
		binary.BigEndian.PutUint32(encodedBatch[headerIndex+keyOffset:valueLenIndex], uint32(i))

		var err error
		// Append the custom WAL payload as the batchrepr value bytes.
		encodedBatch, err = appendWALRecordPayload(encodedBatch, record)
		if err != nil {
			return nil, fmt.Errorf("encodeBatch: encode WAL envelope: %w", err)
		}

		// batchrepr stores value lengths as uvarints.
		valueLen := len(encodedBatch) - valueIndex
		putFixedUvarint32(encodedBatch[valueLenIndex:valueIndex], uint32(valueLen))
	}

	return encodedBatch, nil
}

func appendWALRecordPayload[V types.Hashable[H], H types.Hash, A types.Addr](
	payload []byte,
	record *walRecordEnvelope[V, H, A],
) ([]byte, error) {
	payload = append(payload, byte(record.Kind))
	switch record.Kind {
	case walRecordEntry:
		payload = append(payload, byte(record.EntryKind))
		switch record.EntryKind {
		case walEntryStart:
			payload = appendUint64(payload, uint64(record.StartHeight))
		case walEntryProposal:
			proposalMessage := (*types.Proposal[V, H, A])(record.ProposalEntry)
			payload = appendMessageHeader(payload, proposalMessage.MessageHeader)
			payload = appendInt64(payload, int64(proposalMessage.ValidRound))
			if proposalMessage.Value == nil {
				payload = append(payload, 0)
			} else {
				payload = append(payload, 1)
				var err error
				payload, err = appendValue(payload, proposalMessage.Value)
				if err != nil {
					return nil, err
				}
			}
		case walEntryPrevote:
			payload = appendVotePayload(payload, (*types.Vote[H, A])(record.PrevoteEntry))
		case walEntryPrecommit:
			payload = appendVotePayload(payload, (*types.Vote[H, A])(record.PrecommitEntry))
		case walEntryTimeout:
			timeoutMessage := (*types.Timeout)(record.TimeoutEntry)
			payload = append(payload, byte(timeoutMessage.Step))
			payload = appendUint64(payload, uint64(timeoutMessage.Height))
			payload = appendInt64(payload, int64(timeoutMessage.Round))
		default:
			return nil, fmt.Errorf("unknown WAL entry kind %d", record.EntryKind)
		}
	case walRecordPruneUpToHeight:
		payload = appendUint64(payload, uint64(record.Height))
	default:
		return nil, fmt.Errorf("unknown WAL record kind %d", record.Kind)
	}
	return payload, nil
}

func putFixedUvarint32(buf []byte, value uint32) {
	const fixedUvarintShift = 7

	buf[0] = byte(value) | 0x80
	for i := 1; i < valueLenBytes-1; i++ {
		buf[i] = byte(value>>(fixedUvarintShift*i)) | 0x80
	}
	buf[valueLenBytes-1] = byte(value >> (fixedUvarintShift * (valueLenBytes - 1)))
}

func decodeWALRecord[V types.Hashable[H], H types.Hash, A types.Addr](
	payload []byte,
) (walRecordEnvelope[V, H, A], error) {
	decoder := walRecordDecoder{data: payload}
	kind, err := decoder.readByte()
	if err != nil {
		return walRecordEnvelope[V, H, A]{}, err
	}

	record := walRecordEnvelope[V, H, A]{Kind: walRecordKind(kind)}
	switch record.Kind {
	case walRecordEntry:
		record, err = decodeWALEntryRecord(&decoder, record)
		if err != nil {
			return walRecordEnvelope[V, H, A]{}, err
		}
	case walRecordPruneUpToHeight:
		height, err := decoder.readHeight()
		if err != nil {
			return walRecordEnvelope[V, H, A]{}, err
		}
		record.Height = height
	default:
		return walRecordEnvelope[V, H, A]{}, fmt.Errorf("unknown WAL record kind %d", record.Kind)
	}
	if decoder.remaining() != 0 {
		return walRecordEnvelope[V, H, A]{}, fmt.Errorf(
			"trailing WAL record bytes: %d",
			decoder.remaining(),
		)
	}
	return record, nil
}

func decodeWALEntryRecord[V types.Hashable[H], H types.Hash, A types.Addr](
	decoder *walRecordDecoder,
	record walRecordEnvelope[V, H, A],
) (walRecordEnvelope[V, H, A], error) {
	entryKind, err := decoder.readByte()
	if err != nil {
		return walRecordEnvelope[V, H, A]{}, err
	}

	record.EntryKind = walEntryKind(entryKind)
	switch record.EntryKind {
	case walEntryStart:
		height, err := decoder.readHeight()
		if err != nil {
			return walRecordEnvelope[V, H, A]{}, err
		}
		record.StartHeight = height
	case walEntryProposal:
		proposal, err := decodeProposalRecord[V, H, A](decoder)
		if err != nil {
			return walRecordEnvelope[V, H, A]{}, err
		}
		record.ProposalEntry = proposal
	case walEntryPrevote:
		vote, err := decodeVotePayload[H, A](decoder)
		if err != nil {
			return walRecordEnvelope[V, H, A]{}, err
		}
		prevote := wal.Prevote[H, A](vote)
		record.PrevoteEntry = &prevote
	case walEntryPrecommit:
		vote, err := decodeVotePayload[H, A](decoder)
		if err != nil {
			return walRecordEnvelope[V, H, A]{}, err
		}
		precommit := wal.Precommit[H, A](vote)
		record.PrecommitEntry = &precommit
	case walEntryTimeout:
		timeout, err := decodeTimeoutRecord(decoder)
		if err != nil {
			return walRecordEnvelope[V, H, A]{}, err
		}
		record.TimeoutEntry = timeout
	default:
		return walRecordEnvelope[V, H, A]{}, fmt.Errorf("unknown WAL entry kind %d", record.EntryKind)
	}

	return record, nil
}

// Proposal payload: message header, valid round, optional value.
func decodeProposalRecord[V types.Hashable[H], H types.Hash, A types.Addr](
	decoder *walRecordDecoder,
) (*wal.Proposal[V, H, A], error) {
	header, err := readMessageHeader[A](decoder)
	if err != nil {
		return nil, err
	}
	validRound, err := decoder.readRound()
	if err != nil {
		return nil, err
	}
	hasValue, err := decoder.readPresenceByte()
	if err != nil {
		return nil, err
	}
	proposal := wal.Proposal[V, H, A]{
		MessageHeader: header,
		ValidRound:    validRound,
	}
	if hasValue {
		value, err := readValue[V](decoder)
		if err != nil {
			return nil, err
		}
		proposal.Value = &value
	}
	return &proposal, nil
}

// Vote payload: message header, optional value ID. Used by prevote and precommit.
func decodeVotePayload[H types.Hash, A types.Addr](d *walRecordDecoder) (types.Vote[H, A], error) {
	header, err := readMessageHeader[A](d)
	if err != nil {
		return types.Vote[H, A]{}, err
	}
	hasID, err := d.readPresenceByte()
	if err != nil {
		return types.Vote[H, A]{}, err
	}
	vote := types.Vote[H, A]{MessageHeader: header}
	if hasID {
		id, err := d.readUint64Array()
		if err != nil {
			return types.Vote[H, A]{}, err
		}
		hash := H(id)
		vote.ID = &hash
	}
	return vote, nil
}

// Timeout payload: step, height, round.
func decodeTimeoutRecord(decoder *walRecordDecoder) (*wal.Timeout, error) {
	step, err := decoder.readByte()
	if err != nil {
		return nil, err
	}
	height, err := decoder.readHeight()
	if err != nil {
		return nil, err
	}
	round, err := decoder.readRound()
	if err != nil {
		return nil, err
	}
	timeout := wal.Timeout(types.Timeout{
		Step:   types.Step(step),
		Height: height,
		Round:  round,
	})
	return &timeout, nil
}

func appendMessageHeader[A types.Addr](payload []byte, header types.MessageHeader[A]) []byte {
	payload = appendUint64(payload, uint64(header.Height))
	payload = appendInt64(payload, int64(header.Round))
	return appendUint64Array(payload, header.Sender)
}

func appendVotePayload[H types.Hash, A types.Addr](payload []byte, vote *types.Vote[H, A]) []byte {
	payload = appendMessageHeader(payload, vote.MessageHeader)
	if vote.ID == nil {
		return append(payload, 0)
	}
	payload = append(payload, 1)
	return appendUint64Array(payload, *vote.ID)
}

func appendUint64(payload []byte, value uint64) []byte {
	// Match Pebble batchrepr's little-endian header encoding within this WAL record.
	return binary.LittleEndian.AppendUint64(payload, value)
}

func appendInt64(payload []byte, value int64) []byte {
	return appendUint64(payload, uint64(value))
}

func appendUint64Array[T ~[4]uint64](payload []byte, value T) []byte {
	array := [4]uint64(value)
	for _, limb := range array {
		payload = appendUint64(payload, limb)
	}
	return payload
}

func appendValue[V any](payload []byte, value *V) ([]byte, error) {
	array, err := valueToUint64Array(value)
	if err != nil {
		return nil, err
	}
	for _, limb := range array {
		payload = appendUint64(payload, limb)
	}
	return payload, nil
}

type walRecordDecoder struct {
	data []byte
	pos  int
}

func (d *walRecordDecoder) remaining() int {
	return len(d.data) - d.pos
}

func (d *walRecordDecoder) readByte() (byte, error) {
	if d.remaining() < 1 {
		return 0, io.ErrUnexpectedEOF
	}
	value := d.data[d.pos]
	d.pos++
	return value, nil
}

func (d *walRecordDecoder) readPresenceByte() (bool, error) {
	value, err := d.readByte()
	if err != nil {
		return false, err
	}
	switch value {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("invalid presence byte %d", value)
	}
}

func (d *walRecordDecoder) readUint64() (uint64, error) {
	if d.remaining() < 8 {
		return 0, io.ErrUnexpectedEOF
	}
	value := binary.LittleEndian.Uint64(d.data[d.pos : d.pos+8])
	d.pos += 8
	return value, nil
}

func (d *walRecordDecoder) readHeight() (types.Height, error) {
	value, err := d.readUint64()
	return types.Height(value), err
}

func (d *walRecordDecoder) readRound() (types.Round, error) {
	value, err := d.readUint64()
	return types.Round(int64(value)), err
}

func (d *walRecordDecoder) readUint64Array() ([4]uint64, error) {
	var value [4]uint64
	for i := range value {
		limb, err := d.readUint64()
		if err != nil {
			return [4]uint64{}, err
		}
		value[i] = limb
	}
	return value, nil
}

func readMessageHeader[A types.Addr](d *walRecordDecoder) (types.MessageHeader[A], error) {
	height, err := d.readHeight()
	if err != nil {
		return types.MessageHeader[A]{}, err
	}
	round, err := d.readRound()
	if err != nil {
		return types.MessageHeader[A]{}, err
	}
	sender, err := d.readUint64Array()
	if err != nil {
		return types.MessageHeader[A]{}, err
	}
	return types.MessageHeader[A]{
		Height: height,
		Round:  round,
		Sender: A(sender),
	}, nil
}

func readValue[V any](d *walRecordDecoder) (V, error) {
	array, err := d.readUint64Array()
	if err != nil {
		var zero V
		return zero, err
	}
	return uint64ArrayToValue[V](array)
}

func valueToUint64Array[V any](value *V) ([4]uint64, error) {
	reflectValue := reflect.ValueOf(value).Elem()
	if reflectValue.Kind() != reflect.Array ||
		reflectValue.Len() != 4 ||
		reflectValue.Type().Elem().Kind() != reflect.Uint64 {
		return [4]uint64{}, fmt.Errorf("unsupported WAL value type %T", value)
	}
	var array [4]uint64
	for i := range array {
		array[i] = reflectValue.Index(i).Uint()
	}
	return array, nil
}

func uint64ArrayToValue[V any](array [4]uint64) (V, error) {
	var value V
	reflectValue := reflect.ValueOf(&value).Elem()
	if reflectValue.Kind() != reflect.Array ||
		reflectValue.Len() != 4 ||
		reflectValue.Type().Elem().Kind() != reflect.Uint64 {
		return value, fmt.Errorf("unsupported WAL value type %T", value)
	}
	for i, limb := range array {
		reflectValue.Index(i).SetUint(limb)
	}
	return value, nil
}
