package cbor

// The initial byte of every CBOR item, as RFC 8949 §3 defines it.
const (
	// majorMask and infoMask split that byte into major type and additional info.
	majorMask = 0b111_00000
	infoMask  = 0b000_11111

	// Major types, the top 3 bits. Only the ones the node has to recognise are named.
	uintMajor   = 0 << 5
	negIntMajor = 1 << 5
	arrayMajor  = 4 << 5
	simpleMajor = 7 << 5

	// maxInline is the largest argument that still rides inside the header byte.
	maxInline = info1Byte - 1

	// Additional info values that say how many bytes of argument follow.
	info1Byte = 24
	info2Byte = 25
	info4Byte = 26
	info8Byte = 27

	// 31 in an array means "read until the break". The same 31 in a simple value is that break.
	// 28 to 30 are reserved, and no argument width follows them.
	// The node writes none of the three and has to refuse all of them.
	arrayIndefinite = arrayMajor | 31
	breakStop       = simpleMajor | 31
	reservedInfo    = 28

	// null is what the generic encoder emits for a nil slice.
	null = simpleMajor | 22
)
