import ProofCheck.L2MessageOwnership

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

/-- A nullable L2ToL1Message pointer owns exactly its record and children when present. -/
def L2MessagePointerOwned (p : Loc) (value : Option L2MessageValue) : IProp GF :=
  match value with
  | none => iprop(⌜p = null⌝)
  | some value => L2MessageOwned p value

/-- Published entries own disjoint records, whether allocated individually or
    removed from the unused portion of shared storage. -/
def L2MessagePointerChildren (ptrs : List Loc) (values : List (Option L2MessageValue)) : IProp GF :=
  iprop([∗list] p;value ∈ ptrs;values, L2MessagePointerOwned p value)

def L2MessagePointersOwned (out : GoSlice) (values : List (Option L2MessageValue)) : IProp GF :=
  iprop(∃ ptrs : List Loc, ⌜out.ptr ≠ null⌝ ∗ out ↦* ptrs ∗ L2MessagePointerChildren ptrs values)

def L2MessagePointersResult (out : GoSlice) (values : Option (List (Option L2MessageValue))) : IProp GF :=
  match values with
  | none => iprop(⌜out = slice.nil⌝)
  | some values => L2MessagePointersOwned out values

/-- Only unused slots remain here. Used slots move into L2MessagePointerOwned,
    avoiding duplicate ownership of structs in the shared backing array. -/
def L2MessageStorage (data : GoSlice) (used count : Nat) : IProp GF :=
  arrayElems (sliceIndexRef L2ToL1Message (used : Int) data)
    (List.replicate (count - used) (zero_val L2ToL1Message)) (DFrac.own 1)

theorem l2MessagePointerChildren_nil : ⊢ (L2MessagePointerChildren [] [] : IProp GF) := by
  unfold L2MessagePointerChildren
  iapply BigSepL2.bigSepL2_nil.2
  iempintro

theorem l2MessagePointerChildren_length (ptrs : List Loc) (values : List (Option L2MessageValue)) :
    L2MessagePointerChildren ptrs values ⊢ (⌜ptrs.length = values.length⌝ : IProp GF) := by
  exact BigSepL2.bigSepL2_length

theorem l2MessagePointerChildren_snoc (ptrs : List Loc) (values : List (Option L2MessageValue))
    (p : Loc) (value : Option L2MessageValue) :
    L2MessagePointerChildren ptrs values ∗ L2MessagePointerOwned p value ⊢
      (L2MessagePointerChildren (ptrs ++ [p]) (values ++ [value]) : IProp GF) := by
  unfold L2MessagePointerChildren
  iintro ⟨Hchildren, Hvalue⟩
  iapply BigSepL2.bigSepL2_snoc.2
  iframe Hchildren Hvalue

/-- Allocated zero L2Messages are nonnil records, distinct from nil elements. -/
theorem l2MessagePointerOwned_zero (p : Loc) :
    p ↦ zero_val L2ToL1Message ⊢ (L2MessagePointerOwned p (some l2MessageZero) : IProp GF) := by
  iintro Hp
  unfold L2MessagePointerOwned L2MessageOwned
  iexists zero_val L2ToL1Message
  iframe Hp
  iapply l2MessageChildren_zero

theorem l2MessagePointerOwned_nonnull (p : Loc) (value : L2MessageValue) :
    L2MessagePointerOwned p (some value) ⊢ (⌜p ≠ null⌝ : IProp GF) := by
  iintro Howned
  unfold L2MessagePointerOwned L2MessageOwned
  icases Howned with ⟨%h, Hh, _⟩
  icases typedPointsto_not_null_dup p h (DFrac.own 1) $$ Hh with ⟨_, %Hnn⟩
  ipureintro; exact Hnn

/-- Convert the fresh backing array into an inventory of unused zero slots. -/
theorem l2MessageStorage_init (data : GoSlice) (count : Nat) :
    data ↦* List.replicate count (zero_val L2ToL1Message) ⊢ (L2MessageStorage data 0 count : IProp GF) := by
  rw [ownSlice_unseal]
  unfold ownSliceDef L2MessageStorage
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
theorem l2MessageStorage_take (data : GoSlice) (used count : Nat) (more : used < count) :
    L2MessageStorage data used count ⊢ (iprop(
      L2MessagePointerOwned (sliceIndexRef L2ToL1Message (used : Int) data) (some l2MessageZero) ∗
      L2MessageStorage data (used + 1) count) : IProp GF) := by
  have hcount : count - used = (count - (used + 1)) + 1 := by omega
  unfold L2MessageStorage
  rw [hcount, List.replicate_succ]
  iintro Hslots
  icases (arrayElems_cons _ _ _ _).1 $$ Hslots with ⟨Hfirst, Hslots⟩
  isimp only [go.arrayIndexRef_0] at Hfirst
  have hnext : arrayIndexRef L2ToL1Message 1 (sliceIndexRef L2ToL1Message (used : Int) data) =
      sliceIndexRef L2ToL1Message ((used + 1 : Nat) : Int) data := by
    simp only [sliceIndexRef, ← go.arrayIndexRef_add]
    congr 1
  isimp only [hnext] at Hslots
  iframe Hslots
  iapply l2MessagePointerOwned_zero $$ Hfirst

theorem l2MessagePointersOwned_length (out : GoSlice) (values : List (Option L2MessageValue)) :
    L2MessagePointersOwned out values ⊢ (⌜sint.nat out.len = values.length ∧ out.ptr ≠ null⌝ : IProp GF) := by
  iintro Howned
  unfold L2MessagePointersOwned
  icases Howned with ⟨%ptrs, %Hnn, Hout, Hchildren⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hout
  ihave %Hchildren := l2MessagePointerChildren_length ptrs values $$ Hchildren
  ipureintro
  exact ⟨Hlen.1.symm.trans Hchildren, Hnn⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerChildren_nil
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerChildren_length
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerChildren_snoc
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerOwned_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerOwned_nonnull
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageStorage_init
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageStorage_take
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointersOwned_length
