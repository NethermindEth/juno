import ProofCheck.StateUpdateRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive StateUpdateAPIError where
  | nonzero
  | decode (offset : Nat) (reason : StateUpdateFailureReason)
  | trailing (extra : Nat)

inductive StateUpdateFirstResult where
  | success (value : StateUpdateValue) (rest : List w8)
  | failure (value : StateUpdateValue) (error : StateUpdateAPIError)

/-- Independent caller contract: nonzero destinations are rejected unchanged;
    parsing starts from zero, success exposes the unread suffix, and any record
    failure clears all partially published fields before returning its error. -/
inductive StateUpdateFirstSpec :
    StateUpdateValue → List w8 → StateUpdateFirstResult → Prop where
  | nonzero {old wire} (nonzero : old ≠ stateUpdateZero) :
      StateUpdateFirstSpec old wire (.failure old .nonzero)
  | success {wire final rest}
      (valid : StateUpdateRecordSpec 0 stateUpdateZero wire (.success final rest)) :
      StateUpdateFirstSpec stateUpdateZero wire (.success final rest)
  | rejected {wire retained consumed offset extraDepth reason}
      (failure : StateUpdateRecordSpec 0 stateUpdateZero wire (.failure retained consumed offset extraDepth reason)) :
      StateUpdateFirstSpec stateUpdateZero wire (.failure stateUpdateZero (.decode offset reason))

/-- Every initial destination and input has a specified First outcome, without
    an implementation or helper-correctness premise. -/
theorem stateUpdateFirst_classify (old : StateUpdateValue) (wire : List w8) :
    ∃ result, StateUpdateFirstSpec old wire result := by
  classical
  by_cases zero : old = stateUpdateZero
  · subst old
    rcases stateUpdateRecord_classify 0 stateUpdateZero wire (by decide) with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨_, .success ⟨item, split, valid⟩⟩
    · exact ⟨_, .rejected failure⟩
  · exact ⟨_, .nonzero zero⟩

/-- Uniqueness includes decoded values, exact suffix and public error
    reason/offset; cleanup hides the record reader's retained partial value. -/
theorem stateUpdateFirst_unique (old : StateUpdateValue) (wire : List w8)
    (a b : StateUpdateFirstResult) (first : StateUpdateFirstSpec old wire a)
    (second : StateUpdateFirstSpec old wire b) (bound : wire.length < 2^64) : a = b := by
  cases first with
  | nonzero nonzero =>
    cases second with
    | nonzero => rfl
    | success => exact False.elim (nonzero rfl)
    | rejected => exact False.elim (nonzero rfl)
  | success valid =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid2 =>
      have same := stateUpdateRecordSpec_unique 0 stateUpdateZero wire _ _ valid valid2 bound
      cases same
      rfl
    | rejected failure =>
      have same := stateUpdateRecordSpec_unique 0 stateUpdateZero wire _ _ valid failure bound
      cases same
  | rejected failure =>
    cases second with
    | nonzero nonzero => exact False.elim (nonzero rfl)
    | success valid =>
      have same := stateUpdateRecordSpec_unique 0 stateUpdateZero wire _ _ failure valid bound
      cases same
    | rejected failure2 =>
      have same := stateUpdateRecordSpec_unique 0 stateUpdateZero wire _ _ failure failure2 bound
      cases same
      rfl

/-- A rejected nonzero destination is preserved; all parser failures clear it. -/
theorem stateUpdateFirst_cleanup (old final : StateUpdateValue)
    (wire : List w8) (error : StateUpdateAPIError)
    (spec : StateUpdateFirstSpec old wire (.failure final error)) :
    (final = old ∧ error = .nonzero) ∨ (old = stateUpdateZero ∧ final = stateUpdateZero) := by
  cases spec with
  | nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected => exact Or.inr ⟨rfl, rfl⟩

/-- Successful First calls start from zero and satisfy the complete independent
    record relation, rather than a weaker wrapper-specific encoding rule. -/
theorem stateUpdateFirst_success (old final : StateUpdateValue)
    (wire rest : List w8) (spec : StateUpdateFirstSpec old wire (.success final rest)) :
    old = stateUpdateZero ∧ StateUpdateRecordSpec 0 stateUpdateZero wire (.success final rest) := by
  cases spec with
  | success valid => exact ⟨rfl, valid⟩

/-- The reported parser offset is within the supplied input or exactly at EOF,
    including failures inside the nested StateDiff record. -/
theorem stateUpdateFirst_diagnostic_bound (old final : StateUpdateValue)
    (wire : List w8) (offset : Nat) (reason : StateUpdateFailureReason)
    (spec : StateUpdateFirstSpec old wire (.failure final (.decode offset reason))) :
    offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (stateUpdateRecordRejected_bounds _ _ _ _ _ _ _ _ failure).2


inductive StateUpdateDecodeResult where
  | success (value : StateUpdateValue)
  | failure (value : StateUpdateValue) (error : StateUpdateAPIError)

/-- Full decoding rejects a nonempty suffix and clears the decoded state update. -/
inductive StateUpdateFullSpec (old : StateUpdateValue) (wire : List w8) : StateUpdateDecodeResult → Prop where
  | firstError {value error} (first : StateUpdateFirstSpec old wire (.failure value error)) :
      StateUpdateFullSpec old wire (.failure value error)
  | success {value} (first : StateUpdateFirstSpec old wire (.success value [])) :
      StateUpdateFullSpec old wire (.success value)
  | trailing {value rest} (first : StateUpdateFirstSpec old wire (.success value rest)) (nonempty : rest ≠ []) :
      StateUpdateFullSpec old wire (.failure stateUpdateZero (.trailing rest.length))

theorem stateUpdateFull_classify (old : StateUpdateValue) (wire : List w8) :
    ∃ result, StateUpdateFullSpec old wire result := by
  obtain ⟨result, first⟩ := stateUpdateFirst_classify old wire
  cases result with
  | failure value error => exact ⟨_, .firstError first⟩
  | success value rest =>
    by_cases empty : rest = []
    · subst rest; exact ⟨_, .success first⟩
    · exact ⟨_, .trailing first empty⟩

theorem stateUpdateFull_cleanup (old value : StateUpdateValue) (wire : List w8) (error : StateUpdateAPIError)
    (spec : StateUpdateFullSpec old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ value = stateUpdateZero := by
  cases spec with
  | firstError first =>
    rcases stateUpdateFirst_cleanup old value wire error first with guard | cleared
    · exact Or.inl guard
    · exact Or.inr cleared.2
  | trailing => exact Or.inr rfl

theorem stateUpdateFull_success (old value : StateUpdateValue) (wire : List w8)
    (spec : StateUpdateFullSpec old wire (.success value)) :
    old = stateUpdateZero ∧ StateUpdateRecordValid 0 stateUpdateZero wire [] value := by
  cases spec with
  | success first =>
    obtain ⟨zero, item, split, valid⟩ := stateUpdateFirst_success old value wire [] first
    simp only [List.append_nil] at split
    subst wire
    exact ⟨zero, valid⟩

theorem stateUpdateFull_unique (old : StateUpdateValue) (wire : List w8) (a b : StateUpdateDecodeResult)
    (first : StateUpdateFullSpec old wire a) (second : StateUpdateFullSpec old wire b) (bound : wire.length < 2^64) : a = b := by
  cases first with
  | firstError f => cases second with
    | firstError s => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h; rfl
    | success s => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h
    | trailing s _ => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h
  | success f => cases second with
    | firstError s => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h
    | success s => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h; rfl
    | trailing s ne => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h; exact False.elim (ne rfl)
  | trailing f ne => cases second with
    | firstError s => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h
    | success s => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h; exact False.elim (ne rfl)
    | trailing s _ => have h := stateUpdateFirst_unique old wire _ _ f s bound; cases h; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFirst_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFirst_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFirst_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFirst_success

#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFirst_diagnostic_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFull_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFull_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFull_success
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFull_unique
