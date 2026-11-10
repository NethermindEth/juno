import ProofCheck.HeaderFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- First failures determine both the final destination and the complete public
    diagnostic. This is a specification theorem, not a production WP. -/
theorem headerFirstSpec_failure_unique (old a b : HeaderValue) (wire : List w8)
    (errorA errorB : HeaderAPIError) (bound : wire.length < 2^64)
    (first : HeaderFirstSpec old wire (.failure a errorA))
    (second : HeaderFirstSpec old wire (.failure b errorB)) : a = b ∧ errorA = errorB := by
  cases first with
  | nonzero old wire nonzero =>
    cases second with
    | nonzero _ _ _ => exact ⟨rfl, rfl⟩
    | rejected failure => exact False.elim (nonzero rfl)
  | rejected failure =>
    cases second with
    | nonzero _ _ nonzero => exact False.elim (nonzero rfl)
    | rejected failure2 =>
      have result := headerRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ bound failure failure2
      exact ⟨rfl, by rw [result.2.2.1, result.2.2.2.2]⟩

/-- Full failures also determine whether the error comes from First or from
    trailing data, including the exact trailing-byte count. -/
theorem headerFullSpec_failure_unique (old a b : HeaderValue) (wire : List w8)
    (errorA errorB : HeaderAPIError) (bound : wire.length < 2^64)
    (first : HeaderFullSpec old wire (.failure a errorA))
    (second : HeaderFullSpec old wire (.failure b errorB)) : a = b ∧ errorA = errorB := by
  cases first with
  | firstError first =>
    cases second with
    | firstError second => exact headerFirstSpec_failure_unique _ _ _ _ _ _ bound first second
    | trailing second nonempty =>
      exact False.elim (headerFirstSpec_success_not_failure _ _ _ _ _ _ bound second first)
  | trailing first nonempty =>
    cases second with
    | firstError second =>
      exact False.elim (headerFirstSpec_success_not_failure _ _ _ _ _ _ bound first second)
    | trailing second nonempty2 =>
      have suffix := (headerFirstSpec_success_unique _ _ _ _ _ _ first second).2
      exact ⟨rfl, by rw [suffix]⟩

/-- Every representable input and initial destination admits at most one First
    outcome: value and suffix on success, or destination and diagnostic on error. -/
theorem headerFirstSpec_unique (old : HeaderValue) (wire : List w8)
    (resultA resultB : HeaderFirstResult) (bound : wire.length < 2^64)
    (first : HeaderFirstSpec old wire resultA) (second : HeaderFirstSpec old wire resultB) :
    resultA = resultB := by
  cases resultA with
  | success a restA =>
    cases resultB with
    | success b restB =>
      obtain ⟨rfl, rfl⟩ := headerFirstSpec_success_unique _ _ _ _ _ _ first second
      rfl
    | failure b errorB =>
      exact False.elim (headerFirstSpec_success_not_failure _ _ _ _ _ _ bound first second)
  | failure a errorA =>
    cases resultB with
    | success b restB =>
      exact False.elim (headerFirstSpec_success_not_failure _ _ _ _ _ _ bound second first)
    | failure b errorB =>
      obtain ⟨rfl, rfl⟩ := headerFirstSpec_failure_unique _ _ _ _ _ _ bound first second
      rfl

/-- The complete full-decoding specification is deterministic on representable
    input lengths, including destination guards, parse errors and cleanup. -/
theorem headerFullSpec_unique (old : HeaderValue) (wire : List w8)
    (resultA resultB : HeaderDecodeResult) (bound : wire.length < 2^64)
    (first : HeaderFullSpec old wire resultA) (second : HeaderFullSpec old wire resultB) :
    resultA = resultB := by
  cases resultA with
  | success a =>
    cases resultB with
    | success b =>
      have values := headerFullSpec_success_unique _ _ _ _ first second
      subst b
      rfl
    | failure b errorB =>
      exact False.elim (headerFullSpec_success_not_failure _ _ _ _ _ bound first second)
  | failure a errorA =>
    cases resultB with
    | success b =>
      exact False.elim (headerFullSpec_success_not_failure _ _ _ _ _ bound second first)
    | failure b errorB =>
      obtain ⟨rfl, rfl⟩ := headerFullSpec_failure_unique _ _ _ _ _ _ bound first second
      rfl

/-- Classification and uniqueness together give one specified First result.
    This does not establish execution or termination of the Go implementation. -/
theorem headerFirstSpec_total_unique (old : HeaderValue) (wire : List w8)
    (bound : wire.length < 2^64) :
    ∃ result, HeaderFirstSpec old wire result ∧
      ∀ other, HeaderFirstSpec old wire other → other = result := by
  obtain ⟨result, spec⟩ := headerFirstSpec_classify old wire
  exact ⟨result, spec, fun other otherSpec => headerFirstSpec_unique old wire other result bound otherSpec spec⟩

/-- The independent full API contract assigns exactly one result to every
    representable byte input and initial Header. -/
theorem headerFullSpec_total_unique (old : HeaderValue) (wire : List w8)
    (bound : wire.length < 2^64) :
    ∃ result, HeaderFullSpec old wire result ∧
      ∀ other, HeaderFullSpec old wire other → other = result := by
  obtain ⟨result, spec⟩ := headerFullSpec_classify old wire
  exact ⟨result, spec, fun other otherSpec => headerFullSpec_unique old wire other result bound otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_failure_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullSpec_failure_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullSpec_total_unique
