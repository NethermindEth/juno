// Command gen writes the struct readers of package records whose fields all map to a helper by
// their Go type. Run it from records with go generate.
//
// A field type that no entry of known maps, and that is not a struct with a reader here, stops
// the generator with the type and field named, and so does a type with its own CBOR encoding.
// Either is fixed by mapping the type in known after checking how fxamacker stores it, or by
// giving the field an override.
package main

import (
	"encoding"
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"math/big"
	"os"
	"reflect"
	"strings"
	"text/template"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
)

// reader is a struct to write a reader for.
type reader struct {
	typ reflect.Type
	// name is the reader's name. Its field numbers and table take the name without read.
	name string
	// doc follows the name in the reader's comment.
	doc string
	// locals are statements run before the map is opened, for the overrides.
	locals string
	// overrides holds the case of a field, by Go name, for a field no helper reads by type.
	overrides map[string]string
}

// readers lists the readers to write, in the order they are written.
var readers = []reader{
	{typ: reflect.TypeFor[core.TransactionReceipt](), name: "readReceipt", doc: "reads a stored TransactionReceipt."},
	{typ: reflect.TypeFor[core.Event](), name: "readEvent"},
	{typ: reflect.TypeFor[core.L1ToL2Message](), name: "readL1ToL2Message"},
	{typ: reflect.TypeFor[core.L2ToL1Message](), name: "readL2ToL1Message"},
	{typ: reflect.TypeFor[core.DataAvailability](), name: "readDataAvailability"},
	{typ: reflect.TypeFor[core.GasConsumed](), name: "readGasConsumed"},
	{typ: reflect.TypeFor[core.BuiltinInstanceCounter](), name: "readBuiltinInstanceCounter"},
	{
		typ: reflect.TypeFor[core.Header](), name: "readHeader",
		doc: "reads a stored block header. The keys of the legacy fields GasPrice,\n" +
			"GasPriceSTRK and ExtraData are skipped, as fxamacker skips them.",
		locals: "var prices *[2]core.GasPrice",
		overrides: map[string]string{
			// The two gas prices share one allocation.
			"L1DataGasPrice": "err = readGasPricePtr(r, &dst.L1DataGasPrice, &prices, 0)",
			"L2GasPrice":     "err = readGasPricePtr(r, &dst.L2GasPrice, &prices, 1)",
		},
	},
	{typ: reflect.TypeFor[core.GasPrice](), name: "readGasPrice"},
	{typ: reflect.TypeFor[core.StateUpdate](), name: "readStateUpdate", doc: "reads a stored state update."},
	{typ: reflect.TypeFor[core.StateDiff](), name: "readStateDiff"},
	{typ: reflect.TypeFor[core.SierraClass](), name: "readSierraClass", doc: "reads the map of a stored Sierra class, after its tag."},
	{typ: reflect.TypeFor[core.SierraEntryPointsByType](), name: "readSierraEntryPoints"},
	{typ: reflect.TypeFor[core.SierraEntryPoint](), name: "readSierraEntryPoint"},
	{typ: reflect.TypeFor[core.CasmClass](), name: "readCasmClass"},
	{typ: reflect.TypeFor[core.CasmEntryPoint](), name: "readCasmEntryPoint"},
	{
		typ: reflect.TypeFor[core.InvokeTransaction](), name: "readInvoke",
		doc: "reads the map of a stored invoke transaction, after its tag.",
	},
	{
		typ: reflect.TypeFor[core.DeclareTransaction](), name: "readDeclare",
		doc: "reads the map of a stored declare transaction, after its tag.",
	},
	{typ: reflect.TypeFor[core.DeployTransaction](), name: "readDeploy"},
	{typ: reflect.TypeFor[core.DeployAccountTransaction](), name: "readDeployAccount"},
	{typ: reflect.TypeFor[core.L1HandlerTransaction](), name: "readL1Handler"},
	{typ: reflect.TypeFor[core.ResourceBounds](), name: "readBounds"},
}

// known gives the case that reads a field of a type, with the field for %s. Before adding a
// type, check how fxamacker stores it: a type with its own CBOR encoding needs a helper that
// reads that encoding.
var known = map[reflect.Type]string{
	reflect.TypeFor[uint64]():                                      "err = readUint64(r, &%s)",
	reflect.TypeFor[bool]():                                        "err = readBool(r, &%s)",
	reflect.TypeFor[string]():                                      "err = readString(r, &%s)",
	reflect.TypeFor[[]string]():                                    "err = readSlice(r, &%s, readString)",
	reflect.TypeFor[json.RawMessage]():                             "err = readRaw(r, (*[]byte)(&%s))",
	reflect.TypeFor[*felt.Felt]():                                  "err = readFelt(r, &%s)",
	reflect.TypeFor[felt.Slice[felt.Felt]]():                       "err = readFeltSlice(r, &%s)",
	reflect.TypeFor[[]felt.Felt]():                                 "%s, err = fast.LimbSlice[felt.Felt](r)",
	reflect.TypeFor[[]*felt.Felt]():                                "err = readFeltPtrSlice(r, &%s)",
	reflect.TypeFor[[][]*felt.Felt]():                              "err = readSlice(r, &%s, readFeltPtrSlice)",
	reflect.TypeFor[map[felt.Felt]*felt.Felt]():                    "err = readFeltMap(r, &%s)",
	reflect.TypeFor[map[felt.Felt]map[felt.Felt]*felt.Felt]():      "err = readStorageDiffs(r, &%s)",
	reflect.TypeFor[map[felt.SierraClassHash]felt.CasmClassHash](): "err = readMigratedClasses(r, &%s)",
	reflect.TypeFor[eth.Address]():                                 "err = readEthAddress(r, &%s)",
	reflect.TypeFor[*big.Int]():                                    "err = readBigIntPtr(r, &%s)",
	reflect.TypeFor[core.L1DAMode]():                               "err = readUint(r, &%s)",
	reflect.TypeFor[core.FeeUnit]():                                "err = readByte(r, (*byte)(&%s))",
	reflect.TypeFor[core.DataAvailabilityMode]():                   "err = readUint32(r, &%s)",
	reflect.TypeFor[*core.TransactionVersion]():                    "err = readLimbPtr(r, &%s)",
	reflect.TypeFor[map[core.Resource]core.ResourceBounds]():       "err = readResourceBounds(r, &%s)",
	reflect.TypeFor[*core.ExecutionResources]():                    "err = readExecutionResources(r, &%s)",
	reflect.TypeFor[core.SegmentLengths]():                         "err = readSegmentLengths(r, &%s, 0)",
}

// encodings are the interfaces through which a type takes over its own encoding, among those
// fxamacker honors with the options Juno gives it (TextMarshaler is off by default).
var encodings = []reflect.Type{
	reflect.TypeFor[interface{ MarshalCBOR() ([]byte, error) }](),
	reflect.TypeFor[interface{ UnmarshalCBOR([]byte) error }](),
	reflect.TypeFor[encoding.BinaryMarshaler](),
	reflect.TypeFor[encoding.BinaryUnmarshaler](),
}

const (
	// output is the file written, in the directory gen runs in.
	output = "readers_gen.go"
	// sourceMode is the mode it is written with.
	sourceMode = 0o644
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("gen: ")
	src, err := generate()
	if err == nil {
		err = os.WriteFile(output, src, sourceMode)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// generate returns the source of the readers.
func generate() ([]byte, error) {
	names := map[reflect.Type]string{}
	for _, rd := range readers {
		names[rd.typ] = rd.name
	}
	var body strings.Builder
	for _, rd := range readers {
		if err := writeReader(&body, rd, names); err != nil {
			return nil, err
		}
	}
	var src strings.Builder
	err := templates.ExecuteTemplate(&src, "file", fileData{
		Felt: strings.Contains(body.String(), "felt."),
		Body: body.String(),
	})
	if err != nil {
		return nil, err
	}
	return format.Source([]byte(src.String()))
}

// field is a field as fxamacker sees it: its wire name and its Go name and type.
type field struct {
	wire, name string
	typ        reflect.Type
}

// fieldsOf lists the fields of typ the way fxamacker does, with embedded structs promoted.
func fieldsOf(typ reflect.Type) ([]field, error) {
	var out []field
	for i := range typ.NumField() {
		f := typ.Field(i)
		wire, options, _ := strings.Cut(f.Tag.Get("cbor"), ",")
		switch {
		case wire == "-":
			continue
		case f.Anonymous && wire == "" && f.Type.Kind() == reflect.Struct:
			inner, err := fieldsOf(f.Type)
			if err != nil {
				return nil, err
			}
			out = append(out, inner...)
			continue
		case f.Anonymous && wire == "":
			return nil, fmt.Errorf("%v.%s: an embedded %v is not supported", typ, f.Name, f.Type)
		case !f.IsExported():
			continue
		case options != "" && options != "omitempty":
			return nil, fmt.Errorf("%v.%s: the cbor tag option %q is not supported", typ, f.Name, options)
		case wire == "":
			wire = f.Name
		}
		out = append(out, field{wire: wire, name: f.Name, typ: f.Type})
	}
	return out, nil
}

// writeReader writes the field numbers, the field table and the reader of rd.
func writeReader(b *strings.Builder, rd reader, names map[reflect.Type]string) error {
	if err := ownEncoding(rd.typ); err != nil {
		return fmt.Errorf("%v: %w", rd.typ, err)
	}
	fields, err := fieldsOf(rd.typ)
	if err != nil {
		return err
	}
	prefix := strings.TrimPrefix(strings.TrimPrefix(rd.name, "read"), "Read")
	prefix = strings.ToLower(prefix[:1]) + prefix[1:]
	data := readerData{Name: rd.name, Prefix: prefix, Type: rd.typ.String(), Locals: rd.locals}
	if rd.doc != "" {
		data.Doc = rd.name + " " + rd.doc
	}
	used := 0
	for _, f := range fields {
		c, ok := rd.overrides[f.name]
		if ok {
			used++
		} else if c, err = caseOf(f, names); err != nil {
			return fmt.Errorf("%v.%s: %w; map the type in known or override the field", rd.typ, f.name, err)
		}
		data.Fields = append(data.Fields, fieldData{Const: prefix + f.name, Wire: f.wire, Case: c})
	}
	if used != len(rd.overrides) {
		return fmt.Errorf("%v: an override names a field it does not have", rd.typ)
	}
	return templates.ExecuteTemplate(b, "reader", data)
}

// fileData is what the template of the file is filled with.
type fileData struct {
	// Felt is whether the readers use the felt package, which they then import.
	Felt bool
	Body string
}

// readerData is what the template of a reader is filled with.
type readerData struct {
	Name, Prefix, Type string
	// Doc is the whole comment, without the slashes.
	Doc string
	// Locals are statements run before the map is opened.
	Locals string
	Fields []fieldData
}

type fieldData struct {
	// Const is the name of the field's number, Wire its name in the map and Case the statement
	// that reads it.
	Const, Wire, Case string
}

// templates hold the shape of the generated file: the output reads as the template does.
var templates = template.Must(template.New("file").Funcs(template.FuncMap{
	"comment": func(s string) string { return "// " + strings.ReplaceAll(s, "\n", "\n// ") },
}).Parse(`// Code generated by internal/gen; DO NOT EDIT.

package records

import (
	"github.com/NethermindEth/juno/core"
{{- if .Felt}}
	"github.com/NethermindEth/juno/core/felt"
{{- end}}
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

{{.Body}}

{{- define "reader"}}
const (
{{- range $i, $f := .Fields}}
	{{$f.Const}}{{if eq $i 0}} = iota{{end}}
{{- end}}
)

var {{.Prefix}}Fields = fieldsOf[{{.Type}}]([]string{
{{- range .Fields}}
	{{.Const}}: {{printf "%q" .Wire}},
{{- end}}
})

{{if .Doc}}{{comment .Doc}}
{{end -}}
func {{.Name}}(r *fast.Reader, dst *{{.Type}}) error {
{{- if .Locals}}
	{{.Locals}}
{{- end}}
	s, err := r.OpenStruct({{.Prefix}}Fields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
{{- range .Fields}}
		case {{.Const}}:
			{{.Case}}
{{- end}}
		default:
			err = r.Skip()
		}
	}
	return err
}

{{end}}
`))

// caseOf returns the case that reads f: the entry of known for its type or, for a struct with a
// reader here, a call of that reader, through readPtr, readSlice or readPtrSlice if f holds it
// behind a pointer or in a slice.
func caseOf(f field, names map[reflect.Type]string) (string, error) {
	dst := "dst." + f.name
	if format, ok := known[f.typ]; ok {
		return fmt.Sprintf(format, dst), nil
	}
	if err := ownEncoding(f.typ); err != nil {
		return "", err
	}
	t, call := f.typ, "%[2]s(r, &%[1]s)"
	switch {
	case t.Kind() == reflect.Pointer:
		t, call = t.Elem(), "readPtr(r, &%s, %s)"
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Pointer:
		t, call = t.Elem().Elem(), "readPtrSlice(r, &%s, %s)"
	case t.Kind() == reflect.Slice:
		t, call = t.Elem(), "readSlice(r, &%s, %s)"
	}
	if name, ok := names[t]; ok {
		return "err = " + fmt.Sprintf(call, dst, name), nil
	}
	return "", fmt.Errorf("no helper reads a %v", f.typ)
}

// ownEncoding returns an error if t or a pointer to it encodes itself.
func ownEncoding(t reflect.Type) error {
	for _, e := range encodings {
		if t.Implements(e) || reflect.PointerTo(t).Implements(e) {
			return fmt.Errorf("%v has its own encoding (%s)", t, e.Method(0).Name)
		}
	}
	return nil
}
