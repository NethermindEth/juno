package abi

// notest

func (x *Abi) Equal(y *Abi) bool {
	if x == y {
		return true
	}
	// Compare functions
	if len(x.Functions) != len(y.Functions) {
		return false
	}
	for i, f := range x.Functions {
		if !f.Equal(y.Functions[i]) {
			return false
		}
	}
	// Compare events
	if len(x.Events) != len(y.Events) {
		return false
	}
	for i, e := range x.Events {
		if !e.Equal(y.Events[i]) {
			return false
		}
	}
	// Compare structs
	if len(x.Structs) != len(y.Structs) {
		return false
	}
	for i, s := range x.Structs {
		if !s.Equal(y.Structs[i]) {
			return false
		}
	}
	// Compare L1 Handlers
	if len(x.L1Handlers) != len(y.L1Handlers) {
		return false
	}
	for i, f := range x.L1Handlers {
		if !f.Equal(y.L1Handlers[i]) {
			return false
		}
	}
	// Compare constructor
	if !x.Constructor.Equal(y.Constructor) {
		return false
	}

	return true
}

func (x *Function) Equal(y *Function) bool {
	if x == y {
		return true
	}
	// Compare name
	if x.Name != y.Name {
		return false
	}
	// Compare inputs
	if len(x.Inputs) != len(y.Inputs) {
		return false
	}
	for i, input := range x.Inputs {
		if input.Name != y.Inputs[i].Name ||
			input.Type != y.Inputs[i].Type {
			return false
		}
	}
	// Compare outputs
	if len(x.Outputs) != len(y.Outputs) {
		return false
	}
	for i, output := range x.Outputs {
		if output.Name != y.Outputs[i].Name ||
			output.Type != y.Outputs[i].Type {
			return false
		}
	}
	return true
}

func (x *AbiEvent) Equal(y *AbiEvent) bool {
	if x == y {
		return true
	}
	// Compare name
	if x.Name != y.Name {
		return false
	}
	// Compare data
	if len(x.Data) != len(y.Data) {
		return false
	}
	for i, data := range x.Data {
		if data.Name != y.Data[i].Name ||
			data.Type != y.Data[i].Type {
			return false
		}
	}
	// Compare keys
	if len(x.Keys) != len(y.Keys) {
		return false
	}
	for i, key := range x.Keys {
		if key != y.Keys[i] {
			return false
		}
	}
	return true
}

func (x *Struct) Equal(y *Struct) bool {
	if x == y {
		return true
	}
	// Compare size
	if x.Size != y.Size {
		return false
	}
	// Compare name
	if x.Name != y.Name {
		return false
	}
	// Compare fields
	if len(x.Fields) != len(y.Fields) {
		return false
	}
	for i, field := range x.Fields {
		if field.Name != y.Fields[i].Name ||
			field.Type != y.Fields[i].Type {
			return false
		}
	}
	return true
}
