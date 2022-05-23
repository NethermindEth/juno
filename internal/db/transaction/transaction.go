package transaction

func (x *Transaction) Equal(y *Transaction) bool {
	if x == y {
		return true
	}
	// Compare hash
	if x.GetHash() != y.GetHash() {
		return false
	}
	// Compare Tx
	switch x.GetTx().(type) {
	case *Transaction_Deploy:
		return x.GetDeploy().Equal(y.GetDeploy())
	case *Transaction_Invoke:
		return x.GetInvoke().Equal(y.GetInvoke())
	default:
		return false
	}
}

func (x *Deploy) Equal(y *Deploy) bool {
	if x == y {
		return true
	}
	// Compare contract address salt
	if x.ContractAddressSalt != y.ContractAddressSalt {
		return false
	}
	// Compare call data
	if len(x.ConstructorCallData) != len(y.ConstructorCallData) {
		return false
	}
	for i, data := range x.ConstructorCallData {
		if data != y.ConstructorCallData[i] {
			return false
		}
	}
	return true
}

func (x *InvokeFunction) Equal(y *InvokeFunction) bool {
	if x == y {
		return true
	}
	// Compare contract address
	if x.ContractAddress != y.ContractAddress {
		return false
	}
	// Compare entry point selector
	if x.EntryPointSelector != y.EntryPointSelector {
		return false
	}
	// Compare call data
	if len(x.CallData) != len(y.CallData) {
		return false
	}
	for i, data := range x.CallData {
		if data != y.CallData[i] {
			return false
		}
	}
	// Compare signature
	if len(x.Signature) != len(y.Signature) {
		return false
	}
	for i, sign := range x.Signature {
		if sign != y.Signature[i] {
			return false
		}
	}
	// Compare MaxFee
	if x.MaxFee != y.MaxFee {
		return false
	}
	return true
}
