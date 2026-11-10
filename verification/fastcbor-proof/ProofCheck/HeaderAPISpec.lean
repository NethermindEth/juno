import ProofCheck.HeaderRecordSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Public errors distinguish the destination guard, a record diagnostic at
    its exact input offset, and the count of extraneous trailing bytes. -/
inductive HeaderAPIError where
  | nonzero
  | decode (offset : Nat) (reason : HeaderRecordFailureReason)
  | trailing (extra : Nat)

/-- Only successful First decoding exposes an unread suffix. Error results
    carry the final destination value; the concrete nil suffix is a WP obligation. -/
inductive HeaderFirstResult where
  | success (value : HeaderValue) (rest : List w8)
  | failure (value : HeaderValue) (error : HeaderAPIError)

/-- Independent public First contract, including the zero-destination guard
    and actual API policy of clearing every partially decoded Header on error. -/
inductive HeaderFirstSpec : HeaderValue → List w8 → HeaderFirstResult → Prop where
  | nonzero (old : HeaderValue) (wire : List w8) (nonzero : old ≠ headerZeroValue) :
      HeaderFirstSpec old wire (.failure old .nonzero)
  | success {item rest : List w8} {value : HeaderValue}
      (valid : HeaderRecordValid 0 headerZeroValue item value) :
      HeaderFirstSpec headerZeroValue (item ++ rest) (.success value rest)
  | rejected {wire : List w8} {retained : HeaderValue} {consumed offset extraDepth : Nat}
      {reason : HeaderRecordFailureReason}
      (failure : HeaderRecordRejected 0 headerZeroValue wire retained consumed offset extraDepth reason) :
      HeaderFirstSpec headerZeroValue wire (.failure headerZeroValue (.decode offset reason))

inductive HeaderDecodeResult where
  | success (value : HeaderValue)
  | failure (value : HeaderValue) (error : HeaderAPIError)

/-- Full decoding inherits First's result and rejects any nonempty suffix,
    clearing the otherwise successfully decoded record before returning an error. -/
inductive HeaderFullSpec (old : HeaderValue) (wire : List w8) : HeaderDecodeResult → Prop where
  | firstError {value : HeaderValue} {error : HeaderAPIError}
      (first : HeaderFirstSpec old wire (.failure value error)) :
      HeaderFullSpec old wire (.failure value error)
  | success {value : HeaderValue} (first : HeaderFirstSpec old wire (.success value [])) :
      HeaderFullSpec old wire (.success value)
  | trailing {value : HeaderValue} {rest : List w8}
      (first : HeaderFirstSpec old wire (.success value rest)) (nonempty : rest ≠ []) :
      HeaderFullSpec old wire (.failure headerZeroValue (.trailing rest.length))

/-- Every initial destination and input has a public First specification outcome.
    This is not a WP theorem for the generated Go function. -/
theorem headerFirstSpec_classify (old : HeaderValue) (wire : List w8) :
    ∃ result, HeaderFirstSpec old wire result := by
  classical
  by_cases zero : old = headerZeroValue
  · subst old
    rcases headerRecord_classify 0 headerZeroValue wire (by decide) with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · subst wire
      exact ⟨.success value rest, .success valid⟩
    · exact ⟨.failure headerZeroValue (.decode offset reason), .rejected failure⟩
  · exact ⟨.failure old .nonzero, .nonzero old wire zero⟩

theorem headerFullSpec_classify (old : HeaderValue) (wire : List w8) :
    ∃ result, HeaderFullSpec old wire result := by
  obtain ⟨result, first⟩ := headerFirstSpec_classify old wire
  cases result with
  | failure value error => exact ⟨.failure value error, .firstError first⟩
  | success value rest =>
    by_cases empty : rest = []
    · subst rest
      exact ⟨.success value, .success first⟩
    · exact ⟨.failure headerZeroValue (.trailing rest.length), .trailing first empty⟩

/-- Success retains the exact byte suffix and requires an initially zero record. -/
theorem headerFirstSpec_success (old value : HeaderValue) (wire rest : List w8)
    (spec : HeaderFirstSpec old wire (.success value rest)) :
    old = headerZeroValue ∧ ∃ item, wire = item ++ rest ∧ HeaderRecordValid 0 headerZeroValue item value := by
  cases spec with
  | success valid => exact ⟨rfl, _, rfl, valid⟩

/-- First failures preserve a rejected nonzero destination or clear a record
    that was initially zero. No partially parsed Header escapes as a result. -/
theorem headerFirstSpec_cleanup (old value : HeaderValue) (wire : List w8) (error : HeaderAPIError)
    (spec : HeaderFirstSpec old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ (old = headerZeroValue ∧ value = headerZeroValue) := by
  cases spec with
  | nonzero old wire nonzero => exact Or.inl ⟨rfl, rfl⟩
  | rejected failure => exact Or.inr ⟨rfl, rfl⟩

/-- Both parse failures and trailing-data failures clear output; the zero guard
    alone preserves a rejected nonzero destination. -/
theorem headerFullSpec_cleanup (old value : HeaderValue) (wire : List w8) (error : HeaderAPIError)
    (spec : HeaderFullSpec old wire (.failure value error)) :
    (value = old ∧ error = .nonzero) ∨ value = headerZeroValue := by
  cases spec with
  | firstError first =>
    rcases headerFirstSpec_cleanup old value wire error first with guard | cleared
    · exact Or.inl guard
    · exact Or.inr cleared.2
  | trailing first nonempty => exact Or.inr rfl

/-- Full success consumes the whole input, rather than merely a valid prefix. -/
theorem headerFullSpec_success (old value : HeaderValue) (wire : List w8)
    (spec : HeaderFullSpec old wire (.success value)) :
    old = headerZeroValue ∧ HeaderRecordValid 0 headerZeroValue wire value := by
  cases spec with
  | success first =>
    obtain ⟨zero, item, split, valid⟩ := headerFirstSpec_success old value wire [] first
    simp only [List.append_nil] at split
    subst wire
    exact ⟨zero, valid⟩

theorem headerFirstSpec_diagnostic_bound (old value : HeaderValue) (wire : List w8)
    (offset : Nat) (reason : HeaderRecordFailureReason)
    (spec : HeaderFirstSpec old wire (.failure value (.decode offset reason))) : offset ≤ wire.length := by
  cases spec with
  | rejected failure => exact (headerRecordRejected_bounds _ _ _ _ _ _ _ _ (by decide) failure).2.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullSpec_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_success
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullSpec_cleanup
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullSpec_success
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_diagnostic_bound
