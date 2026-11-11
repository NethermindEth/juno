import ProofCheck.SegmentValues
import ProofCheck.ClassInitialize

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Recursive ownership of the concrete embedded records and their separately
    allocated descendants. The finite value tree supplies the recursion measure;
    it does not impose the decoder's nesting limit on previous caller values. -/
def SegmentOwned (dst : Loc) (value : SegmentValue) : IProp GF :=
  match value with
  | .mk none length => iprop(dst ↦ SegmentLengths.mk slice.nil length)
  | .mk (some children) length => iprop(∃ out : GoSlice,
      dst ↦ SegmentLengths.mk out length ∗
      ⌜out.ptr ≠ null ∧ sint.Z out.len = children.length⌝ ∗
      [∗list] i ↦ child ∈ children.attach,
        SegmentOwned (sliceIndexRef SegmentLengths (i : Int) out) child.val)
termination_by sizeOf value
decreasing_by
  have bound := List.sizeOf_lt_of_mem child.property
  simp at *
  omega

/-- A flat inventory of embedded records uses the recursive ownership above. -/
def SegmentSlots (data : GoSlice) (values : List SegmentValue) : IProp GF :=
  iprop([∗list] i ↦ value ∈ values, SegmentOwned (sliceIndexRef SegmentLengths (i : Int) data) value)

def SegmentArrayOwned (out : GoSlice) (values : List SegmentValue) : IProp GF := iprop(
  ⌜out.ptr ≠ null ∧ sint.Z out.len = values.length⌝ ∗ SegmentSlots out values)

def SegmentArrayResult (out : GoSlice) : Option (List SegmentValue) → IProp GF
  | none => iprop(⌜out = slice.nil⌝)
  | some values => SegmentArrayOwned out values

/-- Eliminate the proof-carrying list used solely to justify definition recursion. -/
theorem segmentSlots_attach (data : GoSlice) (values : List SegmentValue) :
    (iprop([∗list] i ↦ value ∈ values.attach,
      SegmentOwned (sliceIndexRef SegmentLengths (i : Int) data) value.val) : IProp GF) =
      SegmentSlots data values := by
  unfold SegmentSlots
  rw [← BigSepL.bigSepL_map Subtype.val]
  simp

/-- Uniform record ownership exposes exactly its slice header, scalar and
    recursive children, including nil versus nonnil empty children. -/
theorem segmentOwned_unfold (dst : Loc) (value : SegmentValue) :
    SegmentOwned dst value ⊣⊢ (iprop(∃ children : GoSlice,
      dst ↦ SegmentLengths.mk children value.length ∗ SegmentArrayResult children value.children) : IProp GF) := by
  cases value with
  | mk children length =>
    cases children with
    | none =>
      rw [SegmentOwned]
      simp only [SegmentValue.length, SegmentValue.children, SegmentArrayResult]
      constructor
      · iintro Hdst
        iexists slice.nil
        iframe Hdst
        ipureintro; rfl
      · iintro ⟨%out, Hdst, %eq⟩
        subst out
        iexact Hdst
    | some values =>
      rw [SegmentOwned]
      simp only [SegmentValue.length, SegmentValue.children, SegmentArrayResult, SegmentArrayOwned,
        segmentSlots_attach]
      exact BiEntails.of_eq rfl

/-- Borrow the Children header and descendants while retaining Length. -/
theorem segment_children_acc (dst : Loc) (old : SegmentValue) :
    SegmentOwned dst old ⊢ (iprop(∃ children : GoSlice,
      structFieldRef SegmentLengths go!"Children" dst ↦ children ∗ SegmentArrayResult children old.children ∗
      (∀ (out : GoSlice) (value : Option (List SegmentValue)),
        structFieldRef SegmentLengths go!"Children" dst ↦ out ∗ SegmentArrayResult out value -∗
        SegmentOwned dst (segmentSetChildren old value))) : IProp GF) := by
  iintro Howned
  icases (segmentOwned_unfold dst old).1 $$ Howned with ⟨%children, Hh, Hchildren⟩
  icases typedPointsto_not_null_dup dst (SegmentLengths.mk children old.length) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (SegmentLengths.mk children old.length) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HchildrenCell, HlengthCell, -⟩
  iexists children
  iframe HchildrenCell Hchildren
  iintro %out %value ⟨HchildrenCell, Hchildren⟩
  iapply (segmentOwned_unfold dst (segmentSetChildren old value)).2
  isimp only [segmentSetChildren_preserves_length,
    show (segmentSetChildren old value).children = value from rfl]
  iexists out
  iframe Hchildren
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HchildrenCell HlengthCell

/-- Borrow Length while retaining the complete recursive Children tree. -/
theorem segment_length_acc (dst : Loc) (old : SegmentValue) :
    SegmentOwned dst old ⊢ (iprop(
      structFieldRef SegmentLengths go!"Length" dst ↦ old.length ∗
      (∀ value : w64, structFieldRef SegmentLengths go!"Length" dst ↦ value -∗
        SegmentOwned dst (segmentSetLength old value))) : IProp GF) := by
  iintro Howned
  icases (segmentOwned_unfold dst old).1 $$ Howned with ⟨%children, Hh, Hchildren⟩
  icases typedPointsto_not_null_dup dst (SegmentLengths.mk children old.length) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (SegmentLengths.mk children old.length) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HchildrenCell, HlengthCell, -⟩
  iframe HlengthCell
  iintro %value HlengthCell
  iapply (segmentOwned_unfold dst (segmentSetLength old value)).2
  isimp only [segmentSetLength_preserves_children,
    show (segmentSetLength old value).length = value from rfl]
  iexists children
  iframe Hchildren
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HchildrenCell HlengthCell

theorem segmentOwned_zero (dst : Loc) :
    dst ↦ zero_val SegmentLengths ⊢ (SegmentOwned dst segmentZero : IProp GF) := by
  iintro Hdst
  iapply (segmentOwned_unfold dst segmentZero).2
  iexists slice.nil
  have hz : zero_val SegmentLengths = SegmentLengths.mk slice.nil (W64 0) := rfl
  isimp only [hz] at Hdst
  isimp only [segmentZero, SegmentValue.children, SegmentValue.length, SegmentArrayResult]
  iframe Hdst

theorem segmentOwned_nonnull (dst : Loc) (value : SegmentValue) :
    (SegmentOwned dst value : IProp GF) ⊢ ⌜dst ≠ null⌝ := by
  iintro Howned
  icases (segmentOwned_unfold dst value).1 $$ Howned with ⟨%children, Hh, Hchildren⟩
  ihave %Hnonnull := typedPointsto_not_null _ _ _ $$ Hh
  ipureintro; exact Hnonnull

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentSlots_attach
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentOwned_unfold
#print axioms Perennial.github_com.NethermindEth.juno.core.segment_children_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.segment_length_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentOwned_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentOwned_nonnull
