// This executable isolates the generic indexing operation used by unicode/utf8.word.
package main

func First[T ~string | ~[]byte](s T) byte { return s[0] }

type Bytes []byte

func main() {
	if First([]byte{42}) != 42 || First(Bytes{43}) != 43 || First("x") != 'x' {
		panic("generic indexing returned the wrong byte")
	}
}
