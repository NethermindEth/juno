import ProofCheck.HeaderProjectionBodyUnique
import ProofCheck.ProjectionTagUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The complete independent record policy determines exactly one result:
    success includes the decoded value and suffix; rejection includes retained
    fields, consumption, diagnostic offset, entered-map state and reason. -/
theorem headerProjectionRecord_unique (d : Nat) (kind : HeaderProjectionKind)
    (old : HeaderProjectionValue) (wire : List w8) (a b : HeaderProjectionRecordResult)
    (first : HeaderProjectionRecordSpec d kind old wire a)
    (second : HeaderProjectionRecordSpec d kind old wire b) : a = b := by
  cases first with
  | success tag split valid =>
    cases second with
    | success tag2 split2 valid2 =>
      obtain ⟨_, restEq, _⟩ := projectionTag_unique _ _ _ _ _ _ _ tag tag2
      have same := split.symm.trans (restEq.trans split2)
      obtain ⟨_, rfl, rfl⟩ := headerProjectionBody_prefix_unique _ _ _ _ _ _ _ _ _ valid valid2 same
      rfl
    | malformed tag2 =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
    | unsupported tag2 =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
    | rejected tag2 failure =>
      obtain ⟨_, restEq, _⟩ := projectionTag_unique _ _ _ _ _ _ _ tag tag2
      rw [← restEq, split] at failure
      exact False.elim (headerProjectionBody_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid failure)
  | malformed tag =>
    cases second with
    | success tag2 split valid =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
    | malformed tag2 =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
      rfl
    | unsupported tag2 =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
    | rejected tag2 failure =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
  | unsupported tag =>
    cases second with
    | success tag2 split valid =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
    | malformed tag2 =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
    | unsupported tag2 =>
      obtain ⟨rfl, _, _⟩ := projectionTag_unique _ _ _ _ _ _ _ tag tag2
      rfl
    | rejected tag2 failure =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
  | rejected tag failure =>
    cases second with
    | success tag2 split valid =>
      obtain ⟨_, restEq, _⟩ := projectionTag_unique _ _ _ _ _ _ _ tag tag2
      rw [restEq, split] at failure
      exact False.elim (headerProjectionBody_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid failure)
    | malformed tag2 =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
    | unsupported tag2 =>
      have status := (projectionTag_unique _ _ _ _ _ _ _ tag tag2).2.2
      cases status
    | rejected tag2 failure2 =>
      obtain ⟨rfl, restEq, _⟩ := projectionTag_unique _ _ _ _ _ _ _ tag tag2
      rw [← restEq] at failure2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := headerProjectionBodyRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      rfl

/-- At supported nesting depth, every byte input has exactly one specified
    result for each of the five Header projections. This is a specification
    theorem, not a claim about execution termination or the generated loop. -/
theorem headerProjectionRecord_total_unique (d : Nat) (kind : HeaderProjectionKind)
    (old : HeaderProjectionValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ result, HeaderProjectionRecordSpec d kind old wire result ∧
      ∀ other, HeaderProjectionRecordSpec d kind old wire other → other = result := by
  obtain ⟨result, spec⟩ := headerProjectionRecord_classify d kind old wire depth
  exact ⟨result, spec, fun other otherSpec => headerProjectionRecord_unique d kind old wire other result otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRecord_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRecord_total_unique
