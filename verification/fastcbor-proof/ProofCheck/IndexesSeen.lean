import ProofCheck.IndexesRecordSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def indexesMask (field : Bool) : w64 := if field then W64 2 else W64 1

def indexesMark (seen : w64) (choice : Option Bool) : w64 :=
  match choice with
  | none => seen
  | some field => seen ||| indexesMask field

/-- Relate the production bit mask to the independent set of visited fields. -/
def IndexesSeen (seen : w64) (fields : List Bool) : Prop :=
  ∀ field, seen &&& indexesMask field = W64 0 ↔ field ∉ fields

theorem indexesMasks_disjoint (a b : Bool) :
    indexesMask a &&& indexesMask b = W64 0 ↔ a ≠ b := by
  cases a <;> cases b <;> decide

theorem indexesFresh_mark_iff (seen : w64) (choice : Option Bool) (field : Bool) :
    indexesMark seen choice &&& indexesMask field = W64 0 ↔
      seen &&& indexesMask field = W64 0 ∧ choice ≠ some field := by
  cases choice with
  | none => simp [indexesMark]
  | some found =>
    have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
    simp only [indexesMark, BitVec.and_or_distrib_right, hor, indexesMasks_disjoint, ne_eq, Option.some.injEq]

theorem indexesSeen_zero : IndexesSeen (W64 0) [] := by
  intro field
  simp [BitVec.zero_and]

theorem indexesSeen_mark (seen : w64) (fields : List Bool) (choice : Option Bool)
    (hs : IndexesSeen seen fields) : IndexesSeen (indexesMark seen choice) (indexesSeen fields choice) := by
  intro field
  rw [indexesFresh_mark_iff, hs field]
  cases choice <;> simp [indexesSeen, List.mem_cons, and_comm, eq_comm]

/-- The generated numeric dispatch and bit tests select exactly the specified
    fresh field; duplicate keys select the ignored-value branch. -/
theorem indexesChoice_mask (seen : w64) (fields : List Bool) (key : Option w64)
    (hs : IndexesSeen seen fields) :
    indexesChoice fields key = match key with
      | none => none
      | some number =>
        if number = W64 1 ∧ seen &&& W64 1 = W64 0 then some false
        else if number = W64 2 ∧ seen &&& W64 2 = W64 0 then some true
        else none := by
  cases key with
  | none => rfl
  | some number =>
    have ht := hs false
    have hr := hs true
    simp only [indexesMask, Bool.false_eq_true, ite_false, ite_true] at ht hr
    simp only [indexesChoice, ← ht, ← hr]

theorem indexesChoice_fresh (fields : List Bool) (key : Option w64) (field : Bool)
    (selected : indexesChoice fields key = some field) : field ∉ fields := by
  cases key with
  | none => simp [indexesChoice] at selected
  | some number =>
    simp only [indexesChoice] at selected
    split at selected
    · obtain ⟨_, fresh⟩ := ‹number = W64 1 ∧ false ∉ fields›
      have heq : false = field := Option.some.inj selected
      simpa only [← heq] using fresh
    · split at selected
      · obtain ⟨_, fresh⟩ := ‹number = W64 2 ∧ true ∉ fields›
        have heq : true = field := Option.some.inj selected
        simpa only [← heq] using fresh
      · contradiction

def indexesValueField (value : IndexesValue) (field : Bool) : Option (List Int) :=
  if field then value.receipts else value.transactions

/-- A successful pair cannot replace a value whose field was already visited. -/
theorem indexesItem_preserves_seen (d : Nat) (seen : List Bool) (key : Option w64)
    (initial final : IndexesValue) (wire : List w8) (field : Bool) (visited : field ∈ seen)
    (valid : IndexesItemValid d (indexesChoice seen key) initial wire final) :
    indexesValueField final field = indexesValueField initial field := by
  generalize hc : indexesChoice seen key = choice at valid
  cases valid with
  | ignored _ => rfl
  | @selected old selected wire value valid =>
    have fresh := indexesChoice_fresh seen key selected hc
    have hne : selected ≠ field := by intro heq; subst selected; exact fresh visited
    cases selected <;> cases field <;> simp_all [indexesValueField, indexesSet]

/-- First-key-wins holds across every successful suffix of the record, including
    duplicates with arbitrary Skip-valid values and interleaved unknown keys. -/
theorem indexesPairs_preserves_seen (d n : Nat) (seen : List Bool) (initial final : IndexesValue)
    (wire : List w8) (field : Bool) (visited : field ∈ seen)
    (valid : IndexesPairsValid d n seen initial wire final) :
    indexesValueField final field = indexesValueField initial field := by
  induction valid with
  | nil => rfl
  | @step n seen key initial middle final keyBytes value tail keyValid valueValid rest ih =>
    have hv := indexesItem_preserves_seen d seen key initial middle value field visited valueValid
    have hm : field ∈ indexesSeen seen (indexesChoice seen key) := by
      cases hc : indexesChoice seen key <;> simp [indexesSeen, visited]
    exact (ih hm).trans hv

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesMasks_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFresh_mark_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesSeen_mark
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesChoice_mask
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesChoice_fresh
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesItem_preserves_seen
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesPairs_preserves_seen
