//go:build compactfelt

package felt

import "github.com/consensys/gnark-crypto/ecc/stark-curve/fp"

// The compactfelt build writes the compact form.

// maxCBORFeltLen is the longest encoding this build writes: two header bytes + the full value.
const maxCBORFeltLen = 2 + feltByteLen

// MarshalCBOR encodes z in the size-based compact felt form.
func (z *Typed[K]) MarshalCBOR() ([]byte, error) {
	return appendCBOR(make([]byte, 0, maxCBORFeltLen), (*fp.Element)(z)), nil
}

// appendCBOR appends the compact CBOR encoding of value to data.
func appendCBOR(data []byte, value *fp.Element) []byte {
	limbs := value.Bits()
	if limbs[3]|limbs[2]|limbs[1] == 0 && limbs[0] < cborUint8AdditionalInfo {
		return append(data, cborUintMajor|byte(limbs[0]))
	}

	length := trimmedLength(&limbs)
	if length > maxCanonicalByteLen {
		montgomery := bigEndianBytes((*[Limbs]uint64)(value))
		data = append(data, cborByteStringMajor|cborUint8AdditionalInfo, feltByteLen)
		return append(data, montgomery[:]...)
	}
	if length < cborUint8AdditionalInfo {
		data = append(data, cborByteStringMajor|byte(length))
	} else {
		data = append(data, cborByteStringMajor|cborUint8AdditionalInfo, byte(length))
	}

	buffer := bigEndianBytes(&limbs)
	return append(data, buffer[feltByteLen-length:]...)
}
