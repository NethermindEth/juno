package felt

func IsZero[F FeltLike](v *F) bool {
	return asFeltPtr(v).IsZero()
}

func Equal[F FeltLike](a, b *F) bool {
	return asFeltPtr(a).Equal(asFeltPtr(b))
}
