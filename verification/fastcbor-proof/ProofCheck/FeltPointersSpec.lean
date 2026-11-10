import ProofCheck.SliceAllocation

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

/-- A nullable felt element with an independently specified encoding/value. -/
structure FeltWire (depth : Nat) where
  bytes : List w8
  value : Option (GoArray w64 4)
  valid : FeltValid depth bytes value

def feltPointer (data : GoSlice) (i : Nat) (value : Option (GoArray w64 4)) : Loc :=
  match value with
  | none => null
  | some _ => sliceIndexRef (GoArray w64 4) i data

def feltPointers (data : GoSlice) (values : List (Option (GoArray w64 4))) : List Loc :=
  values.mapIdx (fun i value => feltPointer (GF := GF) data i value)

def feltStorage (values : List (Option (GoArray w64 4))) : List (GoArray w64 4) :=
  values.map (fun value => value.getD (zero_val (GoArray w64 4)))

/-- Ownership of both the pointer table and its backing values. Nil entries keep
    their zeroed, unpublished backing slot; nonnil entries point at their own
    slot. Both slices are allocated even when the result is empty. -/
def FeltPointersOwned (out : GoSlice) (values : List (Option (GoArray w64 4))) : IProp GF := iprop(
  ∃ data : GoSlice, out ↦* feltPointers (GF := GF) data values ∗ data ↦* feltStorage values ∗
    ⌜out.ptr ≠ null ∧ data.ptr ≠ null⌝)

inductive FeltPointersValid : Nat → List w8 → Option (List (Option (GoArray w64 4))) → Prop where
  | nullish {d : Nat} {head : w8} (hn : head = W8 246 ∨ head = W8 247) :
      FeltPointersValid d [head] none
  | array {d : Nat} {head : w8} {payload : List w8} {count : w64}
      (items : List (FeltWire (d + 1))) (enc : HeadArgument head payload count)
      (major : head &&& W8 224 = W8 128) (size : uint.Z count = items.length) (bound : d < 32) :
      FeltPointersValid d ((head :: payload) ++ items.flatMap FeltWire.bytes) (some (items.map FeltWire.value))

def FeltPointersResult (out : GoSlice) (value : Option (List (Option (GoArray w64 4)))) : IProp GF :=
  match value with
  | none => iprop(⌜out = slice.nil⌝)
  | some values => FeltPointersOwned out values

theorem feltWire_nonempty (d : Nat) (item : FeltWire d) : 0 < item.bytes.length := by
  rcases item with ⟨bytes, value, valid⟩
  cases valid <;> simp only [List.length_cons, List.length_nil, List.length_append] <;> omega

theorem feltWire_count_bound (d : Nat) (items : List (FeltWire d)) :
    items.length ≤ (items.flatMap FeltWire.bytes).length := by
  induction items with
  | nil => simp
  | cons item items ih =>
    have := feltWire_nonempty d item
    simp only [List.length_cons, List.flatMap_cons, List.length_append]; omega

theorem feltPointers_length (data : GoSlice) (values : List (Option (GoArray w64 4))) :
    (feltPointers (GF := GF) data values).length = values.length := by simp [feltPointers]

theorem feltPointers_step (data : GoSlice) (values : List (Option (GoArray w64 4))) (value : Option (GoArray w64 4)) :
    feltPointers (GF := GF) data (values ++ [value]) = feltPointers (GF := GF) data values ++ [feltPointer (GF := GF) data values.length value] := by
  simp only [feltPointers, List.mapIdx_concat]

/-- Publishing the next element replaces just the first uninitialized slot. -/
theorem initialized_prefix_step {V : Type} (done : List V) (n : Nat) (zero value : V) :
    (done ++ List.replicate (n + 1) zero).set done.length value =
      (done ++ [value]) ++ List.replicate n zero := by
  rw [List.set_append_right _ _ (Nat.le_refl _)]
  simp [List.replicate_succ, List.append_assoc]

/-- Nullish value classification retains the exact one-byte representation. -/
theorem feltValid_none (d : Nat) (wire : List w8) (valid : FeltValid d wire none) :
    ∃ head, wire = [head] ∧ (head = W8 246 ∨ head = W8 247) := by
  cases valid with
  | nullish hn => exact ⟨_, rfl, hn⟩

/-- Every nonnil accepted felt starts with an array head, before any shortcut. -/
theorem feltValid_some_head (d : Nat) (wire : List w8) (words : GoArray w64 4)
    (valid : FeltValid d wire (some words)) :
    ∃ head tail, wire = head :: tail ∧ head &&& W8 224 = W8 128 := by
  cases valid with
  | small items enc major count small bound => exact ⟨_, _, rfl, major⟩
  | extended items extras enc major count four bound valid => exact ⟨_, _, rfl, major⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltWire_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.feltWire_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointers_length
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointers_step
#print axioms Perennial.github_com.NethermindEth.juno.core.initialized_prefix_step

#print axioms Perennial.github_com.NethermindEth.juno.core.feltValid_none
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValid_some_head
