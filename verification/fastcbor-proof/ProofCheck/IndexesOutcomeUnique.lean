import ProofCheck.IndexesFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- First failures determine both the final destination and the complete public
    diagnostic. This is a specification theorem, not a production WP. -/
theorem indexesFirstSpec_failure_unique (old a b : IndexesValue) (wire : List w8)
    (errorA errorB : IndexesAPIError) (bound : wire.length < 2^64)
    (first : IndexesFirstSpec old wire (.failure a errorA))
    (second : IndexesFirstSpec old wire (.failure b errorB)) : a = b ∧ errorA = errorB := by
  cases first with
  | nonzero old wire nonzero =>
    cases second with
    | nonzero _ _ _ => exact ⟨rfl, rfl⟩
    | rejected failure => exact False.elim (nonzero rfl)
  | rejected failure =>
    cases second with
    | nonzero _ _ nonzero => exact False.elim (nonzero rfl)
    | rejected failure2 =>
      have result := indexesRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ bound failure failure2
      exact ⟨rfl, by rw [result.2.2.1, result.2.2.2.2]⟩

/-- Full failures also determine whether the error comes from First or from
    trailing data, including the exact trailing-byte count. -/
theorem indexesFullSpec_failure_unique (old a b : IndexesValue) (wire : List w8)
    (errorA errorB : IndexesAPIError) (bound : wire.length < 2^64)
    (first : IndexesFullSpec old wire (.failure a errorA))
    (second : IndexesFullSpec old wire (.failure b errorB)) : a = b ∧ errorA = errorB := by
  cases first with
  | firstError first =>
    cases second with
    | firstError second => exact indexesFirstSpec_failure_unique _ _ _ _ _ _ bound first second
    | trailing second nonempty =>
      exact False.elim (indexesFirstSpec_success_not_failure _ _ _ _ _ _ bound second first)
  | trailing first nonempty =>
    cases second with
    | firstError second =>
      exact False.elim (indexesFirstSpec_success_not_failure _ _ _ _ _ _ bound first second)
    | trailing second nonempty2 =>
      have suffix := (indexesFirstSpec_success_unique _ _ _ _ _ _ first second).2
      exact ⟨rfl, by rw [suffix]⟩

/-- Every representable input and initial destination admits at most one First
    outcome: value and suffix on success, or destination and diagnostic on error. -/
theorem indexesFirstSpec_unique (old : IndexesValue) (wire : List w8)
    (resultA resultB : IndexesFirstResult) (bound : wire.length < 2^64)
    (first : IndexesFirstSpec old wire resultA) (second : IndexesFirstSpec old wire resultB) :
    resultA = resultB := by
  cases resultA with
  | success a restA =>
    cases resultB with
    | success b restB =>
      obtain ⟨rfl, rfl⟩ := indexesFirstSpec_success_unique _ _ _ _ _ _ first second
      rfl
    | failure b errorB =>
      exact False.elim (indexesFirstSpec_success_not_failure _ _ _ _ _ _ bound first second)
  | failure a errorA =>
    cases resultB with
    | success b restB =>
      exact False.elim (indexesFirstSpec_success_not_failure _ _ _ _ _ _ bound second first)
    | failure b errorB =>
      obtain ⟨rfl, rfl⟩ := indexesFirstSpec_failure_unique _ _ _ _ _ _ bound first second
      rfl

/-- The complete full-decoding specification is deterministic on representable
    input lengths, including destination guards, parse errors and cleanup. -/
theorem indexesFullSpec_unique (old : IndexesValue) (wire : List w8)
    (resultA resultB : IndexesDecodeResult) (bound : wire.length < 2^64)
    (first : IndexesFullSpec old wire resultA) (second : IndexesFullSpec old wire resultB) :
    resultA = resultB := by
  cases resultA with
  | success a =>
    cases resultB with
    | success b =>
      have values := indexesFullSpec_success_unique _ _ _ _ first second
      subst b
      rfl
    | failure b errorB =>
      exact False.elim (indexesFullSpec_success_not_failure _ _ _ _ _ bound first second)
  | failure a errorA =>
    cases resultB with
    | success b =>
      exact False.elim (indexesFullSpec_success_not_failure _ _ _ _ _ bound second first)
    | failure b errorB =>
      obtain ⟨rfl, rfl⟩ := indexesFullSpec_failure_unique _ _ _ _ _ _ bound first second
      rfl

/-- Classification and uniqueness together give one specified First result.
    This does not establish execution or termination of the Go implementation. -/
theorem indexesFirstSpec_total_unique (old : IndexesValue) (wire : List w8)
    (bound : wire.length < 2^64) :
    ∃ result, IndexesFirstSpec old wire result ∧
      ∀ other, IndexesFirstSpec old wire other → other = result := by
  obtain ⟨result, spec⟩ := indexesFirstSpec_classify old wire
  exact ⟨result, spec, fun other otherSpec => indexesFirstSpec_unique old wire other result bound otherSpec spec⟩

/-- The independent full API contract assigns exactly one result to every
    representable byte input and initial Indexes. -/
theorem indexesFullSpec_total_unique (old : IndexesValue) (wire : List w8)
    (bound : wire.length < 2^64) :
    ∃ result, IndexesFullSpec old wire result ∧
      ∀ other, IndexesFullSpec old wire other → other = result := by
  obtain ⟨result, spec⟩ := indexesFullSpec_classify old wire
  exact ⟨result, spec, fun other otherSpec => indexesFullSpec_unique old wire other result bound otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_failure_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFullSpec_failure_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFullSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFullSpec_total_unique
