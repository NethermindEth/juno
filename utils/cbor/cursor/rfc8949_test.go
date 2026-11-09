// The examples of RFC 8949, Appendix A (https://www.rfc-editor.org/rfc/rfc8949.html#appendix-A),
// pin the primitive readers to the standard. Each is read by the readers that should take it,
// refused by the others, and skipped. The expected values come from the RFC.

package cursor_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

type kind int

const (
	unsigned   kind = iota // read by Uint64, Int64 and BigInt
	negative               // read by Int64, when it fits, and BigInt
	bignum                 // tag 2 or 3, read by BigInt only
	byteString             // val is the payload in hex
	textString             // val is the text
	boolean                // val is true or false
	null
	array  // val is the element count
	table  // a map, val is the pair count
	other  // a float, a simple value or a tag the reader does not interpret, which only Skip reads
	unread // a form this package refuses, such as an indefinite-length item or tag 0 or 1
)

// vectors are every row of Table 6. hex is the encoding, the comment the RFC's diagnostic notation.
// kind says which readers accept the item and val what they return.
//
//nolint:lll // one row per vector
var vectors = []struct {
	hex  string
	kind kind
	val  string
}{
	{"00", unsigned, "0"},                                             // 0
	{"01", unsigned, "1"},                                             // 1
	{"0a", unsigned, "10"},                                            // 10
	{"17", unsigned, "23"},                                            // 23
	{"1818", unsigned, "24"},                                          // 24
	{"1819", unsigned, "25"},                                          // 25
	{"1864", unsigned, "100"},                                         // 100
	{"1903e8", unsigned, "1000"},                                      // 1000
	{"1a000f4240", unsigned, "1000000"},                               // 1000000
	{"1b000000e8d4a51000", unsigned, "1000000000000"},                 // 1000000000000
	{"1bffffffffffffffff", unsigned, "18446744073709551615"},          // 18446744073709551615
	{"c249010000000000000000", bignum, "18446744073709551616"},        // 18446744073709551616
	{"3bffffffffffffffff", negative, "-18446744073709551616"},         // -18446744073709551616
	{"c349010000000000000000", bignum, "-18446744073709551617"},       // -18446744073709551617
	{"20", negative, "-1"},                                            // -1
	{"29", negative, "-10"},                                           // -10
	{"3863", negative, "-100"},                                        // -100
	{"3903e7", negative, "-1000"},                                     // -1000
	{"f90000", other, ""},                                             // 0.0
	{"f98000", other, ""},                                             // -0.0
	{"f93c00", other, ""},                                             // 1.0
	{"fb3ff199999999999a", other, ""},                                 // 1.1
	{"f93e00", other, ""},                                             // 1.5
	{"f97bff", other, ""},                                             // 65504.0
	{"fa47c35000", other, ""},                                         // 100000.0
	{"fa7f7fffff", other, ""},                                         // 3.4028234663852886e+38
	{"fb7e37e43c8800759c", other, ""},                                 // 1e+300
	{"f90001", other, ""},                                             // 5.960464477539063e-08
	{"f90400", other, ""},                                             // 6.103515625e-05
	{"f9c400", other, ""},                                             // -4.0
	{"fbc010666666666666", other, ""},                                 // -4.1
	{"f97c00", other, ""},                                             // Infinity
	{"f97e00", other, ""},                                             // NaN
	{"f9fc00", other, ""},                                             // -Infinity
	{"fa7f800000", other, ""},                                         // Infinity
	{"fa7fc00000", other, ""},                                         // NaN
	{"faff800000", other, ""},                                         // -Infinity
	{"fb7ff0000000000000", other, ""},                                 // Infinity
	{"fb7ff8000000000000", other, ""},                                 // NaN
	{"fbfff0000000000000", other, ""},                                 // -Infinity
	{"f4", boolean, "false"},                                          // false
	{"f5", boolean, "true"},                                           // true
	{"f6", null, ""},                                                  // null
	{"f7", other, ""},                                                 // undefined
	{"f0", other, ""},                                                 // simple(16)
	{"f8ff", other, ""},                                               // simple(255)
	{"c074323031332d30332d32315432303a30343a30305a", unread, ""},      // 0("2013-03-21T20:04:00Z")
	{"c11a514b67b0", unread, ""},                                      // 1(1363896240)
	{"c1fb41d452d9ec200000", unread, ""},                              // 1(1363896240.5)
	{"d74401020304", other, ""},                                       // 23(h'01020304')
	{"d818456449455446", other, ""},                                   // 24(h'6449455446')
	{"d82076687474703a2f2f7777772e6578616d706c652e636f6d", other, ""}, // 32("http://www.example.com")
	{"40", byteString, ""},                                            // h''
	{"4401020304", byteString, "01020304"},                            // h'01020304'
	{"60", textString, ""},                                            // ""
	{"6161", textString, "a"},                                         // "a"
	{"6449455446", textString, "IETF"},                                // "IETF"
	{"62225c", textString, "\"\\"},                                    // "\"\\"
	{"62c3bc", textString, "\u00fc"},                                  // "ü"
	{"63e6b0b4", textString, "\u6c34"},                                // "水"
	{"64f0908591", textString, "\U00010151"},                          // "𐅑"
	{"80", array, "0"},                                                // []
	{"83010203", array, "3"},                                          // [1, 2, 3]
	{"8301820203820405", array, "3"},                                  // [1, [2, 3], [4, 5]]
	{"98190102030405060708090a0b0c0d0e0f101112131415161718181819", array, "25"}, // [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 1...
	{"a0", table, "0"},                                                         // {}
	{"a201020304", table, "2"},                                                 // {1: 2, 3: 4}
	{"a26161016162820203", table, "2"},                                         // {"a": 1, "b": [2, 3]}
	{"826161a161626163", array, "2"},                                           // ["a", {"b": "c"}]
	{"a56161614161626142616361436164614461656145", table, "5"},                 // {"a": "A", "b": "B", "c": "C", "d": "D", ...
	{"5f42010243030405ff", unread, ""},                                         // (_ h'0102', h'030405')
	{"7f657374726561646d696e67ff", unread, ""},                                 // "streaming"
	{"9fff", unread, ""},                                                       // []
	{"9f018202039f0405ffff", unread, ""},                                       // [1, [2, 3], [4, 5]]
	{"9f01820203820405ff", unread, ""},                                         // [1, [2, 3], [4, 5]]
	{"83018202039f0405ff", unread, ""},                                         // [1, [2, 3], [4, 5]]
	{"83019f0203ff820405", unread, ""},                                         // [1, [2, 3], [4, 5]]
	{"9f0102030405060708090a0b0c0d0e0f101112131415161718181819ff", unread, ""}, // [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 1...
	{"bf61610161629f0203ffff", unread, ""},                                     // {"a": 1, "b": [2, 3]}
	{"826161bf61626163ff", unread, ""},                                         // ["a", {"b": "c"}]
	{"bf6346756ef563416d7421ff", unread, ""},                                   // {"Fun": true, "Amt": -2}
}

func TestRFC8949AppendixA(t *testing.T) {
	for _, v := range vectors {
		t.Run(v.hex, func(t *testing.T) {
			data, err := hex.DecodeString(v.hex)
			require.NoError(t, err)
			fresh := func() *cursor.Cursor {
				c := cursor.NewCursor(data)
				return &c
			}

			switch v.kind {
			case unsigned, negative, bignum:
				readsInteger(t, fresh, v.kind, v.val)
			case byteString:
				c := fresh()
				got, err := c.Bytes()
				require.NoError(t, err)
				require.Equal(t, v.val, hex.EncodeToString(got))
				require.Zero(t, c.Len())
				_, err = fresh().String()
				require.ErrorIs(t, err, cursor.ErrType)
			case textString:
				c := fresh()
				got, err := c.String()
				require.NoError(t, err)
				require.Equal(t, v.val, got)
				require.Zero(t, c.Len())
				_, err = fresh().Bytes()
				require.ErrorIs(t, err, cursor.ErrType)
			case boolean:
				got, err := fresh().Bool()
				require.NoError(t, err)
				require.Equal(t, v.val == "true", got)
			case null:
				c := fresh()
				require.True(t, c.Null())
				require.Zero(t, c.Len())
			case array, table:
				var n int
				if v.kind == array {
					n, err = fresh().ArrayLen()
				} else {
					n, err = fresh().MapLen()
				}
				require.NoError(t, err)
				require.Equal(t, v.val, big.NewInt(int64(n)).String())
			}

			// Skip reads every item but the ones the reader refuses.
			c := fresh()
			err = c.Skip()
			if v.kind == bignum || v.kind == unread {
				require.ErrorIs(t, err, cursor.ErrUnsupported)
				require.Zero(t, c.Pos())
				return
			}
			require.NoError(t, err)
			require.Zero(t, c.Len())

			// and an item cut short is truncated, however it is cut.
			for n := range data {
				short := cursor.NewCursor(data[:n])
				require.ErrorIs(t, short.Skip(), cursor.ErrTruncated, "cut to %d bytes", n)
			}
		})
	}
}

// readsInteger checks the readers that take an integer, and that the others refuse it.
func readsInteger(t *testing.T, fresh func() *cursor.Cursor, k kind, val string) {
	want, ok := new(big.Int).SetString(val, 10)
	require.True(t, ok)
	c := fresh()
	got, err := c.BigInt()
	require.NoError(t, err)
	require.Zero(t, want.Cmp(got), "BigInt %s", got)
	require.Zero(t, c.Len())

	// A tag where a plain integer is expected is unsupported, an item of another type a type error.
	refused := cursor.ErrType
	if k == bignum {
		refused = cursor.ErrUnsupported
	}
	c = fresh()
	u, err := c.Uint64()
	if k == unsigned {
		require.NoError(t, err)
		require.Equal(t, want.Uint64(), u)
	} else {
		require.ErrorIs(t, err, refused)
		require.Zero(t, c.Pos())
	}

	c = fresh()
	i, err := c.Int64()
	if k != bignum && want.IsInt64() {
		require.NoError(t, err)
		require.Equal(t, want.Int64(), i)
	} else {
		require.ErrorIs(t, err, refused)
		require.Zero(t, c.Pos())
	}
}
