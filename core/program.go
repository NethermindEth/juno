package core

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/wk8/go-ordered-map/v2"
)

type Program struct {
	Attributes       []interface{}                               `json:"attributes,omitempty"`
	Builtins         []string                                    `json:"builtins"`
	CompilerVersion  interface{}                                 `json:"compiler_version,omitempty"`
	Data             []string                                    `json:"data"`
	DebugInfo        interface{}                                 `json:"debug_info"`
	Hints            *orderedmap.OrderedMap[string, interface{}] `json:"hints,omitempty"`
	Identifiers      interface{}                                 `json:"identifiers,omitempty"`
	MainScope        interface{}                                 `json:"main_scope,omitempty"`
	Prime            interface{}                                 `json:"prime,omitempty"`
	ReferenceManager interface{}                                 `json:"reference_manager"`
}

func (p *Program) Format() error {
	p.Attributes = applyReplacer(p.Attributes, nullSkipReplacer).([]interface{})
	if len(p.Attributes) == 0 {
		p.Attributes = nil
	}
	p.Builtins = applyReplacer(p.Builtins, nullSkipReplacer).([]string)
	if p.CompilerVersion != nil {
		p.CompilerVersion = applyReplacer(p.CompilerVersion, nullSkipReplacer).(string)
	}
	p.DebugInfo = nil
	p.Data = applyReplacer(p.Data, nullSkipReplacer).([]string)

	if err := p.ReorderHints(); err != nil {
		return err
	}
	p.Hints = applyReplacer(p.Hints, nullSkipReplacer).(*orderedmap.OrderedMap[string, interface{}])

	if p.CompilerVersion != nil {
		// Anything since compiler version 0.10.0 can be hashed directly. No extra overhead incurred.
		p.Identifiers = applyReplacer(p.Identifiers, nullSkipReplacer)
	} else {
		// This is needed for backward compatibility with pre-0.10.0 contract artefacts.
		p.Identifiers = applyReplacer(p.Identifiers, identifiersNullSkipReplacer)
	}
	p.MainScope = applyReplacer(p.MainScope, nullSkipReplacer)
	p.Prime = applyReplacer(p.Prime, nullSkipReplacer)
	p.ReferenceManager = applyReplacer(p.ReferenceManager, nullSkipReplacer)

	return nil
}

func (p *Program) ReorderHints() error {
	// Extract keys and convert them to integers
	intKeys := []int{}

	for pair := p.Hints.Oldest(); pair != nil; pair = pair.Next() {
		key := pair.Key
		intKey, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("error converting key to integer: %v", err)
		}
		intKeys = append(intKeys, intKey)
	}

	// Sort the integer keys
	sort.Ints(intKeys)

	// Rebuild the OrderedMap using sorted keys
	newHints := orderedmap.New[string, interface{}]()
	for _, intKey := range intKeys {
		strKey := strconv.Itoa(intKey)
		value, _ := p.Hints.Get(strKey)
		newHints.Set(strKey, value)
	}

	p.Hints = newHints
	return nil
}
