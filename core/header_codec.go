package core

import "github.com/NethermindEth/juno/core/felt"

// CodecMissingField handles legacy header fields for Ugorji.
func (h *Header) CodecMissingField(field []byte, value any) bool {
	var target **felt.Felt
	switch string(field) {
	case "GasPrice":
		target = &h.L1GasPriceETH
	case "GasPriceSTRK":
		target = &h.L1GasPriceSTRK
	case "ExtraData":
	default:
		return false
	}
	legacy, ok := genericFelt(value)
	if ok && legacy != nil && target != nil {
		*target = legacy
	}
	return ok
}

// CodecMissingFields completes Ugorji's MissingFielder interface.
func (h *Header) CodecMissingFields() map[string]any { return nil }

// genericFelt converts an array of four unsigned limbs into a felt.
func genericFelt(value any) (*felt.Felt, bool) {
	if value == nil {
		return nil, true
	}
	limbs, ok := value.([]any)
	if !ok || len(limbs) != felt.Limbs {
		return nil, false
	}
	var f felt.Felt
	for i, limb := range limbs {
		if f[i], ok = limb.(uint64); !ok {
			return nil, false
		}
	}
	return &f, true
}
