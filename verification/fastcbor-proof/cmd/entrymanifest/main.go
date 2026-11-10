// Command entrymanifest inventories build-selected production fast-CBOR roots.
// It describes Go shapes and call sites, not a proved wire schema or route smoke.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

const module = "github.com/NethermindEth/juno"
const core = module + "/core."
const output = "verification/fastcbor-proof/results/entry-manifest.json"

type Site struct {
	File string `json:"file"`
	Line int    `json:"line"`
	API  string `json:"api"`
	Type string `json:"type"`
}
type Root struct {
	Type       string   `json:"type"`
	Sites      []Site   `json:"production_sites"`
	Benchmarks []string `json:"benchmark_cases"`
	Typed      bool     `json:"typed_registered"`
	Status     string   `json:"proof_status"`
}
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Embedded bool   `json:"embedded"`
	Exported bool   `json:"exported"`
	CBOR     string `json:"cbor_tag,omitempty"`
	JSON     string `json:"json_tag,omitempty"`
}
type Variant struct {
	Type          string `json:"type"`
	RegistryIndex int    `json:"registry_index"`
	Site          Site   `json:"site"`
}
type Shape struct {
	Type         string   `json:"type"`
	Kind         string   `json:"kind"`
	Source       *Site    `json:"source,omitempty"`
	Dependencies []string `json:"dependencies"`
	Fields       []Field  `json:"fields,omitempty"`
	Hooks        []string `json:"decode_hooks,omitempty"`
	Variants     []string `json:"registered_pointer_variants,omitempty"`
	Boundary     string   `json:"boundary,omitempty"`
}
type Benchmark struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Roots  []string `json:"decoder_roots"`
	Status string   `json:"status"`
}
type Report struct {
	Scope         string            `json:"scope"`
	Target        string            `json:"target"`
	Packages      []string          `json:"packages"`
	Roots         []*Root           `json:"roots"`
	GenericSites  []Site            `json:"generic_forwarders"`
	Registrations []Variant         `json:"tag_registry_order"`
	Shapes        []*Shape          `json:"shapes"`
	Benchmarks    []Benchmark       `json:"benchmarks"`
	Sources       map[string]string `json:"source_sha256"`
	Limitations   []string          `json:"limitations"`
}
type Inventory struct {
	root     string
	fset     *token.FileSet
	report   Report
	roots    map[string]*Root
	nodes    map[string]*Shape
	types    map[string]types.Type
	variants []types.Type
}

func main() {
	check := flag.Bool("check", false, "reject drift from the committed manifest")
	flag.Parse()
	if err := run(*check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func typeID(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return p.Path() })
}
func objectID(o types.Object) string {
	if o == nil || o.Pkg() == nil {
		return ""
	}
	return o.Pkg().Path() + "." + o.Name()
}
func (v *Inventory) source(pos token.Pos, api, typ string) Site {
	p := v.fset.Position(pos)
	name, _ := filepath.Rel(v.root, p.Filename)
	return Site{filepath.ToSlash(name), p.Line, api, typ}
}
func (v *Inventory) hash(path string) error {
	if _, ok := v.report.Sources[path]; ok {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(v.root, path))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	v.report.Sources[path] = hex.EncodeToString(sum[:])
	return nil
}
func (v *Inventory) addRoot(t types.Type) *Root {
	id := typeID(t)
	if r := v.roots[id]; r != nil {
		return r
	}
	r := &Root{Type: id, Sites: []Site{}, Benchmarks: []string{}, Status: "typed reader and general proof pending"}
	v.roots[id] = r
	v.types[id] = t
	return r
}
func hasParam(t types.Type) bool {
	switch t := types.Unalias(t).(type) {
	case *types.TypeParam:
		return true
	case *types.Pointer:
		return hasParam(t.Elem())
	case *types.Slice:
		return hasParam(t.Elem())
	case *types.Array:
		return hasParam(t.Elem())
	case *types.Map:
		return hasParam(t.Key()) || hasParam(t.Elem())
	case *types.Chan:
		return hasParam(t.Elem())
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if hasParam(t.Field(i).Type()) {
				return true
			}
		}
	case *types.Signature:
		if t.TypeParams().Len() != 0 {
			return true
		}
		for _, tuple := range []*types.Tuple{t.Params(), t.Results()} {
			for i := 0; i < tuple.Len(); i++ {
				if hasParam(tuple.At(i).Type()) {
					return true
				}
			}
		}
	case *types.Interface:
		for i := 0; i < t.NumExplicitMethods(); i++ {
			if hasParam(t.ExplicitMethod(i).Type()) {
				return true
			}
		}
		for i := 0; i < t.NumEmbeddeds(); i++ {
			if hasParam(t.EmbeddedType(i)) {
				return true
			}
		}
	case *types.Named:
		for i := 0; i < t.TypeArgs().Len(); i++ {
			if hasParam(t.TypeArgs().At(i)) {
				return true
			}
		}
	}
	return false
}
func run(check bool) error {
	if runtime.Version() != "go1.27.0" {
		return fmt.Errorf("manifest requires Go 1.27.0, got %s", runtime.Version())
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "core/typed_buckets.go")); err != nil {
		return fmt.Errorf("run from repository root: %w", err)
	}
	v := &Inventory{root: root, fset: token.NewFileSet(), roots: map[string]*Root{}, nodes: map[string]*Shape{}, types: map[string]types.Type{}}
	v.report = Report{Scope: "Build-selected production fast-CBOR destinations, registered interface variants, structural Go dependencies and all frozen benchmark cases. Checked inventory; Header remains conditional, its five projections have typed implementations, and remaining record proofs are open.", Target: "linux/amd64/v1, Go 1.27.0, tests excluded", Sources: map[string]string{}, GenericSites: []Site{}, Limitations: []string{
		"Go shape dependencies are not a wire schema. Embedding precedence, custom hooks, integer keys, projections, overrides and special allocation require independent contracts.",
		"Concrete FastCbor, fastcbor.Unmarshal/UnmarshalFirst, and indexed.LazySlice/NewLazySlice instantiations are inventoried. Generic forwarding sites are explicit; arbitrary runtime types are outside the agreed decoder scope.",
		"Tag registry order is extracted from the actual registration list. Numeric tag allocation and initialization still need a checked contract; registry membership is not a proof of interface decoding.",
		"Historical benchmark mappings are reviewed associations, not execution/compatibility evidence. Compound block operations may invoke multiple readers; FX fallback must be excluded by the later smoke and timing harness.",
		"External named types and felt/big.Int/custom-hook representations are explicit proof boundaries, not assumed-correct decoders.",
		"Routing, registration, FX fallback, diagnostic formatting, unrelated serialization and non-fast CBOR callers are outside the decoder proof scope.",
	}}
	ps, err := packages.Load(&packages.Config{Fset: v.fset, Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo, Env: append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "GOAMD64=v1", "GOWORK=off")}, "./...")
	if err != nil {
		return err
	}
	if packages.PrintErrors(ps) != 0 {
		return fmt.Errorf("production package loading failed; refusing a partial manifest")
	}
	var corePkg *packages.Package
	for _, p := range ps {
		v.report.Packages = append(v.report.Packages, p.PkgPath)
		if p.PkgPath == module+"/core" {
			corePkg = p
		}
		for id, inst := range p.TypesInfo.Instances {
			api := objectID(p.TypesInfo.Uses[id])
			typed := api == module+"/utils/cbor/fastcbor.RegisterTyped"
			switch api {
			case module + "/db/typed/value.FastCbor", module + "/utils/cbor/fastcbor.Unmarshal", module + "/utils/cbor/fastcbor.UnmarshalFirst", module + "/utils/cbor/fastcbor.RegisterTyped", module + "/core/indexed.LazySlice", module + "/core/indexed.NewLazySlice":
			default:
				continue
			}
			if inst.TypeArgs.Len() != 1 {
				return fmt.Errorf("unexpected type parameters at %s", v.fset.Position(id.Pos()))
			}
			t := inst.TypeArgs.At(0)
			site := v.source(id.Pos(), api, typeID(t))
			if err := v.hash(site.File); err != nil {
				return err
			}
			if hasParam(t) {
				v.report.GenericSites = append(v.report.GenericSites, site)
				continue
			}
			r := v.addRoot(t)
			if typed {
				r.Typed = true
				r.Status = "registered typed reader; general decoder proof pending"
				if r.Type == core+"Header" {
					r.Status = "Header API composition checked conditional on UTF-8 word helper; unconditional milestone open"
				}
			} else {
				r.Sites = append(r.Sites, site)
			}
		}
		if p.PkgPath == module+"/utils/cbor/registry" {
			for _, file := range p.Syntax {
				ast.Inspect(file, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || objectID(p.TypesInfo.Uses[sel.Sel]) != "reflect.TypeOf" {
						return true
					}
					if len(call.Args) != 1 {
						return true
					}
					t := p.TypesInfo.TypeOf(call.Args[0])
					site := v.source(call.Pos(), "registry reflect.TypeOf", typeID(t))
					v.report.Registrations = append(v.report.Registrations, Variant{typeID(t), len(v.variants), site})
					v.variants = append(v.variants, t)
					v.types[typeID(t)] = t
					return true
				})
			}
		}
	}
	if corePkg == nil {
		return fmt.Errorf("core package missing")
	}
	// Map every historical case, including unavailable/control cases, without dropping it.
	var universe struct{ Cases []struct{ ID, Kind string } }
	b, err := os.ReadFile("verification/fastcbor-proof/results/benchmark-universe.json")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(b, &universe); err != nil {
		return err
	}
	for _, c := range universe.Cases {
		names, status, err := benchmarkRoots(c.Kind)
		if err != nil {
			return err
		}
		row := Benchmark{ID: c.ID, Kind: c.Kind, Roots: []string{}, Status: status}
		for _, name := range names {
			pointer := strings.HasPrefix(name, "*")
			name = strings.TrimPrefix(name, "*")
			obj := corePkg.Types.Scope().Lookup(name)
			if obj == nil {
				return fmt.Errorf("benchmark %s requires absent core.%s", c.ID, name)
			}
			var t types.Type = obj.Type()
			if pointer {
				t = types.NewPointer(t)
			}
			r := v.addRoot(t)
			r.Benchmarks = append(r.Benchmarks, c.ID)
			row.Roots = append(row.Roots, r.Type)
		}
		v.report.Benchmarks = append(v.report.Benchmarks, row)
	}
	if len(v.report.Benchmarks) != 185 {
		return fmt.Errorf("frozen case count changed: %d", len(v.report.Benchmarks))
	}
	for _, r := range v.roots {
		v.walk(v.types[r.Type])
		slices.SortFunc(r.Sites, compareSites)
		slices.Sort(r.Benchmarks)
		v.report.Roots = append(v.report.Roots, r)
	}
	for _, n := range v.nodes {
		v.report.Shapes = append(v.report.Shapes, n)
		if n.Source != nil {
			if err := v.hash(n.Source.File); err != nil {
				return err
			}
		}
	}
	for _, variant := range v.report.Registrations {
		if err := v.hash(variant.Site.File); err != nil {
			return err
		}
	}
	for _, file := range []string{"go.mod", "go.sum", "verification/fastcbor-proof/cmd/entrymanifest/main.go", "verification/fastcbor-proof/results/benchmark-universe.json", "utils/cbor/fxamacker/cbor.go", "utils/cbor/fastcbor/readers.go", "utils/cbor/fastcbor/plan.go", "utils/cbor/fastcbor/generic.go", "utils/cbor/fastcbor/state_update.go", "utils/cbor/cmd/cborgen/main.go", "core/block_transaction_serializer.go"} {
		if err := v.hash(file); err != nil {
			return err
		}
	}
	slices.Sort(v.report.Packages)
	slices.SortFunc(v.report.Roots, func(a, b *Root) int { return strings.Compare(a.Type, b.Type) })
	slices.SortFunc(v.report.Shapes, func(a, b *Shape) int { return strings.Compare(a.Type, b.Type) })
	slices.SortFunc(v.report.GenericSites, compareSites)
	slices.SortFunc(v.report.Benchmarks, func(a, b Benchmark) int { return strings.Compare(a.ID, b.ID) })
	data, err := json.MarshalIndent(v.report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if check {
		old, err := os.ReadFile(output)
		if err != nil {
			return err
		}
		if !bytes.Equal(old, data) {
			return fmt.Errorf("%s is stale; run go run ./verification/fastcbor-proof/cmd/entrymanifest", output)
		}
	} else if err := os.WriteFile(output, data, 0644); err != nil {
		return err
	}
	fmt.Printf("Checked inventory: %d roots, %d shapes, %d registry entries, %d benchmarks, %d production packages\n", len(v.report.Roots), len(v.report.Shapes), len(v.report.Registrations), len(v.report.Benchmarks), len(ps))
	return nil
}
func compareSites(a, b Site) int {
	if n := strings.Compare(a.File, b.File); n != 0 {
		return n
	}
	if a.Line != b.Line {
		return a.Line - b.Line
	}
	if n := strings.Compare(a.API, b.API); n != 0 {
		return n
	}
	return strings.Compare(a.Type, b.Type)
}

func (v *Inventory) walk(t types.Type) {
	id := typeID(t)
	if v.nodes[id] != nil {
		return
	}
	n := &Shape{Type: id, Dependencies: []string{}}
	v.nodes[id] = n
	dep := func(t types.Type) { key := typeID(t); n.Dependencies = append(n.Dependencies, key); v.walk(t) }
	if a, ok := t.(*types.Alias); ok {
		n.Kind = "alias"
		obj := a.Obj()
		if obj.Pkg() != nil && strings.HasPrefix(obj.Pkg().Path(), module+"/") {
			site := v.source(obj.Pos(), "alias declaration", id)
			n.Source = &site
		}
		dep(types.Unalias(a))
		return
	}
	switch t := t.(type) {
	case *types.Named:
		n.Kind = "named"
		obj := t.Obj()
		site := v.source(obj.Pos(), "type declaration", id)
		if obj.Pkg() != nil && strings.HasPrefix(obj.Pkg().Path(), module+"/") {
			n.Source = &site
		}
		for _, candidate := range []types.Type{t, types.NewPointer(t)} {
			set := types.NewMethodSet(candidate)
			for i := 0; i < set.Len(); i++ {
				method := set.At(i).Obj()
				if method.Name() == "UnmarshalCBOR" || method.Name() == "UnmarshalBinary" {
					n.Hooks = append(n.Hooks, typeID(candidate)+"."+method.Name())
				}
			}
		}
		if obj.Pkg() == nil || !strings.HasPrefix(obj.Pkg().Path(), module+"/") {
			n.Boundary = "external named representation; requires an explicit decoder/helper contract"
			break
		}
		if obj.Pkg().Path() == module+"/core/felt" {
			n.Boundary = "felt or felt-slice custom wire representation; use explicit felt/slice contracts"
			break
		}
		if id == core+"discardedCBOR" {
			n.Boundary = "projection discard marker; wire value is skipped, not decoded as an empty struct"
			break
		}
		dep(t.Underlying())
	case *types.Pointer:
		n.Kind = "pointer"
		dep(t.Elem())
	case *types.Slice:
		n.Kind = "slice"
		dep(t.Elem())
	case *types.Array:
		n.Kind = fmt.Sprintf("array[%d]", t.Len())
		dep(t.Elem())
	case *types.Map:
		n.Kind = "map"
		dep(t.Key())
		dep(t.Elem())
	case *types.Basic:
		n.Kind = t.Name()
	case *types.Struct:
		n.Kind = "struct"
		for i := 0; i < t.NumFields(); i++ {
			f := t.Field(i)
			tag := reflect.StructTag(t.Tag(i))
			n.Fields = append(n.Fields, Field{f.Name(), typeID(f.Type()), f.Embedded(), f.Exported(), tag.Get("cbor"), tag.Get("json")})
			if f.Exported() || f.Embedded() {
				if strings.Split(tag.Get("cbor"), ",")[0] != "-" {
					dep(f.Type())
				}
			}
		}
	case *types.Interface:
		n.Kind = "interface"
		t.Complete()
		for _, variant := range v.variants {
			ptr := types.NewPointer(variant)
			if types.Implements(ptr, t) {
				n.Variants = append(n.Variants, typeID(ptr))
				dep(ptr)
			}
		}
		n.Boundary = "only registered pointer variants are enumerated; generic/dynamic interface handling is not a proof"
	default:
		n.Kind = fmt.Sprintf("%T", t)
		n.Boundary = "unclassified Go shape; feasibility must be established"
	}
	slices.Sort(n.Dependencies)
	n.Dependencies = slices.Compact(n.Dependencies)
	slices.Sort(n.Hooks)
	n.Hooks = slices.Compact(n.Hooks)
	slices.Sort(n.Variants)
}
func benchmarkRoots(kind string) ([]string, string, error) {
	status := "compatibility evidence is recorded separately; timing and allocation gate pending"
	simple := map[string]string{"record.header": "Header", "record.state": "StateUpdate", "record.receipt": "TransactionReceipt", "record.class": "ClassDefinition", "indexes": "BlockTransactionsIndexes", "projection.headerHash": "headerHashProjection", "projection.headerRoot": "headerGlobalStateRootProjection", "projection.headerCount": "headerTransactionCountProjection", "projection.headerTimestamp": "headerTimestampProjection", "projection.headerHashRoot": "headerHashAndStateRootProjection", "projection.receiptStatus": "receiptExecutionStatusProjection", "projection.receiptEvents": "receiptEventsProjection", "projection.transactionHash": "transactionHashProjection"}
	if name, ok := simple[kind]; ok {
		return []string{name}, status, nil
	}
	switch kind {
	case "record.transactionDeclare", "record.transactionDeploy", "record.transactionDeployAccount", "record.transactionInvoke", "record.transactionL1Handler":
		return []string{"Transaction"}, status, nil
	}
	if kind == "projection.headerBloom" {
		return nil, "historical projection absent from current Header; retained for explicit incompatibility classification, not silently dropped", nil
	}
	block := map[string][]string{
		"fullSerialized": {"BlockTransactionsIndexes"}, "allTransactions": {"BlockTransactionsIndexes", "Transaction"}, "allReceipts": {"BlockTransactionsIndexes", "*TransactionReceipt"}, "allPairs": {"BlockTransactionsIndexes", "Transaction", "*TransactionReceipt"}, "allEvents": {"BlockTransactionsIndexes", "receiptEventsProjection"}, "allHashes": {"BlockTransactionsIndexes", "transactionHashProjection"}, "lazyAllReceipts": {"*TransactionReceipt"}, "lazyAllTransactions": {"Transaction"}, "oneTransaction": {"BlockTransactionsIndexes", "Transaction"}, "lazyTransaction": {"Transaction"}, "oneReceipt": {"BlockTransactionsIndexes", "*TransactionReceipt"}, "lazyReceipt": {"*TransactionReceipt"}, "onePair": {"BlockTransactionsIndexes", "Transaction", "*TransactionReceipt"}, "oneStatus": {"BlockTransactionsIndexes", "receiptExecutionStatusProjection"},
	}
	if names, ok := block[strings.TrimPrefix(kind, "block.")]; ok && strings.HasPrefix(kind, "block.") {
		return names, status, nil
	}
	return nil, "", fmt.Errorf("unclassified benchmark kind %q", kind)
}
