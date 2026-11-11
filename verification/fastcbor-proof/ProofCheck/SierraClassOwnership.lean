import ProofCheck.SierraClassValues
import ProofCheck.CasmClassPointerComplete
import ProofCheck.SierraEntriesRecordComplete
import ProofCheck.FeltValuesTypedRefine
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

/-- Separate class field cells compose with ownership of the embedded entry
    groups and the independently allocated compiled class. -/
def SierraClassOwned (dst : Loc) (value : SierraClassValue) : IProp GF := iprop(
  ∃ (abiHash : Loc) (program : GoSlice) (programHash : Loc) (compiled : Loc),
    ⌜dst ≠ null⌝ ∗
    structFieldRef SierraClass go!"Abi" dst ↦ value.abi ∗
    structFieldRef SierraClass go!"AbiHash" dst ↦ abiHash ∗
    FeltResult abiHash value.abiHash ∗
    SierraEntriesOwned (structFieldRef SierraClass go!"EntryPoints" dst) value.entries ∗
    structFieldRef SierraClass go!"Program" dst ↦ program ∗
    FeltValuesResult program value.program ∗
    structFieldRef SierraClass go!"ProgramHash" dst ↦ programHash ∗
    FeltResult programHash value.programHash ∗
    structFieldRef SierraClass go!"SemanticVersion" dst ↦ value.version ∗
    structFieldRef SierraClass go!"Compiled" dst ↦ compiled ∗
    CasmClassPointerOwned compiled value.compiled)

theorem sierraClass_abi_acc (dst : Loc) (old : SierraClassValue) :
    SierraClassOwned dst old ⊢ (iprop(
      structFieldRef SierraClass go!"Abi" dst ↦ old.abi ∗
      (∀ value : GoString, structFieldRef SierraClass go!"Abi" dst ↦ value -∗
        SierraClassOwned dst {old with abi := value})) : IProp GF) := by
  unfold SierraClassOwned
  iintro ⟨%abiHash, %program, %programHash, %compiled, %Hnn, HabiCell, HabiHashCell, HabiHash, Hentries, HprogramCell, Hprogram, HprogramHashCell, HprogramHash, HversionCell, HcompiledCell, Hcompiled⟩
  iframe HabiCell
  iintro %value HabiCell
  iexists abiHash, program, programHash, compiled
  iframe HabiCell HabiHashCell HabiHash Hentries HprogramCell Hprogram HprogramHashCell HprogramHash HversionCell HcompiledCell Hcompiled
  ipureintro; exact Hnn

theorem sierraClass_abiHash_acc (dst : Loc) (old : SierraClassValue) :
    SierraClassOwned dst old ⊢ (iprop(∃ abiHash : Loc,
      structFieldRef SierraClass go!"AbiHash" dst ↦ abiHash ∗ FeltResult abiHash old.abiHash ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef SierraClass go!"AbiHash" dst ↦ out ∗ FeltResult out value -∗
        SierraClassOwned dst {old with abiHash := value})) : IProp GF) := by
  unfold SierraClassOwned
  iintro ⟨%abiHash, %program, %programHash, %compiled, %Hnn, HabiCell, HabiHashCell, HabiHash, Hentries, HprogramCell, Hprogram, HprogramHashCell, HprogramHash, HversionCell, HcompiledCell, Hcompiled⟩
  iexists abiHash
  iframe HabiHashCell HabiHash
  iintro %out %value ⟨HabiHashCell, HabiHash⟩
  iexists out, program, programHash, compiled
  iframe HabiCell HabiHashCell HabiHash Hentries HprogramCell Hprogram HprogramHashCell HprogramHash HversionCell HcompiledCell Hcompiled
  ipureintro; exact Hnn

theorem sierraClass_entries_acc (dst : Loc) (old : SierraClassValue) :
    SierraClassOwned dst old ⊢ (iprop(
      SierraEntriesOwned (structFieldRef SierraClass go!"EntryPoints" dst) old.entries ∗
      (∀ value : SierraEntriesValue, SierraEntriesOwned (structFieldRef SierraClass go!"EntryPoints" dst) value -∗
        SierraClassOwned dst {old with entries := value})) : IProp GF) := by
  unfold SierraClassOwned
  iintro ⟨%abiHash, %program, %programHash, %compiled, %Hnn, HabiCell, HabiHashCell, HabiHash, Hentries, HprogramCell, Hprogram, HprogramHashCell, HprogramHash, HversionCell, HcompiledCell, Hcompiled⟩
  iframe Hentries
  iintro %value Hentries
  iexists abiHash, program, programHash, compiled
  iframe HabiCell HabiHashCell HabiHash Hentries HprogramCell Hprogram HprogramHashCell HprogramHash HversionCell HcompiledCell Hcompiled
  ipureintro; exact Hnn

theorem sierraClass_program_acc (dst : Loc) (old : SierraClassValue) :
    SierraClassOwned dst old ⊢ (iprop(∃ program : GoSlice,
      structFieldRef SierraClass go!"Program" dst ↦ program ∗ FeltValuesResult program old.program ∗
      (∀ (out : GoSlice) (value : Option (List (GoArray w64 4))),
        structFieldRef SierraClass go!"Program" dst ↦ out ∗ FeltValuesResult out value -∗
        SierraClassOwned dst {old with program := value})) : IProp GF) := by
  unfold SierraClassOwned
  iintro ⟨%abiHash, %program, %programHash, %compiled, %Hnn, HabiCell, HabiHashCell, HabiHash, Hentries, HprogramCell, Hprogram, HprogramHashCell, HprogramHash, HversionCell, HcompiledCell, Hcompiled⟩
  iexists program
  iframe HprogramCell Hprogram
  iintro %out %value ⟨HprogramCell, Hprogram⟩
  iexists abiHash, out, programHash, compiled
  iframe HabiCell HabiHashCell HabiHash Hentries HprogramCell Hprogram HprogramHashCell HprogramHash HversionCell HcompiledCell Hcompiled
  ipureintro; exact Hnn

theorem sierraClass_programHash_acc (dst : Loc) (old : SierraClassValue) :
    SierraClassOwned dst old ⊢ (iprop(∃ programHash : Loc,
      structFieldRef SierraClass go!"ProgramHash" dst ↦ programHash ∗ FeltResult programHash old.programHash ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef SierraClass go!"ProgramHash" dst ↦ out ∗ FeltResult out value -∗
        SierraClassOwned dst {old with programHash := value})) : IProp GF) := by
  unfold SierraClassOwned
  iintro ⟨%abiHash, %program, %programHash, %compiled, %Hnn, HabiCell, HabiHashCell, HabiHash, Hentries, HprogramCell, Hprogram, HprogramHashCell, HprogramHash, HversionCell, HcompiledCell, Hcompiled⟩
  iexists programHash
  iframe HprogramHashCell HprogramHash
  iintro %out %value ⟨HprogramHashCell, HprogramHash⟩
  iexists abiHash, program, out, compiled
  iframe HabiCell HabiHashCell HabiHash Hentries HprogramCell Hprogram HprogramHashCell HprogramHash HversionCell HcompiledCell Hcompiled
  ipureintro; exact Hnn

theorem sierraClass_version_acc (dst : Loc) (old : SierraClassValue) :
    SierraClassOwned dst old ⊢ (iprop(
      structFieldRef SierraClass go!"SemanticVersion" dst ↦ old.version ∗
      (∀ value : GoString, structFieldRef SierraClass go!"SemanticVersion" dst ↦ value -∗
        SierraClassOwned dst {old with version := value})) : IProp GF) := by
  unfold SierraClassOwned
  iintro ⟨%abiHash, %program, %programHash, %compiled, %Hnn, HabiCell, HabiHashCell, HabiHash, Hentries, HprogramCell, Hprogram, HprogramHashCell, HprogramHash, HversionCell, HcompiledCell, Hcompiled⟩
  iframe HversionCell
  iintro %value HversionCell
  iexists abiHash, program, programHash, compiled
  iframe HabiCell HabiHashCell HabiHash Hentries HprogramCell Hprogram HprogramHashCell HprogramHash HversionCell HcompiledCell Hcompiled
  ipureintro; exact Hnn

theorem sierraClass_compiled_acc (dst : Loc) (old : SierraClassValue) :
    SierraClassOwned dst old ⊢ (iprop(∃ compiled : Loc,
      structFieldRef SierraClass go!"Compiled" dst ↦ compiled ∗ CasmClassPointerOwned compiled old.compiled ∗
      (∀ (out : Loc) (value : Option CasmClassValue),
        structFieldRef SierraClass go!"Compiled" dst ↦ out ∗ CasmClassPointerOwned out value -∗
        SierraClassOwned dst {old with compiled := value})) : IProp GF) := by
  unfold SierraClassOwned
  iintro ⟨%abiHash, %program, %programHash, %compiled, %Hnn, HabiCell, HabiHashCell, HabiHash, Hentries, HprogramCell, Hprogram, HprogramHashCell, HprogramHash, HversionCell, HcompiledCell, Hcompiled⟩
  iexists compiled
  iframe HcompiledCell Hcompiled
  iintro %out %value ⟨HcompiledCell, Hcompiled⟩
  iexists abiHash, program, programHash, out
  iframe HabiCell HabiHashCell HabiHash Hentries HprogramCell Hprogram HprogramHashCell HprogramHash HversionCell HcompiledCell Hcompiled
  ipureintro; exact Hnn

theorem sierraClassOwned_zero (dst : Loc) :
    dst ↦ zero_val SierraClass ⊢ (SierraClassOwned dst sierraClassZero : IProp GF) := by
  iintro Hdst
  icases typedPointsto_not_null_dup dst (zero_val SierraClass) (DFrac.own 1) $$ Hdst with ⟨Hdst, %Hnn⟩
  icases typedPointsto_split dst (zero_val SierraClass) (DFrac.own 1) $$ Hdst with Hdst
  isimp only [TypedPointsto.typedPointstoDef] at Hdst
  icases Hdst with ⟨HabiCell, HabiHashCell, HentriesCell, HprogramCell, HprogramHashCell, HversionCell, HcompiledCell, -⟩
  ihave Hentries := sierraEntriesOwned_zero (structFieldRef SierraClass go!"EntryPoints" dst) $$ HentriesCell
  unfold SierraClassOwned
  iexists null, slice.nil, null, null
  isimp only [sierraClassZero, FeltResult, FeltValuesResult, CasmClassPointerOwned]
  iframe HabiCell HabiHashCell Hentries HprogramCell HprogramHashCell HversionCell HcompiledCell
  ipureintro; exact Hnn

theorem sierraClassOwned_nonnull (dst : Loc) (value : SierraClassValue) :
    (SierraClassOwned dst value : IProp GF) ⊢ ⌜dst ≠ null⌝ := by
  unfold SierraClassOwned
  iintro ⟨%_, %_, %_, %_, %Hnn, _⟩
  ipureintro; exact Hnn

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClass_abi_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClass_abiHash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClass_entries_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClass_program_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClass_programHash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClass_version_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClass_compiled_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassOwned_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassOwned_nonnull
