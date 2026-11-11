import ProofCheck.ReceiptRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Pointer elements consume null/undefined before allocating. Every other
    element decodes into an initially zero TransactionReceipt; an empty map is a nonnil
    zero TransactionReceipt, distinct from a nil element. The real suffix remains explicit. -/
inductive ReceiptPointerValid (d : Nat) : List w8 → List w8 → Option ReceiptValue → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      ReceiptPointerValid d [head] rest none
  | record {item rest value}
      (hn : (item ++ rest).head? ≠ some (W8 246)) (hu : (item ++ rest).head? ≠ some (W8 247))
      (valid : ReceiptRecordValid d receiptZero item rest value) :
      ReceiptPointerValid d item rest (some value)

/-- Failed nonnil elements retain their private partial TransactionReceipt and retained nested depth;
    the pointer wrapper never publishes that failed allocation. -/
inductive ReceiptPointerRejected (d : Nat) : List w8 → ReceiptValue → Nat → Nat → Nat → ReceiptFailureReason → Prop where
  | record {wire retained consumed offset extraDepth reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ReceiptRecordRejected d receiptZero wire retained consumed offset extraDepth reason) :
      ReceiptPointerRejected d wire retained consumed offset extraDepth reason

theorem receiptPointer_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ ReceiptPointerValid d item rest value) ∨
    (∃ retained consumed offset extraDepth reason, ReceiptPointerRejected d wire retained consumed offset extraDepth reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, none, rfl, .nullish tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases receiptRecord_classify d receiptZero wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
  · rw [split] at hn hu
    exact Or.inl ⟨item, rest, some final, split, .record hn hu valid⟩
  · exact Or.inr ⟨final, consumed, offset, extraDepth, reason, .record hn hu failure⟩

theorem receiptPointer_nonempty (d : Nat) (item rest : List w8) (value : Option ReceiptValue)
    (valid : ReceiptPointerValid d item rest value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | record hn hu valid => exact receiptRecord_nonempty _ _ _ _ _ valid

theorem receiptPointerRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : ReceiptValue)
    (extraDepth : Nat) (reason : ReceiptFailureReason)
    (failure : ReceiptPointerRejected d wire retained consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | record hn hu failure => exact receiptRecordRejected_bounds _ _ _ _ _ _ _ _ failure

theorem receiptPointer_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : Option ReceiptValue)
    (first : ReceiptPointerValid d left restA a) (second : ReceiptPointerValid d right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | nullish suffix hn =>
    cases second with
    | nullish suffix2 hn2 =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | record hn2 hu2 valid2 =>
      rw [← same] at hn2 hu2
      rcases hn with rfl | rfl
      · exact False.elim (hn2 rfl)
      · exact False.elim (hu2 rfl)
  | record hn hu valid =>
    cases second with
    | nullish suffix2 hn2 =>
      rw [same] at hn hu
      rcases hn2 with rfl | rfl
      · exact False.elim (hn rfl)
      · exact False.elim (hu rfl)
    | record hn2 hu2 valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := receiptRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem receiptPointer_not_rejected (d consumed offset : Nat) (item rest : List w8) (value : Option ReceiptValue)
    (retained : ReceiptValue) (extraDepth : Nat) (reason : ReceiptFailureReason)
    (valid : ReceiptPointerValid d item rest value)
    (failure : ReceiptPointerRejected d (item ++ rest) retained consumed offset extraDepth reason) : False := by
  cases failure with
  | record hn hu failure =>
    cases valid with
    | nullish suffix nullish =>
      rcases nullish with rfl | rfl
      · exact hn rfl
      · exact hu rfl
    | record hn2 hu2 valid => exact receiptRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure

theorem receiptPointerRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : ReceiptValue) (extraDepthA extraDepthB : Nat) (reasonA reasonB : ReceiptFailureReason)
    (first : ReceiptPointerRejected d wire a consumedA offsetA extraDepthA reasonA)
    (second : ReceiptPointerRejected d wire b consumedB offsetB extraDepthB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ extraDepthA = extraDepthB ∧ reasonA = reasonB := by
  cases first with
  | record hn hu failure => cases second with
    | record hn2 hu2 failure2 => exact receiptRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2

inductive ReceiptPointerOutcome where
  | success (value : Option ReceiptValue) (rest : List w8)
  | failure (retained : ReceiptValue) (consumed offset : Nat) (extraDepth : Nat) (reason : ReceiptFailureReason)

def ReceiptPointerSpec (d : Nat) (wire : List w8) : ReceiptPointerOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ ReceiptPointerValid d item rest value
  | .failure retained consumed offset extraDepth reason => ReceiptPointerRejected d wire retained consumed offset extraDepth reason

theorem receiptPointerSpec_unique (d : Nat) (wire : List w8) (a b : ReceiptPointerOutcome)
    (first : ReceiptPointerSpec d wire a) (second : ReceiptPointerSpec d wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := receiptPointer_prefix_unique d item item2 rest rest2 value value2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset extraDepth reason =>
      subst wire
      exact False.elim (receiptPointer_not_rejected _ _ _ _ _ _ _ _ _ valid second)
  | failure retained consumed offset extraDepth reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (receiptPointer_not_rejected _ _ _ _ _ _ _ _ _ valid first)
    | failure retained2 consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := receiptPointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem receiptPointerSpec_total_unique (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, ReceiptPointerSpec d wire outcome ∧
      ∀ other, ReceiptPointerSpec d wire other → other = outcome := by
  have existsOutcome : ∃ outcome, ReceiptPointerSpec d wire outcome := by
    rcases receiptPointer_classify d wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure retained consumed offset extraDepth reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => receiptPointerSpec_unique d wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointer_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointer_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointer_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointer_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerSpec_total_unique
