import ProofCheck.ProjectionTagSpec
import ProofCheck.SkipDisjoint

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The tag policy fixes consumption, suffix and status, including the exact
    malformed-head reason. In particular, success and rejection cannot overlap. -/
theorem projectionTag_unique (wire restA restB : List w8) (consumedA consumedB : Nat)
    (a b : ProjectionTagStatus) (first : ProjectionTagSpec wire consumedA restA a)
    (second : ProjectionTagSpec wire consumedB restB b) :
    consumedA = consumedB ∧ restA = restB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | untagged plain =>
    cases second with
    | untagged => exact ⟨rfl, same, rfl⟩
    | @allowed head payload rest tag enc major allowed =>
      exact False.elim (plain ⟨head, payload ++ _, same, major⟩)
    | malformed tagged failure =>
      rw [← same] at tagged
      exact False.elim (plain tagged)
    | @unsupported head payload rest tag enc major rejected =>
      exact False.elim (plain ⟨head, payload ++ _, same, major⟩)
  | @allowed head payload rest tag enc major allowed =>
    cases second with
    | untagged plain =>
      exact False.elim (plain ⟨head, payload ++ _, same.symm, major⟩)
    | @allowed head2 payload2 rest2 tag2 enc2 major2 allowed2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique head head2 payload payload2 _ _ tag tag2 enc enc2 same
      exact ⟨rfl, rfl, rfl⟩
    | malformed tagged failure =>
      exact False.elim (expectedFailure_excludes_encoded (W8 192) head payload _ _ tag _ enc major same.symm failure)
    | @unsupported head2 payload2 rest2 tag2 enc2 major2 rejected2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique head head2 payload payload2 _ _ tag tag2 enc enc2 same
      exact False.elim (allowed rejected2)
  | @malformed wire reason tagged failure =>
    cases second with
    | untagged plain =>
      rw [← same] at plain
      exact False.elim (plain tagged)
    | @allowed head payload rest tag enc major allowed =>
      exact False.elim (expectedFailure_excludes_encoded (W8 192) head payload _ wire tag _ enc major same failure)
    | @malformed wire2 reason2 tagged2 failure2 =>
      rw [← same] at failure2
      have hr := expectedFailure_reason_unique (W8 192) wire reason reason2 failure failure2
      exact ⟨rfl, same, congrArg ProjectionTagStatus.malformed hr⟩
    | @unsupported head payload rest tag enc major rejected =>
      exact False.elim (expectedFailure_excludes_encoded (W8 192) head payload _ wire tag _ enc major same failure)
  | @unsupported head payload rest tag enc major rejected =>
    cases second with
    | untagged plain =>
      exact False.elim (plain ⟨head, payload ++ _, same.symm, major⟩)
    | @allowed head2 payload2 rest2 tag2 enc2 major2 allowed2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique head head2 payload payload2 _ _ tag tag2 enc enc2 same
      exact False.elim (allowed2 rejected)
    | malformed tagged failure =>
      exact False.elim (expectedFailure_excludes_encoded (W8 192) head payload _ _ tag _ enc major same.symm failure)
    | @unsupported head2 payload2 rest2 tag2 enc2 major2 rejected2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique head head2 payload payload2 _ _ tag tag2 enc enc2 same
      exact ⟨rfl, rfl, rfl⟩

/-- Every byte sequence has exactly one tag-prelude outcome. This is pure
    specification totality, independent of execution termination. -/
theorem projectionTag_total_unique (wire : List w8) :
    ∃ consumed rest status, ProjectionTagSpec wire consumed rest status ∧
      ∀ otherConsumed otherRest otherStatus, ProjectionTagSpec wire otherConsumed otherRest otherStatus →
        otherConsumed = consumed ∧ otherRest = rest ∧ otherStatus = status := by
  obtain ⟨consumed, rest, status, spec⟩ := projectionTag_classify wire
  exact ⟨consumed, rest, status, spec, fun _ _ _ other => projectionTag_unique wire _ rest _ consumed _ status other spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.projectionTag_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.projectionTag_total_unique
