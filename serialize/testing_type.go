package serialize

type TestInner struct {
	InnerA string
}

type TestMarshal struct {
	A int64
	B string
	C bool
	D TestInner
}
