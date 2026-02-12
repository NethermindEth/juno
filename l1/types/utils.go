package types

func Equal[T U256Like](a, b *T) bool {
	fa := U256(*a)
	fb := U256(*b)
	return fa.Equal(&fb)
}
