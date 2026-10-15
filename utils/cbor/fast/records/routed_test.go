package records_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core"
)

// routed is a type the fast decoder reads, as the tests that run over every one of them see it.
// Adding a routed type to routedTypes gives it the adversarial, random-value and fuzz seed
// tests; it needs a one-line fuzz target in fuzz_test.go, which names it.
type routed struct {
	// name is what records of the type are called: receipt, header, state, transaction or class.
	name string
	// pointer is whether decodes into a nil *T, the **T route, are tested. An interface has none.
	pointer bool
	// fixtures are values of the type by name: the built-in samples and the seeds of the fuzz.
	fixtures map[string]any
	// decode decodes data into a fresh value of the type with unmarshal, or with pointer into a
	// nil *T.
	decode func(data []byte, pointer bool, unmarshal func([]byte, any) error) (any, error)
	// random checks that random values of the type decode as fxamacker decodes them.
	random func(t *testing.T)
}

func routedType[T any](name string, fixtures map[string]any) routed {
	return routed{
		name:     name,
		pointer:  reflect.TypeFor[T]().Kind() != reflect.Interface,
		fixtures: fixtures,
		decode:   into[T],
		random:   checkRandomValues[T],
	}
}

func into[T any](data []byte, pointer bool, unmarshal func([]byte, any) error) (any, error) {
	if pointer {
		var dst *T
		err := unmarshal(data, &dst)
		return dst, err
	}
	var dst T
	err := unmarshal(data, &dst)
	return dst, err
}

var routedTypes = []routed{
	routedType[core.TransactionReceipt]("receipt", map[string]any{
		"receipt": receipt(), "zero receipt": &core.TransactionReceipt{},
	}),
	routedType[core.Header]("header", map[string]any{
		"header": header(), "header with an events bloom": oldHeader(), "zero header": &core.Header{},
	}),
	routedType[core.StateUpdate]("state", map[string]any{
		"state": stateUpdate(), "zero state": &core.StateUpdate{},
		"empty diff state": &core.StateUpdate{StateDiff: &core.StateDiff{}},
	}),
	routedType[core.Transaction]("transaction", transactionFixtures()),
	routedType[core.ClassDefinition]("class", map[string]any{
		"class": sierraClass(), "cairo 0 class": &core.DeprecatedCairoClass{Abi: []byte(`[]`), Program: "p"},
	}),
}

func transactionFixtures() map[string]any {
	out := map[string]any{}
	for name, tx := range transactions() {
		out["transaction "+name] = tx
	}
	return out
}

// routedFor returns the routed type that a record is of, by the name of its file or golden
// vector, or nil.
func routedFor(name string) *routed {
	lower := strings.ToLower(name)
	kind := ""
	switch {
	case strings.Contains(lower, "receipt"):
		kind = "receipt"
	case strings.HasPrefix(lower, "header"):
		kind = "header"
	case strings.HasPrefix(lower, "state"):
		kind = "state"
	case strings.Contains(lower, "class"):
		kind = "class"
	case strings.Contains(lower, "transaction") && !strings.Contains(lower, "indexes"):
		kind = "transaction"
	}
	if kind == "" {
		return nil
	}
	return routedByName(kind)
}

// routedByName returns the routed type of that name.
func routedByName(name string) *routed {
	i := slices.IndexFunc(routedTypes, func(r routed) bool { return r.name == name })
	if i < 0 {
		panic("no routed type " + name)
	}
	return &routedTypes[i]
}
