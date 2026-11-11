import ProofCheck.LimbsComplete
import ProofCheck.FeltDeterminism

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A sequence of in-place felt values. Unlike felt pointers, these elements do
    not accept null/undefined. Each value is exactly four Montgomery limbs. -/
inductive LimbItemsValid (d : Nat) : Nat → List w8 → List (GoArray w64 4) → Prop where
  | nil : LimbItemsValid d 0 [] []
  | cons {n item tail value values}
      (head : FeltValid d item (some value)) (rest : LimbItemsValid d n tail values) :
      LimbItemsValid d (n + 1) (item ++ tail) (value :: values)

/-- A rejected sequence identifies the successful, already-written prefix and
    two positions: the failing element start and its inner diagnostic offset. -/
inductive LimbItemsRejected (d : Nat) : Nat → List w8 → List (GoArray w64 4) → Nat → Nat → FeltFailureReason → Prop where
  | first {n wire offset reason} (failure : LimbsFailure d wire offset reason) :
      LimbItemsRejected d (n + 1) wire [] 0 offset reason
  | next {n item tail value done consumed offset reason}
      (valid : FeltValid d item (some value))
      (failure : LimbItemsRejected d n tail done consumed offset reason) :
      LimbItemsRejected d (n + 1) (item ++ tail) (value :: done)
        (item.length + consumed) (item.length + offset) reason

theorem limbItems_classify (d n : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest values, wire = item ++ rest ∧ LimbItemsValid d n item values) ∨
    (∃ done consumed offset reason, LimbItemsRejected d n wire done consumed offset reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil⟩
  | succ n ih =>
    rcases limbs_classify d wire depth with ⟨item, tail, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, validRest⟩ | ⟨done, consumed, offset, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, by simp only [List.append_assoc], .cons valid validRest⟩
      · exact Or.inr ⟨value :: done, item.length + consumed, item.length + offset, reason, .next valid failure⟩
    · exact Or.inr ⟨[], 0, offset, reason, .first failure⟩

theorem limbItems_valid_length (d n : Nat) (wire : List w8) (values : List (GoArray w64 4))
    (valid : LimbItemsValid d n wire values) : values.length = n := by
  induction valid <;> simp_all

theorem limbItems_valid_count_bound (d n : Nat) (wire : List w8) (values : List (GoArray w64 4))
    (valid : LimbItemsValid d n wire values) : n ≤ wire.length := by
  induction valid with
  | nil => simp
  | @cons n item tail value values head rest ih =>
    have nonempty : 0 < item.length := by
      cases head <;> simp
    simp only [List.length_append]
    omega

theorem limbItems_rejected_bounds (d n consumed offset : Nat) (wire : List w8)
    (done : List (GoArray w64 4)) (reason : FeltFailureReason)
    (failure : LimbItemsRejected d n wire done consumed offset reason) :
    done.length < n ∧ consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | first failure =>
    have bound := limbsFailure_offset _ _ _ _ failure
    simp only [List.length_nil]
    omega
  | next valid failure ih =>
    simp only [List.length_cons, List.length_append]
    omega

/-- The number of requested elements fixes a unique decoded value list and
    consumed boundary, even when individual felts use different limb widths. -/
theorem limbItems_prefix_unique (d n : Nat) (left right restA restB : List w8)
    (a b : List (GoArray w64 4)) (first : LimbItemsValid d n left a)
    (second : LimbItemsValid d n right b) (same : left ++ restA = right ++ restB) :
    left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right b with
  | nil => cases second; exact ⟨rfl, rfl, same⟩
  | @cons n item tail value values head rest ih =>
    cases second with
    | @cons n2 item2 tail2 value2 values2 head2 rest2 =>
      have split : item ++ (tail ++ restA) = item2 ++ (tail2 ++ restB) := by simpa only [List.append_assoc] using same
      obtain ⟨rfl, valuesEq, tailEq⟩ := feltValid_prefix_unique d _ _ item item2 (tail ++ restA) (tail2 ++ restB) head head2 split
      have eq : value = value2 := Option.some.inj valuesEq
      subst value2
      obtain ⟨rfl, rfl, rfl⟩ := ih tail2 values2 rest2 tailEq
      exact ⟨rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_valid_length
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_valid_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_prefix_unique
