package felt

// IsZero reports whether v is zero; limb equality is the field element's own definition.
func IsZero[F FeltLike](v *F) bool {
	return *v == F{}
}

// Equal reports whether a and b are equal; limb equality is the field element's own definition.
func Equal[F FeltLike](a, b *F) bool {
	return *a == *b
}
