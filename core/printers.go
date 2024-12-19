//go:build no_coverage
// +build no_coverage

package core

import "fmt"

// These methods are moved here, so that we can exclude them from codecov.
// They are quite useful when debugging transaction execution.

func (r *TransactionReceipt) Print() {
	fmt.Println("Transaction Receipt:")
	fmt.Println("  Fee:", r.Fee.String())
	fmt.Println("  Fee Unit:", r.FeeUnit)
	fmt.Println("  Transaction Hash:", r.TransactionHash.String())
	fmt.Println("  Reverted:", r.Reverted)
	if r.Reverted {
		fmt.Println("  Revert Reason:", r.RevertReason)
	}
	fmt.Println("  Events:")
	for i, event := range r.Events {
		fmt.Printf("    Event %d:\n", i+1)
		fmt.Println("      From:", event.From.String())
		fmt.Println("      Data:")
		for j, data := range event.Data {
			fmt.Printf("        Data %d: %s\n", j+1, data.String())
		}
		fmt.Println("      Keys:")
		for j, key := range event.Keys {
			fmt.Printf("        Key %d: %s\n", j+1, key.String())
		}
	}
	if r.ExecutionResources != nil {
		fmt.Println("  Execution Resources:")
		r.ExecutionResources.Print()
	}
	fmt.Println("  L1 To L2 Message: TODO")
	fmt.Println("  L2 To L1 Messages: TODO")
}

func (d *StateDiff) Print() {
	fmt.Println("StateDiff {")
	fmt.Println("  StorageDiffs:")
	for addr, keyValueMap := range d.StorageDiffs {
		fmt.Printf("    %s:\n", addr.String())
		for key, value := range keyValueMap {
			fmt.Printf("      %s: %s\n", key.String(), value.String())
		}
	}
	fmt.Println("  Nonces:")
	for addr, nonce := range d.Nonces {
		fmt.Printf("    %s: %s\n", addr.String(), nonce.String())
	}
	fmt.Println("  DeployedContracts:")
	for addr, classHash := range d.DeployedContracts {
		fmt.Printf("    %s: %s\n", addr.String(), classHash.String())
	}
	fmt.Println("  DeclaredV0Classes:")
	for _, classHash := range d.DeclaredV0Classes {
		fmt.Printf("    %s\n", classHash.String())
	}
	fmt.Println("  DeclaredV1Classes:")
	for classHash, compiledClassHash := range d.DeclaredV1Classes {
		fmt.Printf("    %s: %s\n", classHash.String(), compiledClassHash.String())
	}
	fmt.Println("  ReplacedClasses:")
	for addr, classHash := range d.ReplacedClasses {
		fmt.Printf("    %s: %s\n", addr.String(), classHash.String())
	}
	fmt.Println("}")
}

func (er *ExecutionResources) Print() {
	fmt.Println("    Builtin Instance Counter:")
	er.BuiltinInstanceCounter.Print()
	fmt.Println("    Memory Holes:", er.MemoryHoles)
	fmt.Println("    Steps:", er.Steps)
	if er.DataAvailability != nil {
		fmt.Println("    Data Availability:")
		er.DataAvailability.Print()
	}
	if er.TotalGasConsumed != nil {
		fmt.Println("    Total Gas Consumed:")
		er.TotalGasConsumed.Print()
	}
}

func (bic *BuiltinInstanceCounter) Print() {
	fmt.Println("      Pedersen:", bic.Pedersen)
	fmt.Println("      RangeCheck:", bic.RangeCheck)
	fmt.Println("      Bitwise:", bic.Bitwise)
	fmt.Println("      Output:", bic.Output)
	fmt.Println("      Ecsda:", bic.Ecsda)
	fmt.Println("      EcOp:", bic.EcOp)
	fmt.Println("      Keccak:", bic.Keccak)
	fmt.Println("      Poseidon:", bic.Poseidon)
	fmt.Println("      SegmentArena:", bic.SegmentArena)
	fmt.Println("      AddMod:", bic.AddMod)
	fmt.Println("      MulMod:", bic.MulMod)
	fmt.Println("      RangeCheck96:", bic.RangeCheck96)
}

func (gc *GasConsumed) Print() {
	fmt.Println("      L1 Gas:", gc.L1Gas)
	fmt.Println("      L1 Data Gas:", gc.L1DataGas)
}

func (da *DataAvailability) Print() {
	fmt.Println("      L1 Gas:", da.L1Gas)
	fmt.Println("      L1 Data Gas:", da.L1DataGas)
}
