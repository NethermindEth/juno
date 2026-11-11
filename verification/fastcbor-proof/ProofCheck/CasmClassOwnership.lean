import ProofCheck.CasmClassValues
import ProofCheck.SegmentArrayComplete
import ProofCheck.CasmEntryValuesComplete
import ProofCheck.FeltValuesTypedRefine
import ProofCheck.RawMessageComplete
import ProofCheck.BigIntPointerComplete
import ProofCheck.StringComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Separate field cells permit ownership of the embedded recursive segment
    record without overlapping whole-record ownership with child ownership. -/
def CasmClassOwned (dst : Loc) (value : CasmClassValue) : IProp GF := iprop(
  ∃ (bytecode : GoSlice) (pythonicHints : GoSlice) (hints : GoSlice) (prime : Loc) (external : GoSlice) (l1Handler : GoSlice) (constructor : GoSlice),
    ⌜dst ≠ null⌝ ∗
    structFieldRef CasmClass go!"Bytecode" dst ↦ bytecode ∗
    FeltValuesResult bytecode value.bytecode ∗
    structFieldRef CasmClass go!"PythonicHints" dst ↦ pythonicHints ∗
    RawMessageOwned pythonicHints value.pythonicHints ∗
    structFieldRef CasmClass go!"CompilerVersion" dst ↦ value.compilerVersion ∗
    structFieldRef CasmClass go!"Hints" dst ↦ hints ∗
    RawMessageOwned hints value.hints ∗
    structFieldRef CasmClass go!"Prime" dst ↦ prime ∗
    BigIntPointerOwned prime value.prime ∗
    structFieldRef CasmClass go!"External" dst ↦ external ∗
    CasmEntryValuesResult external value.external ∗
    structFieldRef CasmClass go!"L1Handler" dst ↦ l1Handler ∗
    CasmEntryValuesResult l1Handler value.l1Handler ∗
    structFieldRef CasmClass go!"Constructor" dst ↦ constructor ∗
    CasmEntryValuesResult constructor value.constructor ∗
    SegmentOwned (structFieldRef CasmClass go!"BytecodeSegmentLengths" dst) value.segments)

theorem casmClass_bytecode_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(∃ bytecode : GoSlice,
      structFieldRef CasmClass go!"Bytecode" dst ↦ bytecode ∗ FeltValuesResult bytecode old.bytecode ∗
      (∀ (out : GoSlice) (value : Option (List (GoArray w64 4))),
        structFieldRef CasmClass go!"Bytecode" dst ↦ out ∗ FeltValuesResult out value -∗
        CasmClassOwned dst {old with bytecode := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iexists bytecode
  iframe HbytecodeCell Hbytecode
  iintro %out %value ⟨HbytecodeCell, Hbytecode⟩
  iexists out, pythonicHints, hints, prime, external, l1Handler, constructor
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClass_pythonicHints_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(∃ pythonicHints : GoSlice,
      structFieldRef CasmClass go!"PythonicHints" dst ↦ pythonicHints ∗ RawMessageOwned pythonicHints old.pythonicHints ∗
      (∀ (out : GoSlice) (value : Option (List w8)),
        structFieldRef CasmClass go!"PythonicHints" dst ↦ out ∗ RawMessageOwned out value -∗
        CasmClassOwned dst {old with pythonicHints := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iexists pythonicHints
  iframe HpythonicHintsCell HpythonicHints
  iintro %out %value ⟨HpythonicHintsCell, HpythonicHints⟩
  iexists bytecode, out, hints, prime, external, l1Handler, constructor
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClass_compilerVersion_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(
      structFieldRef CasmClass go!"CompilerVersion" dst ↦ old.compilerVersion ∗
      (∀ value : GoString, structFieldRef CasmClass go!"CompilerVersion" dst ↦ value -∗
        CasmClassOwned dst {old with compilerVersion := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iframe HcompilerVersionCell
  iintro %value HcompilerVersionCell
  iexists bytecode, pythonicHints, hints, prime, external, l1Handler, constructor
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClass_hints_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(∃ hints : GoSlice,
      structFieldRef CasmClass go!"Hints" dst ↦ hints ∗ RawMessageOwned hints old.hints ∗
      (∀ (out : GoSlice) (value : Option (List w8)),
        structFieldRef CasmClass go!"Hints" dst ↦ out ∗ RawMessageOwned out value -∗
        CasmClassOwned dst {old with hints := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iexists hints
  iframe HhintsCell Hhints
  iintro %out %value ⟨HhintsCell, Hhints⟩
  iexists bytecode, pythonicHints, out, prime, external, l1Handler, constructor
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClass_prime_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(∃ prime : Loc,
      structFieldRef CasmClass go!"Prime" dst ↦ prime ∗ BigIntPointerOwned prime old.prime ∗
      (∀ (out : Loc) (value : Option Int),
        structFieldRef CasmClass go!"Prime" dst ↦ out ∗ BigIntPointerOwned out value -∗
        CasmClassOwned dst {old with prime := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iexists prime
  iframe HprimeCell Hprime
  iintro %out %value ⟨HprimeCell, Hprime⟩
  iexists bytecode, pythonicHints, hints, out, external, l1Handler, constructor
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClass_external_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(∃ external : GoSlice,
      structFieldRef CasmClass go!"External" dst ↦ external ∗ CasmEntryValuesResult external old.external ∗
      (∀ (out : GoSlice) (value : Option (List CasmEntryPointValue)),
        structFieldRef CasmClass go!"External" dst ↦ out ∗ CasmEntryValuesResult out value -∗
        CasmClassOwned dst {old with external := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iexists external
  iframe HexternalCell Hexternal
  iintro %out %value ⟨HexternalCell, Hexternal⟩
  iexists bytecode, pythonicHints, hints, prime, out, l1Handler, constructor
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClass_l1Handler_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(∃ l1Handler : GoSlice,
      structFieldRef CasmClass go!"L1Handler" dst ↦ l1Handler ∗ CasmEntryValuesResult l1Handler old.l1Handler ∗
      (∀ (out : GoSlice) (value : Option (List CasmEntryPointValue)),
        structFieldRef CasmClass go!"L1Handler" dst ↦ out ∗ CasmEntryValuesResult out value -∗
        CasmClassOwned dst {old with l1Handler := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iexists l1Handler
  iframe Hl1HandlerCell Hl1Handler
  iintro %out %value ⟨Hl1HandlerCell, Hl1Handler⟩
  iexists bytecode, pythonicHints, hints, prime, external, out, constructor
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClass_constructor_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(∃ constructor : GoSlice,
      structFieldRef CasmClass go!"Constructor" dst ↦ constructor ∗ CasmEntryValuesResult constructor old.constructor ∗
      (∀ (out : GoSlice) (value : Option (List CasmEntryPointValue)),
        structFieldRef CasmClass go!"Constructor" dst ↦ out ∗ CasmEntryValuesResult out value -∗
        CasmClassOwned dst {old with constructor := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iexists constructor
  iframe HconstructorCell Hconstructor
  iintro %out %value ⟨HconstructorCell, Hconstructor⟩
  iexists bytecode, pythonicHints, hints, prime, external, l1Handler, out
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClass_segments_acc (dst : Loc) (old : CasmClassValue) :
    CasmClassOwned dst old ⊢ (iprop(
      SegmentOwned (structFieldRef CasmClass go!"BytecodeSegmentLengths" dst) old.segments ∗
      (∀ value : SegmentValue, SegmentOwned (structFieldRef CasmClass go!"BytecodeSegmentLengths" dst) value -∗
        CasmClassOwned dst {old with segments := value})) : IProp GF) := by
  unfold CasmClassOwned
  iintro ⟨%bytecode, %pythonicHints, %hints, %prime, %external, %l1Handler, %constructor, %Hnn, HbytecodeCell, Hbytecode, HpythonicHintsCell, HpythonicHints, HcompilerVersionCell, HhintsCell, Hhints, HprimeCell, Hprime, HexternalCell, Hexternal, Hl1HandlerCell, Hl1Handler, HconstructorCell, Hconstructor, Hsegments⟩
  iframe Hsegments
  iintro %value Hsegments
  iexists bytecode, pythonicHints, hints, prime, external, l1Handler, constructor
  iframe HbytecodeCell Hbytecode HpythonicHintsCell HpythonicHints HcompilerVersionCell HhintsCell Hhints HprimeCell Hprime HexternalCell Hexternal Hl1HandlerCell Hl1Handler HconstructorCell Hconstructor Hsegments
  ipureintro; exact Hnn

theorem casmClassOwned_zero (dst : Loc) :
    dst ↦ zero_val CasmClass ⊢ (CasmClassOwned dst casmClassZero : IProp GF) := by
  iintro Hdst
  icases typedPointsto_not_null_dup dst (zero_val CasmClass) (DFrac.own 1) $$ Hdst with ⟨Hdst, %Hnn⟩
  icases typedPointsto_split dst (zero_val CasmClass) (DFrac.own 1) $$ Hdst with Hdst
  isimp only [TypedPointsto.typedPointstoDef] at Hdst
  icases Hdst with ⟨HbytecodeCell, HpythonicHintsCell, HcompilerVersionCell, HhintsCell, HprimeCell,
    HexternalCell, Hl1HandlerCell, HconstructorCell, HsegmentsCell, -⟩
  ihave Hsegments := segmentOwned_zero (structFieldRef CasmClass go!"BytecodeSegmentLengths" dst) $$ HsegmentsCell
  unfold CasmClassOwned
  iexists slice.nil, slice.nil, slice.nil, null, slice.nil, slice.nil, slice.nil
  isimp only [casmClassZero, FeltValuesResult, RawMessageOwned, BigIntPointerOwned, CasmEntryValuesResult]
  iframe HbytecodeCell HpythonicHintsCell HcompilerVersionCell HhintsCell HprimeCell HexternalCell Hl1HandlerCell HconstructorCell Hsegments
  ipureintro; exact Hnn

theorem casmClassOwned_nonnull (dst : Loc) (value : CasmClassValue) :
    (CasmClassOwned dst value : IProp GF) ⊢ ⌜dst ≠ null⌝ := by
  unfold CasmClassOwned
  iintro ⟨%_, %_, %_, %_, %_, %_, %_, %Hnn, _⟩
  ipureintro; exact Hnn

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_bytecode_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_pythonicHints_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_compilerVersion_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_hints_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_prime_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_external_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_l1Handler_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_constructor_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClass_segments_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassOwned_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassOwned_nonnull
