package key

type emptySerializer struct{}

func (emptySerializer) Marshal(value struct{}) []byte {
	return nil
}
