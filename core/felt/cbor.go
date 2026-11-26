package felt

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/feltcbor"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// A felt has two CBOR forms. The legacy form is an array of the 4 Montgomery limbs, each a
// minimally sized uint. The compact form is a uint below 24, a byte string of up to 30 trimmed
// big-endian bytes, or a 32-byte string of Montgomery limbs for values whose canonical form needs
// 31 or 32 bytes, so the length alone tells the forms apart. Every build decodes both; the
// compactfelt build tag picks which form MarshalCBOR and appendCBOR write, and whether StoredBytes
// trims standalone values (cbor_compact.go or cbor_legacy.go). SetMinimalBytes reads both.

var (
	_ cbor.SelfEncoder = (*Felt)(nil)
	_ cbor.SelfDecoder = (*Felt)(nil)
)

// UnmarshalCBOR decodes a compact or legacy felt that spans all of data, writing z only on success.
func (z *Typed[K]) UnmarshalCBOR(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("decoding felt: %w", errFeltEmpty)
	}
	var decoded fp.Element
	consumed, err := feltcbor.Decode(data, &decoded)
	if err == nil && consumed == len(data) {
		*(*fp.Element)(z) = decoded
		return nil
	}
	if err == feltcbor.ErrLegacy || (err == nil && data[0] == cborLegacyArrayHeader) {
		if err := cbor.Unmarshal(data, (*fp.Element)(z)); err != nil {
			return fmt.Errorf("decoding legacy felt: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("decoding felt: %w", err)
	}

	return fmt.Errorf(
		"decoding felt: %w: input has %d bytes but the felt encoding ends after %d",
		errFeltTrailingBytes, len(data), consumed,
	)
}

const (
	// These derive from the CBOR spec, see https://www.rfc-editor.org/rfc/rfc8949.html#section-3
	cborUint8AdditionalInfo  = 24 // 1 byte follows
	cborUint16AdditionalInfo = 25 // 2 bytes follow
	cborUint32AdditionalInfo = 26 // 4 bytes follow
	cborUint64AdditionalInfo = 27 // 8 bytes follow

	cborUintMajor       = 0 << 5
	cborByteStringMajor = 2 << 5
	cborArrayMajor      = 4 << 5

	// cborLegacyArrayHeader is the first byte of a legacy felt: an array of Limbs items.
	cborLegacyArrayHeader = cborArrayMajor | Limbs

	maxCBORLegacyFeltLen = feltcbor.WideLen

	// maxCanonicalByteLen is the longest canonical byte string; longer values use Montgomery form.
	maxCanonicalByteLen = feltcbor.MaxTrimmed
	// A compact value below 24 is a single header byte
	minCBORFeltLen = 1
)

var errFeltTrailingBytes = errors.New("trailing bytes")
