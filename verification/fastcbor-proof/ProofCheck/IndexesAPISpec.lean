import ProofCheck.IndexesRecordSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Public errors distinguish the destination guard, a record diagnostic at
    its exact input offset, and the count of extraneous trailing bytes. -/
inductive IndexesAPIError where
  | nonzero
  | decode (offset : Nat) (reason : IndexesRecordFailureReason)
  | trailing (extra : Nat)

/-- Only successful First decoding exposes an unread suffix. Error results
    carry the final destination value; the concrete nil suffix is a WP obligation. -/
inductive IndexesFirstResult where
  | success (value : IndexesValue) (rest : List w8)
  | failure (value : IndexesValue) (error : IndexesAPIError)

/-- Independent public First contract, including the zero-destination guard
    and actual API policy of clearing every partially decoded Indexes on error. -/
inductive IndexesFirstSpec : IndexesValue → List w8 → IndexesFirstResult → Prop where
  | nonzero (old : IndexesValue) (wire : List w8) (nonzero : old ≠ indexesZeroValue) :
      IndexesFirstSpec old wire (.failure old .nonzero)
  | success {item rest : List w8} {value : IndexesValue}
      (valid : IndexesRecordValid 0 indexesZeroValue item value) :
      IndexesFirstSpec indexesZeroValue (item ++ rest) (.success value rest)
  | rejected {wire : List w8} {retained : IndexesValue} {consumed offset extraDepth : Nat}
      {reason : IndexesRecordFailureReason}
      (failure : IndexesRecordRejected 0 indexesZeroValue wire retained consumed offset extraDepth reason) :
      IndexesFirstSpec indexesZeroValue wire (.failure indexesZeroValue (.decode offset reason))

inductive IndexesDecodeResult where
  | success (value : IndexesValue)
  | failure (value : IndexesValue) (error : IndexesAPIError)

/-- Full decoding inherits First's result and rejects any nonempty suffix,
    clearing the otherwise successfully decoded record before returning an error. -/
inductive IndexesFullSpec (old : IndexesValue) (wire : List w8) : IndexesDecodeResult → Prop where
  | firstError {value : IndexesValue} {error : IndexesAPIError}
      (first : IndexesFirstSpec old wire (.failure value error)) :
      IndexesFullSpec old wire (.failure value error)
  | success {value : IndexesValue} (first : IndexesFirstSpec old wire (.success value [])) :
      IndexesFullSpec old wire (.success value)
  | trailing {value : IndexesValue} {rest : List w8}
      (first : IndexesFirstSpec old wire (.success value rest)) (nonempty : rest ≠ []) :
      IndexesFullSpec old wire (.failure indexesZeroValue (.trailing rest.length))

/-- Every initial destination and input has a public First specification outcome.
    This is not a WP theorem for the generated Go function. -/
theorem indexesFirstSpec_classify (old : IndexesValue) (wire : List w8) :
    ∃ result, IndexesFirstSpec old wire result := by
  classical
  by_cases zero : old = indexesZeroValue
  · subst old
    rcases indexesRecord_classify 0 indexesZeroValue wire (by decide) with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · subst wire
      exact ⟨.success value rest, .success valid⟩
    · exact ⟨.failure indexesZeroValue (.decode offset reason), .rejected failure⟩
  · exact ⟨.failure old .nonzero, .nonzero old wire zero⟩

theorem indexesFullSpec_classify (old : IndexesValue) (wire : List w8) :
    ∃ result, IndexesFullSpec old wire result := by
  obtain ⟨result, first⟩ := indexesFirstSpec_classify old wire
  cases result with
  | failure value error => exact ⟨.failure value error, .firstError first⟩
  | success value rest =>
    by_cases empty : rest = []
    · subst rest
      exact ⟨.success value, .success first⟩
    · exact ⟨.failure indexesZeroValue (.trailing rest.length), .trailing first empty⟩

/-- Success retains the exact byte suffix and requires an initially zero record. -/
theorem indexesFirstSpec_success (old value : IndexesValue) (wire rest : List w8)
    (spec : IndexesFirstSpec old wire (.success value rest)) :
    old = indexesZeroValue ∧ ∃ item, wire = item ++ rest ∧ IndexesRecordValid 0 indexesZeroValue item value := by
  cases spec with
  | success valid => exact ⟨rfl, _, rfl, valid⟩

/-- First failures preserve a rejected nonzero destination or clear a record
    that was initially zero. No partially parsed Indexes escapes as a result. -/
theorem indexesFirstSpec_cleanup (old value : IndexesValue) (wire : List w8) (error : IndexesAPIError)
    (spec : IndexesFirstSpec old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ (old = indexesZeroValue ∧ value = indexesZeroValue) := by
  cases spec with
  | nonzero old wire nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected failure => exact Or.inr ⟨rfl, rfl⟩

/-- Both parse failures and trailing-data failures clear output; the zero guard
    alone preserves a rejected nonzero destination. -/
theorem indexesFullSpec_cleanup (old value : IndexesValue) (wire : List w8) (error : IndexesAPIError)
    (spec : IndexesFullSpec old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ value = indexesZeroValue := by
  cases spec with
  | firstError first =>
    rcases indexesFirstSpec_cleanup old value wire error first with guard | cleared
    · exact Or.inl guard
    · exact Or.inr cleared.2
  | trailing first nonempty => exact Or.inr rfl

/-- Full success consumes the whole input, rather than merely a valid prefix. -/
theorem indexesFullSpec_success (old value : IndexesValue) (wire : List w8)
    (spec : IndexesFullSpec old wire (.success value)) :
    old = indexesZeroValue ∧ IndexesRecordValid 0 indexesZeroValue wire value := by
  cases spec with
  | success first =>
    obtain ⟨zero, item, split, valid⟩ := indexesFirstSpec_success old value wire [] first
    simp only [List.append_nil] at split
    subst wire
    exact ⟨zero, valid⟩

theorem indexesFirstSpec_diagnostic_bound (old value : IndexesValue) (wire : List w8)
    (offset : Nat) (reason : IndexesRecordFailureReason)
    (spec : IndexesFirstSpec old wire (.failure value (.decode offset reason))) : offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (indexesRecordRejected_bounds _ _ _ _ _ _ _ _ (by decide) failure).2.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFullSpec_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_success
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFullSpec_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFullSpec_success
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_diagnostic_bound
