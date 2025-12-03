package felt

func IsZero[F FeltLike](v F) bool {
	f := Felt(v)
	return f.IsZero()
}

func Equal[F FeltLike](a, b F) bool {
	fa := Felt(a)
	fb := Felt(b)
	return fa.Equal(&fb)
}
