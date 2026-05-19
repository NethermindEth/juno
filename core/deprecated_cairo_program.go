package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

// This file reimplements the legacy Cairo0 hinted class hash serialization
// used by starknet-core's legacy contract hashing path in pure Go.
//
// Main upstream reference:
// https://docs.rs/starknet-core/latest/src/starknet_core/types/contract/legacy.rs.html
//
// The helpers below intentionally preserve the legacy field ordering, escaping,
// omitted-field rules, and pre-0.10 Cairo quirks required for hash
// compatibility.
type deprecatedCairoProgram struct {
	Attributes      []legacyAttribute `json:"attributes,omitempty"`
	Builtins        []string          `json:"builtins"`
	CompilerVersion *string           `json:"compiler_version,omitempty"`
	Data            []felt.Felt       `json:"data"`
	// debug_info is accepted from input artifacts but always serialized as null
	// for hinted class hashing, matching legacy behavior.
	DebugInfo        json.RawMessage        `json:"debug_info"`
	Hints            legacyHints            `json:"hints"`
	Identifiers      legacyIdentifiers      `json:"identifiers"`
	MainScope        string                 `json:"main_scope"`
	Prime            string                 `json:"prime"`
	ReferenceManager legacyReferenceManager `json:"reference_manager"`
}

func unmarshalDeprecatedCairoProgram(raw json.RawMessage) (*deprecatedCairoProgram, error) {
	program := new(deprecatedCairoProgram)
	if err := json.Unmarshal(raw, program); err != nil {
		return nil, err
	}
	return program, nil
}

// computeHintedClassHash writes the canonical legacy {"abi": ..., "program": ...}
// payload directly into a buffer before hashing it with StarknetKeccak.
func computeHintedClassHash(
	abi json.RawMessage,
	program *deprecatedCairoProgram,
) (felt.Felt, error) {
	legacyABI, err := parseLegacyABI(abi)
	if err != nil {
		return felt.Felt{}, err
	}

	var buffer bytes.Buffer
	if err := writeHintedClassHashInput(
		&buffer,
		legacyABI,
		program,
	); err != nil {
		return felt.Felt{}, err
	}

	return crypto.StarknetKeccak(buffer.Bytes()), nil
}

func writeHintedClassHashInput(
	buffer *bytes.Buffer,
	legacyABI []legacyABIEntry,
	program *deprecatedCairoProgram,
) error {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "abi", &first)
	if err := writeLegacyABIEntries(buffer, legacyABI); err != nil {
		return err
	}
	writeJSONFieldPrefix(buffer, "program", &first)
	if err := writeDeprecatedCairoProgramCanonical(buffer, program); err != nil {
		return err
	}
	buffer.WriteByte('}')
	return nil
}

func writeJSONFieldPrefix(buffer *bytes.Buffer, name string, first *bool) {
	if !*first {
		buffer.WriteString(", ")
	}
	*first = false
	writeJSONString(buffer, name)
	buffer.WriteString(": ")
}

// writeJSONString mirrors the legacy JSON escaping used by the upstream Rust
// serialization path, including UTF-16 escaping for non-ASCII runes.
func writeJSONString(buffer *bytes.Buffer, value string) {
	buffer.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\', '"':
			buffer.WriteByte('\\')
			buffer.WriteRune(r)
		case '\b':
			buffer.WriteString(`\b`)
		case '\f':
			buffer.WriteString(`\f`)
		case '\n':
			buffer.WriteString(`\n`)
		case '\r':
			buffer.WriteString(`\r`)
		case '\t':
			buffer.WriteString(`\t`)
		default:
			switch {
			case r < ' ':
				writeJSONHex16(buffer, uint16(r))
			case r < utf8.RuneSelf:
				buffer.WriteByte(byte(r))
			default:
				for _, u16 := range utf16.Encode([]rune{r}) {
					writeJSONHex16(buffer, u16)
				}
			}
		}
	}
	buffer.WriteByte('"')
}

func writeJSONHex16(buffer *bytes.Buffer, value uint16) {
	const hexChars = "0123456789abcdef"

	buffer.WriteString(`\u`)
	buffer.WriteByte(hexChars[(value>>12)&0xf])
	buffer.WriteByte(hexChars[(value>>8)&0xf])
	buffer.WriteByte(hexChars[(value>>4)&0xf])
	buffer.WriteByte(hexChars[value&0xf])
}

func writeJSONUint64(buffer *bytes.Buffer, value uint64) {
	buffer.WriteString(strconv.FormatUint(value, 10))
}

func writeJSONRaw(buffer *bytes.Buffer, raw json.RawMessage) {
	if len(raw) == 0 {
		buffer.WriteString("null")
		return
	}
	buffer.Write(raw)
}

func writeJSONFelt(buffer *bytes.Buffer, value *felt.Felt) {
	writeJSONString(buffer, value.String())
}

func writeJSONStringArray(buffer *bytes.Buffer, values []string) {
	if values == nil {
		buffer.WriteString("null")
		return
	}
	buffer.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			buffer.WriteString(", ")
		}
		writeJSONString(buffer, value)
	}
	buffer.WriteByte(']')
}

func writeTypedParameters(buffer *bytes.Buffer, values []legacyTypedParameter) {
	if values == nil {
		buffer.WriteString("null")
		return
	}
	buffer.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			buffer.WriteString(", ")
		}
		writeTypedParameter(buffer, value)
	}
	buffer.WriteByte(']')
}

func writeTypedParameter(buffer *bytes.Buffer, value legacyTypedParameter) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "name", &first)
	writeJSONString(buffer, value.Name)
	writeJSONFieldPrefix(buffer, "type", &first)
	writeJSONString(buffer, value.Type)
	buffer.WriteByte('}')
}

func writeABIMembers(buffer *bytes.Buffer, values []legacyABIMember) {
	if values == nil {
		buffer.WriteString("null")
		return
	}
	buffer.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteByte('{')
		first := true
		writeJSONFieldPrefix(buffer, "name", &first)
		writeJSONString(buffer, value.Name)
		writeJSONFieldPrefix(buffer, "offset", &first)
		writeJSONUint64(buffer, value.Offset)
		writeJSONFieldPrefix(buffer, "type", &first)
		writeJSONString(buffer, value.Type)
		buffer.WriteByte('}')
	}
	buffer.WriteByte(']')
}

// Legacy ABI entries are serialized in a fixed variant-specific shape to match
// the upstream hinted-hash payload exactly.
func writeLegacyABIEntries(buffer *bytes.Buffer, entries []legacyABIEntry) error {
	if entries == nil {
		buffer.WriteString("null")
		return nil
	}
	buffer.WriteByte('[')
	for i := range entries {
		if i > 0 {
			buffer.WriteString(", ")
		}
		entry := &entries[i]
		switch entry.Type {
		case "constructor":
			writeLegacyConstructorABIEntry(buffer, &legacyConstructorABIEntry{
				Inputs:  entry.Inputs,
				Name:    entry.Name,
				Outputs: entry.Outputs,
				Type:    entry.Type,
			})
		case "function":
			writeLegacyFunctionABIEntry(buffer, &legacyFunctionABIEntry{
				Inputs:          entry.Inputs,
				Name:            entry.Name,
				Outputs:         entry.Outputs,
				StateMutability: entry.StateMutability,
				Type:            entry.Type,
			})
		case "struct":
			writeLegacyStructABIEntry(buffer, legacyStructABIEntry{
				Members: entry.Members,
				Name:    entry.Name,
				Size:    entry.Size,
				Type:    entry.Type,
			})
		case "l1_handler":
			writeLegacyL1HandlerABIEntry(buffer, &legacyL1HandlerABIEntry{
				Inputs:  entry.Inputs,
				Name:    entry.Name,
				Outputs: entry.Outputs,
				Type:    entry.Type,
			})
		case "event":
			writeLegacyEventABIEntry(buffer, &legacyEventABIEntry{
				Data: entry.Data,
				Keys: entry.Keys,
				Name: entry.Name,
				Type: entry.Type,
			})
		default:
			return fmt.Errorf("unknown legacy ABI entry type %q", entry.Type)
		}
	}
	buffer.WriteByte(']')
	return nil
}

func writeLegacyConstructorABIEntry(buffer *bytes.Buffer, value *legacyConstructorABIEntry) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "inputs", &first)
	writeTypedParameters(buffer, value.Inputs)
	writeJSONFieldPrefix(buffer, "name", &first)
	writeJSONString(buffer, value.Name)
	writeJSONFieldPrefix(buffer, "outputs", &first)
	writeTypedParameters(buffer, value.Outputs)
	writeJSONFieldPrefix(buffer, "type", &first)
	writeJSONString(buffer, value.Type)
	buffer.WriteByte('}')
}

func writeLegacyFunctionABIEntry(buffer *bytes.Buffer, value *legacyFunctionABIEntry) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "inputs", &first)
	writeTypedParameters(buffer, value.Inputs)
	writeJSONFieldPrefix(buffer, "name", &first)
	writeJSONString(buffer, value.Name)
	writeJSONFieldPrefix(buffer, "outputs", &first)
	writeTypedParameters(buffer, value.Outputs)
	if value.StateMutability != nil {
		writeJSONFieldPrefix(buffer, "stateMutability", &first)
		writeJSONString(buffer, *value.StateMutability)
	}
	writeJSONFieldPrefix(buffer, "type", &first)
	writeJSONString(buffer, value.Type)
	buffer.WriteByte('}')
}

func writeLegacyStructABIEntry(buffer *bytes.Buffer, value legacyStructABIEntry) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "members", &first)
	writeABIMembers(buffer, value.Members)
	writeJSONFieldPrefix(buffer, "name", &first)
	writeJSONString(buffer, value.Name)
	writeJSONFieldPrefix(buffer, "size", &first)
	writeJSONUint64(buffer, value.Size)
	writeJSONFieldPrefix(buffer, "type", &first)
	writeJSONString(buffer, value.Type)
	buffer.WriteByte('}')
}

func writeLegacyL1HandlerABIEntry(buffer *bytes.Buffer, value *legacyL1HandlerABIEntry) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "inputs", &first)
	writeTypedParameters(buffer, value.Inputs)
	writeJSONFieldPrefix(buffer, "name", &first)
	writeJSONString(buffer, value.Name)
	writeJSONFieldPrefix(buffer, "outputs", &first)
	writeTypedParameters(buffer, value.Outputs)
	writeJSONFieldPrefix(buffer, "type", &first)
	writeJSONString(buffer, value.Type)
	buffer.WriteByte('}')
}

func writeLegacyEventABIEntry(buffer *bytes.Buffer, value *legacyEventABIEntry) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "data", &first)
	writeTypedParameters(buffer, value.Data)
	writeJSONFieldPrefix(buffer, "keys", &first)
	writeTypedParameters(buffer, value.Keys)
	writeJSONFieldPrefix(buffer, "name", &first)
	writeJSONString(buffer, value.Name)
	writeJSONFieldPrefix(buffer, "type", &first)
	writeJSONString(buffer, value.Type)
	buffer.WriteByte('}')
}

func writeDeprecatedCairoProgramCanonical(
	buffer *bytes.Buffer,
	program *deprecatedCairoProgram,
) error {
	buffer.WriteByte('{')
	first := true
	if len(program.Attributes) > 0 {
		writeJSONFieldPrefix(buffer, "attributes", &first)
		writeLegacyAttributes(buffer, program.Attributes)
	}
	writeJSONFieldPrefix(buffer, "builtins", &first)
	writeJSONStringArray(buffer, program.Builtins)
	if program.CompilerVersion != nil {
		writeJSONFieldPrefix(buffer, "compiler_version", &first)
		writeJSONString(buffer, *program.CompilerVersion)
	}
	writeJSONFieldPrefix(buffer, "data", &first)
	writeFeltArray(buffer, program.Data)
	writeJSONFieldPrefix(buffer, "debug_info", &first)
	// debug_info does not contribute to the legacy hinted class hash.
	buffer.WriteString("null")
	writeJSONFieldPrefix(buffer, "hints", &first)
	if err := writeLegacyHints(buffer, program.Hints); err != nil {
		return err
	}
	writeJSONFieldPrefix(buffer, "identifiers", &first)
	if err := writeLegacyIdentifiers(
		buffer,
		program.Identifiers,
		program.CompilerVersion == nil,
	); err != nil {
		return err
	}
	writeJSONFieldPrefix(buffer, "main_scope", &first)
	writeJSONString(buffer, program.MainScope)
	writeJSONFieldPrefix(buffer, "prime", &first)
	writeJSONString(buffer, program.Prime)
	writeJSONFieldPrefix(buffer, "reference_manager", &first)
	writeLegacyReferenceManager(buffer, program.ReferenceManager)
	buffer.WriteByte('}')
	return nil
}

func writeLegacyAttributes(buffer *bytes.Buffer, values []legacyAttribute) {
	buffer.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteByte('{')
		first := true
		if len(value.AccessibleScopes) > 0 {
			writeJSONFieldPrefix(buffer, "accessible_scopes", &first)
			writeJSONStringArray(buffer, value.AccessibleScopes)
		}
		writeJSONFieldPrefix(buffer, "end_pc", &first)
		writeJSONUint64(buffer, value.EndPC)
		if value.FlowTrackingData != nil {
			writeJSONFieldPrefix(buffer, "flow_tracking_data", &first)
			writeLegacyFlowTrackingData(buffer, *value.FlowTrackingData)
		}
		writeJSONFieldPrefix(buffer, "name", &first)
		writeJSONString(buffer, value.Name)
		writeJSONFieldPrefix(buffer, "start_pc", &first)
		writeJSONUint64(buffer, value.StartPC)
		writeJSONFieldPrefix(buffer, "value", &first)
		writeJSONString(buffer, value.Value)
		buffer.WriteByte('}')
	}
	buffer.WriteByte(']')
}

func writeFeltArray(buffer *bytes.Buffer, values []felt.Felt) {
	if values == nil {
		buffer.WriteString("null")
		return
	}
	buffer.WriteByte('[')
	for i := range values {
		if i > 0 {
			buffer.WriteString(", ")
		}
		writeJSONFelt(buffer, &values[i])
	}
	buffer.WriteByte(']')
}

// Legacy maps are sorted explicitly because Go map iteration is not stable,
// while the upstream Rust path relies on deterministic BTreeMap ordering.
func writeLegacyHints(buffer *bytes.Buffer, hints legacyHints) error {
	if hints == nil {
		buffer.WriteString("{}")
		return nil
	}
	keys := make([]int, 0, len(hints))
	for key := range hints {
		intKey, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("convert hint key %q to integer: %w", key, err)
		}
		keys = append(keys, intKey)
	}
	sort.Ints(keys)
	buffer.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buffer.WriteString(", ")
		}
		writeJSONString(buffer, strconv.Itoa(key))
		buffer.WriteString(": ")
		writeLegacyHintArray(buffer, hints[strconv.Itoa(key)])
	}
	buffer.WriteByte('}')
	return nil
}

func writeLegacyHintArray(buffer *bytes.Buffer, values []legacyHint) {
	if values == nil {
		buffer.WriteString("null")
		return
	}
	buffer.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteByte('{')
		first := true
		writeJSONFieldPrefix(buffer, "accessible_scopes", &first)
		writeJSONStringArray(buffer, value.AccessibleScopes)
		writeJSONFieldPrefix(buffer, "code", &first)
		writeJSONString(buffer, value.Code)
		writeJSONFieldPrefix(buffer, "flow_tracking_data", &first)
		writeLegacyFlowTrackingData(buffer, value.FlowTrackingData)
		buffer.WriteByte('}')
	}
	buffer.WriteByte(']')
}

func writeLegacyFlowTrackingData(buffer *bytes.Buffer, value legacyFlowTrackingData) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "ap_tracking", &first)
	writeLegacyApTrackingData(buffer, value.ApTracking)
	writeJSONFieldPrefix(buffer, "reference_ids", &first)
	writeLegacyReferenceIDs(buffer, value.ReferenceIDs)
	buffer.WriteByte('}')
}

func writeLegacyApTrackingData(buffer *bytes.Buffer, value legacyApTrackingData) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "group", &first)
	writeJSONUint64(buffer, value.Group)
	writeJSONFieldPrefix(buffer, "offset", &first)
	writeJSONUint64(buffer, value.Offset)
	buffer.WriteByte('}')
}

func writeLegacyIdentifiers(
	buffer *bytes.Buffer,
	identifiers legacyIdentifiers,
	patchLegacy bool,
) error {
	if identifiers == nil {
		buffer.WriteString("null")
		return nil
	}
	keys := make([]string, 0, len(identifiers))
	for key := range identifiers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	buffer.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buffer.WriteString(", ")
		}
		writeJSONString(buffer, key)
		buffer.WriteString(": ")
		value := identifiers[key]
		if err := writeLegacyIdentifier(buffer, &value, patchLegacy); err != nil {
			return err
		}
	}
	buffer.WriteByte('}')
	return nil
}

func writeLegacyIdentifier(buffer *bytes.Buffer, value *legacyIdentifier, patchLegacy bool) error {
	buffer.WriteByte('{')
	first := true
	if value.Decorators != nil {
		writeJSONFieldPrefix(buffer, "decorators", &first)
		writeJSONStringArray(buffer, *value.Decorators)
	}
	if value.CairoType != nil {
		writeJSONFieldPrefix(buffer, "cairo_type", &first)
		cairoType := *value.CairoType
		if patchLegacy {
			if patched, changed := patchLegacyCairoType(cairoType); changed {
				cairoType = patched
			}
		}
		writeJSONString(buffer, cairoType)
	}
	if value.FullName != nil {
		writeJSONFieldPrefix(buffer, "full_name", &first)
		writeJSONString(buffer, *value.FullName)
	}
	if value.Members != nil {
		writeJSONFieldPrefix(buffer, "members", &first)
		if err := writeLegacyIdentifierMembers(buffer, *value.Members, patchLegacy); err != nil {
			return err
		}
	}
	if value.References != nil {
		writeJSONFieldPrefix(buffer, "references", &first)
		writeLegacyReferences(buffer, *value.References)
	}
	if value.Size != nil {
		writeJSONFieldPrefix(buffer, "size", &first)
		writeJSONUint64(buffer, *value.Size)
	}
	if value.PC != nil {
		writeJSONFieldPrefix(buffer, "pc", &first)
		writeJSONUint64(buffer, *value.PC)
	}
	if value.Destination != nil {
		writeJSONFieldPrefix(buffer, "destination", &first)
		writeJSONString(buffer, *value.Destination)
	}
	writeJSONFieldPrefix(buffer, "type", &first)
	writeJSONString(buffer, value.Type)
	if len(value.Value) > 0 {
		writeJSONFieldPrefix(buffer, "value", &first)
		writeJSONRaw(buffer, value.Value)
	}
	buffer.WriteByte('}')
	return nil
}

func writeLegacyIdentifierMembers(
	buffer *bytes.Buffer,
	members legacyIdentifierMembers,
	patchLegacy bool,
) error {
	if members == nil {
		buffer.WriteString("null")
		return nil
	}
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	buffer.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buffer.WriteString(", ")
		}
		writeJSONString(buffer, key)
		buffer.WriteString(": ")
		member := members[key]
		buffer.WriteByte('{')
		first := true
		writeJSONFieldPrefix(buffer, "cairo_type", &first)
		cairoType := member.CairoType
		if patchLegacy {
			if patched, changed := patchLegacyCairoType(cairoType); changed {
				cairoType = patched
			}
		}
		writeJSONString(buffer, cairoType)
		writeJSONFieldPrefix(buffer, "offset", &first)
		writeJSONUint64(buffer, member.Offset)
		buffer.WriteByte('}')
	}
	buffer.WriteByte('}')
	return nil
}

func writeLegacyReferences(buffer *bytes.Buffer, values []legacyReference) {
	if values == nil {
		buffer.WriteString("null")
		return
	}
	buffer.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteByte('{')
		first := true
		writeJSONFieldPrefix(buffer, "ap_tracking_data", &first)
		writeLegacyApTrackingData(buffer, value.ApTrackingData)
		writeJSONFieldPrefix(buffer, "pc", &first)
		writeJSONUint64(buffer, value.PC)
		writeJSONFieldPrefix(buffer, "value", &first)
		writeJSONString(buffer, value.Value)
		buffer.WriteByte('}')
	}
	buffer.WriteByte(']')
}

func writeLegacyReferenceIDs(buffer *bytes.Buffer, ids legacyReferenceIDs) {
	if ids == nil {
		buffer.WriteString("null")
		return
	}
	keys := make([]string, 0, len(ids))
	for key := range ids {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	buffer.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buffer.WriteString(", ")
		}
		writeJSONString(buffer, key)
		buffer.WriteString(": ")
		writeJSONUint64(buffer, ids[key])
	}
	buffer.WriteByte('}')
}

func writeLegacyReferenceManager(buffer *bytes.Buffer, manager legacyReferenceManager) {
	buffer.WriteByte('{')
	first := true
	writeJSONFieldPrefix(buffer, "references", &first)
	writeLegacyReferences(buffer, manager.References)
	buffer.WriteByte('}')
}

type legacyAttribute struct {
	AccessibleScopes []string                `json:"accessible_scopes,omitempty"`
	EndPC            uint64                  `json:"end_pc"`
	FlowTrackingData *legacyFlowTrackingData `json:"flow_tracking_data,omitempty"`
	Name             string                  `json:"name"`
	StartPC          uint64                  `json:"start_pc"`
	Value            string                  `json:"value"`
}

type legacyHints map[string][]legacyHint

type legacyHint struct {
	AccessibleScopes []string               `json:"accessible_scopes"`
	Code             string                 `json:"code"`
	FlowTrackingData legacyFlowTrackingData `json:"flow_tracking_data"`
}

type legacyFlowTrackingData struct {
	ApTracking   legacyApTrackingData `json:"ap_tracking"`
	ReferenceIDs legacyReferenceIDs   `json:"reference_ids"`
}

type legacyApTrackingData struct {
	Group  uint64 `json:"group"`
	Offset uint64 `json:"offset"`
}

type legacyIdentifiers map[string]legacyIdentifier

type legacyIdentifier struct {
	Decorators  *[]string                `json:"decorators,omitempty"`
	CairoType   *string                  `json:"cairo_type,omitempty"`
	FullName    *string                  `json:"full_name,omitempty"`
	Members     *legacyIdentifierMembers `json:"members,omitempty"`
	References  *[]legacyReference       `json:"references,omitempty"`
	Size        *uint64                  `json:"size,omitempty"`
	PC          *uint64                  `json:"pc,omitempty"`
	Destination *string                  `json:"destination,omitempty"`
	Type        string                   `json:"type"`
	Value       json.RawMessage          `json:"value,omitempty"`
}

type legacyIdentifierMember struct {
	CairoType string `json:"cairo_type"`
	Offset    uint64 `json:"offset"`
}

type legacyIdentifierMembers map[string]legacyIdentifierMember

type legacyReferenceIDs map[string]uint64

type legacyReferenceManager struct {
	References []legacyReference `json:"references"`
}

type legacyReference struct {
	ApTrackingData legacyApTrackingData `json:"ap_tracking_data"`
	PC             uint64               `json:"pc"`
	Value          string               `json:"value"`
}

// Pre-0.10 legacy artifacts sometimes encoded cairo_type with spaces around
// ":" in the hinted-hash payload. Preserve that quirk for compatibility.
func patchLegacyCairoType(cairoType string) (string, bool) {
	if !strings.Contains(cairoType, ": ") {
		return "", false
	}

	return strings.ReplaceAll(cairoType, ": ", " : "), true
}

// parseLegacyABI keeps only the legacy ABI entry variants that participate in
// Cairo0 hinted class hashing.
func parseLegacyABI(raw json.RawMessage) ([]legacyABIEntry, error) {
	var rawEntries []json.RawMessage
	if err := json.Unmarshal(raw, &rawEntries); err != nil {
		return nil, err
	}

	entries := make([]legacyABIEntry, 0, len(rawEntries))
	for _, rawEntry := range rawEntries {
		var entry legacyABIEntry
		if err := json.Unmarshal(rawEntry, &entry); err != nil {
			return nil, err
		}

		switch entry.Type {
		case "constructor":
			entries = append(entries, entry)
		case "function":
			entries = append(entries, entry)
		case "struct":
			entries = append(entries, entry)
		case "l1_handler":
			entries = append(entries, entry)
		case "event":
			entries = append(entries, entry)
		default:
			return nil, fmt.Errorf("unknown legacy ABI entry type %q", entry.Type)
		}
	}

	return entries, nil
}

type legacyABIEntry struct {
	Type            string                 `json:"type"`
	Name            string                 `json:"name"`
	Inputs          []legacyTypedParameter `json:"inputs"`
	Outputs         []legacyTypedParameter `json:"outputs"`
	StateMutability *string                `json:"stateMutability,omitempty"`
	Members         []legacyABIMember      `json:"members"`
	Size            uint64                 `json:"size"`
	Data            []legacyTypedParameter `json:"data"`
	Keys            []legacyTypedParameter `json:"keys"`
}

type legacyTypedParameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type legacyConstructorABIEntry struct {
	Inputs  []legacyTypedParameter `json:"inputs"`
	Name    string                 `json:"name"`
	Outputs []legacyTypedParameter `json:"outputs"`
	Type    string                 `json:"type"`
}

type legacyFunctionABIEntry struct {
	Inputs          []legacyTypedParameter `json:"inputs"`
	Name            string                 `json:"name"`
	Outputs         []legacyTypedParameter `json:"outputs"`
	StateMutability *string                `json:"stateMutability,omitempty"`
	Type            string                 `json:"type"`
}

type legacyStructABIEntry struct {
	Members []legacyABIMember `json:"members"`
	Name    string            `json:"name"`
	Size    uint64            `json:"size"`
	Type    string            `json:"type"`
}

type legacyL1HandlerABIEntry struct {
	Inputs  []legacyTypedParameter `json:"inputs"`
	Name    string                 `json:"name"`
	Outputs []legacyTypedParameter `json:"outputs"`
	Type    string                 `json:"type"`
}

type legacyEventABIEntry struct {
	Data []legacyTypedParameter `json:"data"`
	Keys []legacyTypedParameter `json:"keys"`
	Name string                 `json:"name"`
	Type string                 `json:"type"`
}

type legacyABIMember struct {
	Name   string `json:"name"`
	Offset uint64 `json:"offset"`
	Type   string `json:"type"`
}
