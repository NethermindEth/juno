import ProofCheck.CasmClassRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Pointer elements consume null/undefined before allocating. Every other
    element decodes into an initially zero CasmClass; an empty map is a nonnil
    zero CasmClass, distinct from a nil element. The real suffix remains explicit. -/
inductive CasmClassPointerValid (d : Nat) : List w8 → List w8 → Option CasmClassValue → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      CasmClassPointerValid d [head] rest none
  | record {item rest value}
      (hn : (item ++ rest).head? ≠ some (W8 246)) (hu : (item ++ rest).head? ≠ some (W8 247))
      (valid : CasmClassRecordValid d casmClassZero item rest value) :
      CasmClassPointerValid d item rest (some value)

/-- Failed nonnil elements retain their private partial CasmClass and map depth;
    the pointer wrapper never publishes that failed allocation. -/
inductive CasmClassPointerRejected (d : Nat) : List w8 → CasmClassValue → Nat → Nat → Nat → CasmClassFailureReason → Prop where
  | record {wire retained consumed offset extraDepth reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : CasmClassRecordRejected d casmClassZero wire retained consumed offset extraDepth reason) :
      CasmClassPointerRejected d wire retained consumed offset extraDepth reason

theorem casmClassPointer_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ CasmClassPointerValid d item rest value) ∨
    (∃ retained consumed offset extraDepth reason, CasmClassPointerRejected d wire retained consumed offset extraDepth reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, none, rfl, .nullish tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases casmClassRecord_classify d casmClassZero wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
  · rw [split] at hn hu
    exact Or.inl ⟨item, rest, some final, split, .record hn hu valid⟩
  · exact Or.inr ⟨final, consumed, offset, extraDepth, reason, .record hn hu failure⟩

theorem casmClassPointer_nonempty (d : Nat) (item rest : List w8) (value : Option CasmClassValue)
    (valid : CasmClassPointerValid d item rest value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | record hn hu valid => exact casmClassRecord_nonempty _ _ _ _ _ valid

theorem casmClassPointerRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : CasmClassValue)
    (extraDepth : Nat) (reason : CasmClassFailureReason)
    (failure : CasmClassPointerRejected d wire retained consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | record hn hu failure => exact casmClassRecordRejected_bounds _ _ _ _ _ _ _ _ failure

theorem casmClassPointer_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : Option CasmClassValue)
    (first : CasmClassPointerValid d left restA a) (second : CasmClassPointerValid d right restB b)
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
      obtain ⟨rfl, rfl, rfl⟩ := casmClassRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem casmClassPointer_not_rejected (d consumed offset : Nat) (item rest : List w8) (value : Option CasmClassValue)
    (retained : CasmClassValue) (extraDepth : Nat) (reason : CasmClassFailureReason)
    (valid : CasmClassPointerValid d item rest value)
    (failure : CasmClassPointerRejected d (item ++ rest) retained consumed offset extraDepth reason) : False := by
  cases failure with
  | record hn hu failure =>
    cases valid with
    | nullish suffix nullish =>
      rcases nullish with rfl | rfl
      · exact hn rfl
      · exact hu rfl
    | record hn2 hu2 valid => exact casmClassRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure

theorem casmClassPointerRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : CasmClassValue) (extraDepthA extraDepthB : Nat) (reasonA reasonB : CasmClassFailureReason)
    (first : CasmClassPointerRejected d wire a consumedA offsetA extraDepthA reasonA)
    (second : CasmClassPointerRejected d wire b consumedB offsetB extraDepthB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ extraDepthA = extraDepthB ∧ reasonA = reasonB := by
  cases first with
  | record hn hu failure => cases second with
    | record hn2 hu2 failure2 => exact casmClassRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2

inductive CasmClassPointerOutcome where
  | success (value : Option CasmClassValue) (rest : List w8)
  | failure (retained : CasmClassValue) (consumed offset : Nat) (extraDepth : Nat) (reason : CasmClassFailureReason)

def CasmClassPointerSpec (d : Nat) (wire : List w8) : CasmClassPointerOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ CasmClassPointerValid d item rest value
  | .failure retained consumed offset extraDepth reason => CasmClassPointerRejected d wire retained consumed offset extraDepth reason

theorem casmClassPointerSpec_unique (d : Nat) (wire : List w8) (a b : CasmClassPointerOutcome)
    (first : CasmClassPointerSpec d wire a) (second : CasmClassPointerSpec d wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := casmClassPointer_prefix_unique d item item2 rest rest2 value value2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset extraDepth reason =>
      subst wire
      exact False.elim (casmClassPointer_not_rejected _ _ _ _ _ _ _ _ _ valid second)
  | failure retained consumed offset extraDepth reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (casmClassPointer_not_rejected _ _ _ _ _ _ _ _ _ valid first)
    | failure retained2 consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := casmClassPointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem casmClassPointerSpec_total_unique (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, CasmClassPointerSpec d wire outcome ∧
      ∀ other, CasmClassPointerSpec d wire other → other = outcome := by
  have existsOutcome : ∃ outcome, CasmClassPointerSpec d wire outcome := by
    rcases casmClassPointer_classify d wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure retained consumed offset extraDepth reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => casmClassPointerSpec_unique d wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPointer_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPointer_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPointerRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPointer_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPointer_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPointerRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPointerSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPointerSpec_total_unique
