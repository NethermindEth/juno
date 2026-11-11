import ProofCheck.InitializationState
import ProofCheck.Fields
import ProofCheck.FieldFold

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Exact implementation metadata for the 8 class-package tables.
    These tables do not by themselves specify or prove record payload decoding. -/
inductive ClassSchema where
  | deprecatedEntryPoint
  | deprecatedCairoClass
  | sierraEntryPoint
  | sierraEntryPointsByType
  | casmEntryPoint
  | segmentLengths
  | casmClass
  | sierraClass

def classNames : ClassSchema → List GoString
  | .deprecatedEntryPoint => [go!"Selector", go!"Offset"]
  | .deprecatedCairoClass => [go!"Abi", go!"Externals", go!"L1Handlers", go!"Constructors", go!"Program"]
  | .sierraEntryPoint => [go!"Index", go!"Selector"]
  | .sierraEntryPointsByType => [go!"Constructor", go!"External", go!"L1Handler"]
  | .casmEntryPoint => [go!"Offset", go!"Builtins", go!"Selector"]
  | .segmentLengths => [go!"Children", go!"Length"]
  | .casmClass => [go!"Bytecode", go!"PythonicHints", go!"CompilerVersion", go!"Hints", go!"Prime", go!"External", go!"L1Handler", go!"Constructor", go!"BytecodeSegmentLengths"]
  | .sierraClass => [go!"Abi", go!"AbiHash", go!"EntryPoints", go!"Program", go!"ProgramHash", go!"SemanticVersion", go!"Compiled"]

def classKeys : ClassSchema → List cursor.StaticField
  | .deprecatedEntryPoint => [
      ⟨array.mk 3 [W64 0x74657366664f66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fOffset", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6f7463656c655368, W64 0x72, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hSelector", W64 0, W64 0x1, true⟩]
  | .deprecatedCairoClass => [
      ⟨array.mk 3 [W64 0x69624163, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffff, W64 0x0, W64 0x0],
       go!"cAbi", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6d6172676f725067, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gProgram", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x616e726574784569, W64 0x736c, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iExternals", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6c646e6148314c6a, W64 0x737265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffff, W64 0x0],
       go!"jL1Handlers", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x757274736e6f436c, W64 0x73726f7463, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffff, W64 0x0],
       go!"lConstructors", W64 3, W64 0x8, true⟩]
  | .sierraEntryPoint => [
      ⟨array.mk 3 [W64 0x7865646e4965, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eIndex", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6f7463656c655368, W64 0x72, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hSelector", W64 1, W64 0x2, true⟩]
  | .sierraEntryPointsByType => [
      ⟨array.mk 3 [W64 0x616e726574784568, W64 0x6c, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hExternal", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6c646e6148314c69, W64 0x7265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iL1Handler", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x757274736e6f436b, W64 0x726f7463, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffff, W64 0x0],
       go!"kConstructor", W64 0, W64 0x1, true⟩]
  | .casmEntryPoint => [
      ⟨array.mk 3 [W64 0x74657366664f66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fOffset", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6e69746c69754268, W64 0x73, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hBuiltins", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6f7463656c655368, W64 0x72, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hSelector", W64 2, W64 0x4, true⟩]
  | .segmentLengths => [
      ⟨array.mk 3 [W64 0x6874676e654c66, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffff, W64 0x0, W64 0x0],
       go!"fLength", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6572646c69684368, W64 0x6e, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hChildren", W64 0, W64 0x1, true⟩]
  | .casmClass => [
      ⟨array.mk 3 [W64 0x73746e694865, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"eHints", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x656d69725065, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffff, W64 0x0, W64 0x0],
       go!"ePrime", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x646f636574794268, W64 0x65, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hBytecode", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x616e726574784568, W64 0x6c, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hExternal", W64 5, W64 0x20, true⟩,
      ⟨array.mk 3 [W64 0x6c646e6148314c69, W64 0x7265, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffff, W64 0x0],
       go!"iL1Handler", W64 6, W64 0x40, true⟩,
      ⟨array.mk 3 [W64 0x757274736e6f436b, W64 0x726f7463, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffff, W64 0x0],
       go!"kConstructor", W64 7, W64 0x80, true⟩,
      ⟨array.mk 3 [W64 0x696e6f687479506d, W64 0x73746e694863, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffff, W64 0x0],
       go!"mPythonicHints", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x656c69706d6f436f, W64 0x6e6f697372655672, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oCompilerVersion", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x646f636574794276, W64 0x746e656d67655365, W64 0x736874676e654c],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0xffffffffffffff],
       go!"vBytecodeSegmentLengths", W64 8, W64 0x100, true⟩]
  | .sierraClass => [
      ⟨array.mk 3 [W64 0x69624163, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffff, W64 0x0, W64 0x0],
       go!"cAbi", W64 0, W64 0x1, true⟩,
      ⟨array.mk 3 [W64 0x6873614869624167, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gAbiHash", W64 1, W64 0x2, true⟩,
      ⟨array.mk 3 [W64 0x6d6172676f725067, W64 0x0, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0x0, W64 0x0],
       go!"gProgram", W64 3, W64 0x8, true⟩,
      ⟨array.mk 3 [W64 0x656c69706d6f4368, W64 0x64, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xff, W64 0x0],
       go!"hCompiled", W64 6, W64 0x40, true⟩,
      ⟨array.mk 3 [W64 0x6f507972746e456b, W64 0x73746e69, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffff, W64 0x0],
       go!"kEntryPoints", W64 2, W64 0x4, true⟩,
      ⟨array.mk 3 [W64 0x6d6172676f72506b, W64 0x68736148, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffff, W64 0x0],
       go!"kProgramHash", W64 4, W64 0x10, true⟩,
      ⟨array.mk 3 [W64 0x69746e616d65536f, W64 0x6e6f697372655663, W64 0x0],
       array.mk 3 [W64 0xffffffffffffffff, W64 0xffffffffffffffff, W64 0x0],
       go!"oSemanticVersion", W64 5, W64 0x20, true⟩]

def classMask (schema : ClassSchema) : w64 := (W64 1 <<< W64 (classNames schema).length) - W64 1

def classGlobal : ClassSchema → GoString
  | .deprecatedEntryPoint => internal.cborclasses.cborDeprecatedEntryPointFields
  | .deprecatedCairoClass => internal.cborclasses.cborDeprecatedCairoClassFields
  | .sierraEntryPoint => internal.cborclasses.cborSierraEntryPointFields
  | .sierraEntryPointsByType => internal.cborclasses.cborSierraEntryPointsByTypeFields
  | .casmEntryPoint => internal.cborclasses.cborCasmEntryPointFields
  | .segmentLengths => internal.cborclasses.cborSegmentLengthsFields
  | .casmClass => internal.cborclasses.cborCasmClassFields
  | .sierraClass => internal.cborclasses.cborSierraClassFields
/-- Published table ownership includes the actual global and both immutable
    backing slices; calls may borrow it without duplicating mutable storage. -/
def ClassTable (schema : ClassSchema) : IProp GF := iprop(∃ fields : Loc,
  globalAddr (classGlobal schema) ↦□ fields ∗
  StaticFieldsShared fields (classKeys schema) (classNames schema) (classMask schema))

instance classTable_persistent (schema : ClassSchema) : Persistent (ClassTable (GF := GF) schema) := by
  unfold ClassTable
  infer_instance

def ClassMetadata : IProp GF := iprop(
  ClassTable .deprecatedEntryPoint ∗
  ClassTable .deprecatedCairoClass ∗
  ClassTable .sierraEntryPoint ∗
  ClassTable .sierraEntryPointsByType ∗
  ClassTable .casmEntryPoint ∗
  ClassTable .segmentLengths ∗
  ClassTable .casmClass ∗
  ClassTable .sierraClass)

instance classMetadata_persistent : Persistent (ClassMetadata (GF := GF)) := by
  unfold ClassMetadata
  infer_instance

instance classIsPkgInit : IsPkgInit (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses :=
  define_is_pkg_init (ClassMetadata (GF := GF))
instance classGetIsPkgInitWf : GetIsPkgInitWf (IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses :=
  build_get_is_pkg_init_wf

/-- Cached words/masks, encoded keys, indices and selection bits agree with
    explicitly listed metadata names for every class-package table. -/
theorem classKeys_valid (schema : ClassSchema) :
    (classKeys schema).all (fieldEntryValid (classNames schema)) = true := by
  cases schema <;> rfl

theorem classNames_small (schema : ClassSchema) : (classNames schema).length ≤ 64 := by
  cases schema <;> decide

theorem classNames_distinct (schema : ClassSchema) :
    ((classNames schema).map (List.map fieldFoldByte)).Nodup := by
  cases schema <;> decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classKeys_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.classNames_small
#print axioms Perennial.github_com.NethermindEth.juno.core.classNames_distinct
