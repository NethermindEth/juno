package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// readGasPricePtr reads a gas price behind a pointer, into slot of prices. A header's two gas
// prices share one allocation. Null leaves the pointer nil.
func readGasPricePtr(r *fast.Reader, dst **core.GasPrice, prices **[2]core.GasPrice, slot int) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	if *prices == nil {
		*prices = new([2]core.GasPrice)
	}
	value := &(*prices)[slot]
	if err := readGasPrice(r, value); err != nil {
		return err
	}
	*dst = value
	return nil
}
