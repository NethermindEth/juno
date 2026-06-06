package eth

// AddressLength is the expected length of the address.
const AddressLength = 20

// Address represents the 20 byte address of an Ethereum account.
type Address [AddressLength]byte

// AddressFromBytes returns Address with value b.
// If b is larger than AddressLength, b will be cropped from the left.
func AddressFromBytes(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

// AddressFromString returns Address with byte values parsed from s. The 0x
// prefix is optional; if s is larger than 2*AddressLength hex chars, it
// will be cropped from the left.
func AddressFromString(s string) Address { return AddressFromBytes(fromHex(s)) }

// Bytes returns a copy of the underlying byte representation of the address.
func (a Address) Bytes() []byte { return a[:] }

// SetBytes sets the address to the value of b. If b is larger than
// AddressLength, b will be cropped from the left; if shorter, left-padded
// with zeros.
// TODO: Once migrated, panic/error if b is larger than address
func (a *Address) SetBytes(b []byte) { setBytes(a[:], b) }

// MarshalText returns the hex representation of a (0x-prefixed lowercase).
// Mirrors go-ethereum's common.Address.MarshalText.
func (a Address) MarshalText() ([]byte, error) { return encodeHex(a[:]), nil }

// UnmarshalText parses an address from its hex representation. Matches
// go-ethereum's common.Address.UnmarshalText: requires "0x"-prefixed hex of
// exactly AddressLength bytes; case-insensitive.
func (a *Address) UnmarshalText(input []byte) error { return decodeHexText(a[:], input) }

// UnmarshalJSON parses an address from a JSON string. Matches go-ethereum's
// common.Address.UnmarshalJSON: requires a quoted "0x"-prefixed hex string of
// exactly AddressLength bytes; case-insensitive.
func (a *Address) UnmarshalJSON(input []byte) error { return decodeHexJSON(a[:], input) }
