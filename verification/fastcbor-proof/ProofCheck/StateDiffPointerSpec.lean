import ProofCheck.StateDiffRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Pointer elements consume null/undefined before allocating. Every other
    element decodes into an initially zero StateDiff; an empty map is a nonnil
    zero StateDiff, distinct from a nil element. The real suffix remains explicit. -/
inductive StateDiffPointerValid (d : Nat) : List w8 → List w8 → Option StateDiffValue → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      StateDiffPointerValid d [head] rest none
  | record {item rest value}
      (hn : (item ++ rest).head? ≠ some (W8 246)) (hu : (item ++ rest).head? ≠ some (W8 247))
      (valid : StateDiffRecordValid d stateDiffZero item rest value) :
      StateDiffPointerValid d item rest (some value)

/-- Failed nonnil elements retain their private partial StateDiff and map depth;
    the pointer wrapper never publishes that failed allocation. -/
inductive StateDiffPointerRejected (d : Nat) : List w8 → StateDiffValue → Nat → Nat → Nat → StateDiffFailureReason → Prop where
  | record {wire retained consumed offset extraDepth reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : StateDiffRecordRejected d stateDiffZero wire retained consumed offset extraDepth reason) :
      StateDiffPointerRejected d wire retained consumed offset extraDepth reason

theorem stateDiffPointer_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ StateDiffPointerValid d item rest value) ∨
    (∃ retained consumed offset extraDepth reason, StateDiffPointerRejected d wire retained consumed offset extraDepth reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, none, rfl, .nullish tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases stateDiffRecord_classify d stateDiffZero wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
  · rw [split] at hn hu
    exact Or.inl ⟨item, rest, some final, split, .record hn hu valid⟩
  · exact Or.inr ⟨final, consumed, offset, extraDepth, reason, .record hn hu failure⟩

theorem stateDiffPointer_nonempty (d : Nat) (item rest : List w8) (value : Option StateDiffValue)
    (valid : StateDiffPointerValid d item rest value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | record hn hu valid => exact stateDiffRecord_nonempty _ _ _ _ _ valid

theorem stateDiffPointerRejected_bounds (d consumed offset : Nat) (wire : List w8) (retained : StateDiffValue)
    (extraDepth : Nat) (reason : StateDiffFailureReason)
    (failure : StateDiffPointerRejected d wire retained consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | record hn hu failure => exact stateDiffRecordRejected_bounds _ _ _ _ _ _ _ _ failure

theorem stateDiffPointer_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : Option StateDiffValue)
    (first : StateDiffPointerValid d left restA a) (second : StateDiffPointerValid d right restB b)
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
      obtain ⟨rfl, rfl, rfl⟩ := stateDiffRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem stateDiffPointer_not_rejected (d consumed offset : Nat) (item rest : List w8) (value : Option StateDiffValue)
    (retained : StateDiffValue) (extraDepth : Nat) (reason : StateDiffFailureReason)
    (valid : StateDiffPointerValid d item rest value)
    (failure : StateDiffPointerRejected d (item ++ rest) retained consumed offset extraDepth reason)
    (bound : (item ++ rest).length < 2^64) : False := by
  cases failure with
  | record hn hu failure =>
    cases valid with
    | nullish suffix nullish =>
      rcases nullish with rfl | rfl
      · exact hn rfl
      · exact hu rfl
    | record hn2 hu2 valid => exact stateDiffRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure bound

theorem stateDiffPointerRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : StateDiffValue) (extraDepthA extraDepthB : Nat) (reasonA reasonB : StateDiffFailureReason)
    (first : StateDiffPointerRejected d wire a consumedA offsetA extraDepthA reasonA)
    (second : StateDiffPointerRejected d wire b consumedB offsetB extraDepthB reasonB)
    (bound : wire.length < 2^64) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ extraDepthA = extraDepthB ∧ reasonA = reasonB := by
  cases first with
  | record hn hu failure => cases second with
    | record hn2 hu2 failure2 => exact stateDiffRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2 bound

/-- Failed child maps retain their depth, but never exceed the supported limit. -/
theorem stateDiffPointerRejected_depth (d consumed offset extraDepth : Nat) (wire : List w8)
    (retained : StateDiffValue) (reason : StateDiffFailureReason) (depth : d ≤ 32)
    (failure : StateDiffPointerRejected d wire retained consumed offset extraDepth reason) :
    d + extraDepth ≤ 32 := by
  cases failure with
  | record hn hu failure => exact stateDiffRecordRejected_depth _ _ _ _ _ _ _ _ depth failure

inductive StateDiffPointerOutcome where
  | success (value : Option StateDiffValue) (rest : List w8)
  | failure (retained : StateDiffValue) (consumed offset : Nat) (extraDepth : Nat) (reason : StateDiffFailureReason)

def StateDiffPointerSpec (d : Nat) (wire : List w8) : StateDiffPointerOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ StateDiffPointerValid d item rest value
  | .failure retained consumed offset extraDepth reason => StateDiffPointerRejected d wire retained consumed offset extraDepth reason

theorem stateDiffPointerSpec_unique (d : Nat) (wire : List w8) (a b : StateDiffPointerOutcome)
    (first : StateDiffPointerSpec d wire a) (second : StateDiffPointerSpec d wire b) (bound : wire.length < 2^64) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := stateDiffPointer_prefix_unique d item item2 rest rest2 value value2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset extraDepth reason =>
      subst wire
      exact False.elim (stateDiffPointer_not_rejected _ _ _ _ _ _ _ _ _ valid second bound)
  | failure retained consumed offset extraDepth reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (stateDiffPointer_not_rejected _ _ _ _ _ _ _ _ _ valid first bound)
    | failure retained2 consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := stateDiffPointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ first second bound
      rfl

theorem stateDiffPointerSpec_total_unique (d : Nat) (wire : List w8) (depth : d ≤ 32) (bound : wire.length < 2^64) :
    ∃ outcome, StateDiffPointerSpec d wire outcome ∧
      ∀ other, StateDiffPointerSpec d wire other → other = outcome := by
  have existsOutcome : ∃ outcome, StateDiffPointerSpec d wire outcome := by
    rcases stateDiffPointer_classify d wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure retained consumed offset extraDepth reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => stateDiffPointerSpec_unique d wire other outcome otherSpec spec bound⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointer_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointer_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointerRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointer_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointer_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointerRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointerSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointerSpec_total_unique

#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointerRejected_depth
