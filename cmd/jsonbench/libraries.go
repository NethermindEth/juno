package jsonbench

// JSONLibrary abstracts a JSON marshal/unmarshal implementation.
type JSONLibrary struct {
	Name      string
	Marshal   func(v any) ([]byte, error)
	Unmarshal func(data []byte, v any) error
}
