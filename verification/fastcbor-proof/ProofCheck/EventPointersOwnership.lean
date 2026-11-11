import ProofCheck.EventOwnership

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A nullable Event pointer owns exactly its record and children when present. -/
def EventPointerOwned (p : Loc) (value : Option EventValue) : IProp GF :=
  match value with
  | none => iprop(⌜p = null⌝)
  | some value => EventOwned p value

/-- Published entries own disjoint records, whether allocated individually or
    removed from the unused portion of shared storage. -/
def EventPointerChildren (ptrs : List Loc) (values : List (Option EventValue)) : IProp GF :=
  iprop([∗list] p;value ∈ ptrs;values, EventPointerOwned p value)

def EventPointersOwned (out : GoSlice) (values : List (Option EventValue)) : IProp GF :=
  iprop(∃ ptrs : List Loc, ⌜out.ptr ≠ null⌝ ∗ out ↦* ptrs ∗ EventPointerChildren ptrs values)

def EventPointersResult (out : GoSlice) (values : Option (List (Option EventValue))) : IProp GF :=
  match values with
  | none => iprop(⌜out = slice.nil⌝)
  | some values => EventPointersOwned out values

/-- Only unused slots remain here. Used slots move into EventPointerOwned,
    avoiding duplicate ownership of structs in the shared backing array. -/
def EventStorage (data : GoSlice) (used count : Nat) : IProp GF :=
  arrayElems (sliceIndexRef Event (used : Int) data)
    (List.replicate (count - used) (zero_val Event)) (DFrac.own 1)

theorem eventPointerChildren_nil : ⊢ (EventPointerChildren [] [] : IProp GF) := by
  unfold EventPointerChildren
  iapply BigSepL2.bigSepL2_nil.2
  iempintro

theorem eventPointerChildren_length (ptrs : List Loc) (values : List (Option EventValue)) :
    EventPointerChildren ptrs values ⊢ (⌜ptrs.length = values.length⌝ : IProp GF) := by
  exact BigSepL2.bigSepL2_length

theorem eventPointerChildren_snoc (ptrs : List Loc) (values : List (Option EventValue))
    (p : Loc) (value : Option EventValue) :
    EventPointerChildren ptrs values ∗ EventPointerOwned p value ⊢
      (EventPointerChildren (ptrs ++ [p]) (values ++ [value]) : IProp GF) := by
  unfold EventPointerChildren
  iintro ⟨Hchildren, Hvalue⟩
  iapply BigSepL2.bigSepL2_snoc.2
  iframe Hchildren Hvalue

/-- Allocated zero Events are nonnil records, distinct from nil elements. -/
theorem eventPointerOwned_zero (p : Loc) :
    p ↦ zero_val Event ⊢ (EventPointerOwned p (some eventZero) : IProp GF) := by
  iintro Hp
  unfold EventPointerOwned EventOwned
  iexists zero_val Event
  iframe Hp
  iapply eventChildren_zero

theorem eventPointerOwned_nonnull (p : Loc) (value : EventValue) :
    EventPointerOwned p (some value) ⊢ (⌜p ≠ null⌝ : IProp GF) := by
  iintro Howned
  unfold EventPointerOwned EventOwned
  icases Howned with ⟨%h, Hh, _⟩
  icases typedPointsto_not_null_dup p h (DFrac.own 1) $$ Hh with ⟨_, %Hnn⟩
  ipureintro; exact Hnn

/-- Convert the fresh backing array into an inventory of unused zero slots. -/
theorem eventStorage_init (data : GoSlice) (count : Nat) :
    data ↦* List.replicate count (zero_val Event) ⊢ (EventStorage data 0 count : IProp GF) := by
  rw [ownSlice_unseal]
  unfold ownSliceDef EventStorage
  simp only [Nat.sub_zero, Int.natCast_zero, sliceIndexRef, go.arrayIndexRef_0]
  iintro (%Hnil | ⟨Harray, _⟩)
  · have hc : count = 0 := by
      have h := congrArg List.length Hnil.2
      simpa using h
    subst count
    simp only [List.replicate_zero, arrayElems]
    iapply BigSepL.bigSepL_nil.2
    iempintro
  · icases typedPointsto_split _ _ _ $$ Harray with Harray
    isimp only [TypedPointsto.typedPointstoDef] at Harray
    icases Harray with ⟨_, Hslots⟩
    iexact Hslots

/-- Transfer exactly the next shared slot, retaining all later unused slots. -/
theorem eventStorage_take (data : GoSlice) (used count : Nat) (more : used < count) :
    EventStorage data used count ⊢ (iprop(
      EventPointerOwned (sliceIndexRef Event (used : Int) data) (some eventZero) ∗
      EventStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold EventStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef Event 1 (sliceIndexRef Event (used : Int) data) =
      sliceIndexRef Event ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iapply eventPointerOwned_zero $$ Hfirst

theorem eventPointersOwned_length (out : GoSlice) (values : List (Option EventValue)) :
    EventPointersOwned out values ⊢ (⌜sint.nat out.len = values.length ∧ out.ptr ≠ null⌝ : IProp GF) := by
  iintro Howned
  unfold EventPointersOwned
  icases Howned with ⟨%ptrs, %Hnn, Hout, Hchildren⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hout
  ihave %Hchildren := eventPointerChildren_length ptrs values $$ Hchildren
  ipureintro
  exact ⟨Hlen.1.symm.trans Hchildren, Hnn⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerChildren_nil
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerChildren_length
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerChildren_snoc
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerOwned_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointerOwned_nonnull
#print axioms Perennial.github_com.NethermindEth.juno.core.eventStorage_init
#print axioms Perennial.github_com.NethermindEth.juno.core.eventStorage_take
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPointersOwned_length
