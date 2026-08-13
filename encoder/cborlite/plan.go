package cborlite

import (
	"fmt"
	"reflect"
	"strings"
)

type planField struct {
	index int
	reader
}

// A plan is what a struct type becomes: its fields, keyed by the name they have on the
// wire, each with the reader that fills it.
type plan struct {
	fields            map[string]planField
	rejectUnknownKeys bool
	name              string
}

func planFor(structType reflect.Type, strict bool) (*plan, error) {
	key := cacheKey{valueType: structType, strict: strict}

	if cached, ok := plans[key]; ok {
		return cached, nil
	}
	if buildingPlan, ok := buildingPlans[key]; ok {
		return buildingPlan, nil
	}

	built := &plan{
		fields:            make(map[string]planField, structType.NumField()),
		name:              structType.String(),
		rejectUnknownKeys: strict,
	}
	buildingPlans[key] = built
	defer delete(buildingPlans, key)

	// A non-empty type without exported fields is not valid.
	if structType.NumField() > 0 && !hasExportedField(structType) {
		return nil, fmt.Errorf("%s: no exported fields to read", structType)
	}

	for index := range structType.NumField() {
		field := structType.Field(index)

		if field.Anonymous {
			return nil, fmt.Errorf("%s.%s: embedded fields are not supported",
				structType, field.Name)
		}

		if !field.IsExported() {
			continue
		}

		// check for unsupported CBOR keys
		name, supported := cborKey(&field)
		if !supported {
			return nil, fmt.Errorf("%s.%s: unsupported cbor tag %q",
				structType, field.Name, field.Tag.Get("cbor"))
		}

		reader, err := buildReader(field.Type, strict)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", structType, field.Name, err)
		}

		built.fields[name] = planField{index: index, reader: reader}
	}

	plans[key] = built
	return built, nil
}

func hasExportedField(structType reflect.Type) bool {
	for index := range structType.NumField() {
		if structType.Field(index).IsExported() {
			return true
		}
	}
	return false
}

// cborKey gets name of a field, the CBOR tag name when it has one, else the Go field name.
// Fails if a CBOR tag is not supported.
func cborKey(field *reflect.StructField) (name string, supported bool) {
	tag, ok := field.Tag.Lookup("cbor")
	if !ok {
		// Fields without tags return the Go name.
		return field.Name, true
	}

	tagName, options, _ := strings.Cut(tag, ",")
	// The tag says the field is not on the wire, which this package does not implement.
	if tagName == "-" {
		return "", false
	}

	for option := range strings.SplitSeq(options, ",") {
		// The only options we support at the moment.
		if option != "" && option != "omitempty" {
			return "", false
		}
	}

	if tagName == "" {
		return field.Name, true
	}

	return tagName, true
}

// reader uses the plan to find the field each key goes to.
func (p *plan) reader(target reflect.Value, data []byte) (consumed int, err error) {
	pairsCount, offset, ok := MapHeader(data)
	if !ok {
		return 0, fmt.Errorf("%s: %w", p.name, errShape)
	}

	for range pairsCount {
		key, keyLength, ok := StringNoCopy(data[offset:])
		if !ok {
			return 0, fmt.Errorf("%s: reading a key: %w", p.name, errShape)
		}
		offset += keyLength

		field, known := p.fields[string(key)]
		if !known {
			if p.rejectUnknownKeys {
				return 0, fmt.Errorf("%s: unknown field %q: %w", p.name, key, errShape)
			}

			skippedBytes, ok := Skip(data[offset:])
			if !ok {
				return 0, fmt.Errorf("%s: skipping unknown field %q: %w",
					p.name, key, errShape)
			}
			offset += skippedBytes
			continue
		}

		usedBytes, err := field.reader(target.Field(field.index), data[offset:])
		if err != nil {
			return 0, fmt.Errorf("%s.%s: %w", p.name, key, err)
		}
		offset += usedBytes
	}
	return offset, nil
}
