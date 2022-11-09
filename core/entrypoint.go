package core

// EntryPoint uniquely identifies a Cairo function to execute.
type EntryPoint struct {
	// Keccak hash of the function signature.
	Selector []byte
	// The offset of the instruction that should be called in the class's bytecode.
	Offset uint
}
