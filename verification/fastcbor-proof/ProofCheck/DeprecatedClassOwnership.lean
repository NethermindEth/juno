import ProofCheck.DeprecatedClassValues
import ProofCheck.DeprecatedEntryValuesComplete
import ProofCheck.RawMessageComplete
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

def DeprecatedClassOwned (dst : Loc) (value : DeprecatedClassValue) : IProp GF := iprop(
  ∃ (abi externals l1Handlers constructors : GoSlice),
    dst ↦ DeprecatedCairoClass.mk abi externals l1Handlers constructors value.program ∗
    RawMessageOwned abi value.abi ∗
    DeprecatedEntryValuesResult externals value.externals ∗
    DeprecatedEntryValuesResult l1Handlers value.l1Handlers ∗
    DeprecatedEntryValuesResult constructors value.constructors)

/-- Borrow Abi and its children while retaining all other class fields. -/
theorem deprecatedClass_abi_acc (dst : Loc) (old : DeprecatedClassValue) :
    DeprecatedClassOwned dst old ⊢ (iprop(∃ abi : GoSlice,
      structFieldRef DeprecatedCairoClass go!"Abi" dst ↦ abi ∗ RawMessageOwned abi old.abi ∗
      (∀ (out : GoSlice) (value : Option (List w8)),
        structFieldRef DeprecatedCairoClass go!"Abi" dst ↦ out ∗ RawMessageOwned out value -∗
        DeprecatedClassOwned dst {old with abi := value})) : IProp GF) := by
  unfold DeprecatedClassOwned
  iintro ⟨%abi, %externals, %l1Handlers, %constructors, Hh, Habi, Hexternals, Hl1Handlers, Hconstructors⟩
  icases typedPointsto_not_null_dup dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HabiCell, HexternalsCell, Hl1HandlersCell, HconstructorsCell, HprogramCell, -⟩
  iexists abi
  iframe HabiCell Habi
  iintro %out %value ⟨HabiCell, Habi⟩
  iexists out, externals, l1Handlers, constructors
  iframe Habi Hexternals Hl1Handlers Hconstructors
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HabiCell HexternalsCell Hl1HandlersCell HconstructorsCell HprogramCell

/-- Borrow Externals and its children while retaining all other class fields. -/
theorem deprecatedClass_externals_acc (dst : Loc) (old : DeprecatedClassValue) :
    DeprecatedClassOwned dst old ⊢ (iprop(∃ externals : GoSlice,
      structFieldRef DeprecatedCairoClass go!"Externals" dst ↦ externals ∗ DeprecatedEntryValuesResult externals old.externals ∗
      (∀ (out : GoSlice) (value : Option (List DeprecatedEntryPointValue)),
        structFieldRef DeprecatedCairoClass go!"Externals" dst ↦ out ∗ DeprecatedEntryValuesResult out value -∗
        DeprecatedClassOwned dst {old with externals := value})) : IProp GF) := by
  unfold DeprecatedClassOwned
  iintro ⟨%abi, %externals, %l1Handlers, %constructors, Hh, Habi, Hexternals, Hl1Handlers, Hconstructors⟩
  icases typedPointsto_not_null_dup dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HabiCell, HexternalsCell, Hl1HandlersCell, HconstructorsCell, HprogramCell, -⟩
  iexists externals
  iframe HexternalsCell Hexternals
  iintro %out %value ⟨HexternalsCell, Hexternals⟩
  iexists abi, out, l1Handlers, constructors
  iframe Habi Hexternals Hl1Handlers Hconstructors
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HabiCell HexternalsCell Hl1HandlersCell HconstructorsCell HprogramCell

/-- Borrow L1Handlers and its children while retaining all other class fields. -/
theorem deprecatedClass_l1Handlers_acc (dst : Loc) (old : DeprecatedClassValue) :
    DeprecatedClassOwned dst old ⊢ (iprop(∃ l1Handlers : GoSlice,
      structFieldRef DeprecatedCairoClass go!"L1Handlers" dst ↦ l1Handlers ∗ DeprecatedEntryValuesResult l1Handlers old.l1Handlers ∗
      (∀ (out : GoSlice) (value : Option (List DeprecatedEntryPointValue)),
        structFieldRef DeprecatedCairoClass go!"L1Handlers" dst ↦ out ∗ DeprecatedEntryValuesResult out value -∗
        DeprecatedClassOwned dst {old with l1Handlers := value})) : IProp GF) := by
  unfold DeprecatedClassOwned
  iintro ⟨%abi, %externals, %l1Handlers, %constructors, Hh, Habi, Hexternals, Hl1Handlers, Hconstructors⟩
  icases typedPointsto_not_null_dup dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HabiCell, HexternalsCell, Hl1HandlersCell, HconstructorsCell, HprogramCell, -⟩
  iexists l1Handlers
  iframe Hl1HandlersCell Hl1Handlers
  iintro %out %value ⟨Hl1HandlersCell, Hl1Handlers⟩
  iexists abi, externals, out, constructors
  iframe Habi Hexternals Hl1Handlers Hconstructors
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HabiCell HexternalsCell Hl1HandlersCell HconstructorsCell HprogramCell

/-- Borrow Constructors and its children while retaining all other class fields. -/
theorem deprecatedClass_constructors_acc (dst : Loc) (old : DeprecatedClassValue) :
    DeprecatedClassOwned dst old ⊢ (iprop(∃ constructors : GoSlice,
      structFieldRef DeprecatedCairoClass go!"Constructors" dst ↦ constructors ∗ DeprecatedEntryValuesResult constructors old.constructors ∗
      (∀ (out : GoSlice) (value : Option (List DeprecatedEntryPointValue)),
        structFieldRef DeprecatedCairoClass go!"Constructors" dst ↦ out ∗ DeprecatedEntryValuesResult out value -∗
        DeprecatedClassOwned dst {old with constructors := value})) : IProp GF) := by
  unfold DeprecatedClassOwned
  iintro ⟨%abi, %externals, %l1Handlers, %constructors, Hh, Habi, Hexternals, Hl1Handlers, Hconstructors⟩
  icases typedPointsto_not_null_dup dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HabiCell, HexternalsCell, Hl1HandlersCell, HconstructorsCell, HprogramCell, -⟩
  iexists constructors
  iframe HconstructorsCell Hconstructors
  iintro %out %value ⟨HconstructorsCell, Hconstructors⟩
  iexists abi, externals, l1Handlers, out
  iframe Habi Hexternals Hl1Handlers Hconstructors
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HabiCell HexternalsCell Hl1HandlersCell HconstructorsCell HprogramCell

/-- Borrow the program string while retaining all four slices and their children. -/
theorem deprecatedClass_program_acc (dst : Loc) (old : DeprecatedClassValue) :
    DeprecatedClassOwned dst old ⊢ (iprop(
      structFieldRef DeprecatedCairoClass go!"Program" dst ↦ old.program ∗
      (∀ value : GoString, structFieldRef DeprecatedCairoClass go!"Program" dst ↦ value -∗
        DeprecatedClassOwned dst {old with program := value})) : IProp GF) := by
  unfold DeprecatedClassOwned
  iintro ⟨%abi, %externals, %l1Handlers, %constructors, Hh, Habi, Hexternals, Hl1Handlers, Hconstructors⟩
  icases typedPointsto_not_null_dup dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (DeprecatedCairoClass.mk abi externals l1Handlers constructors old.program) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HabiCell, HexternalsCell, Hl1HandlersCell, HconstructorsCell, HprogramCell, -⟩
  iframe HprogramCell
  iintro %value HprogramCell
  iexists abi, externals, l1Handlers, constructors
  iframe Habi Hexternals Hl1Handlers Hconstructors
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HabiCell HexternalsCell Hl1HandlersCell HconstructorsCell HprogramCell

theorem deprecatedClassOwned_zero (dst : Loc) :
    dst ↦ zero_val DeprecatedCairoClass ⊢ (DeprecatedClassOwned dst deprecatedClassZero : IProp GF) := by
  iintro Hdst
  unfold DeprecatedClassOwned
  iexists slice.nil, slice.nil, slice.nil, slice.nil
  have hz : zero_val DeprecatedCairoClass = DeprecatedCairoClass.mk slice.nil slice.nil slice.nil slice.nil (go!"") := rfl
  isimp only [hz] at Hdst
  isimp only [deprecatedClassZero, RawMessageOwned, DeprecatedEntryValuesResult]
  iframe Hdst
  ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClass_abi_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClass_externals_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClass_l1Handlers_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClass_constructors_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClass_program_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassOwned_zero
