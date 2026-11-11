import ProofCheck.ClassDispatchSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A distinct tag profile determines one complete dispatch result, including
    the tag width, unknown-tag suffix, payload variant and native record outcome. -/
theorem classDispatchSpec_unique (d : Nat) (tags : CBORClassTags) (wire : List w8)
    (distinct : ClassTagsDistinct tags) (a b : ClassDispatchResult)
    (first : ClassDispatchSpec d tags wire a) (second : ClassDispatchSpec d tags wire b) : a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | nullish head rest hn =>
    cases second with
    | nullish head2 rest2 hn2 =>
      obtain ⟨rfl, rfl⟩ := List.cons.inj same
      rfl
    | tagFailure hn2 hu2 failure2 =>
      rw [← same] at hn2 hu2
      rcases hn with rfl | rfl
      · exact False.elim (hn2 rfl)
      · exact False.elim (hu2 rfl)
    | unknown enc2 major2 unknown2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | payload enc2 major2 valid2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | tagFailure hn hu failure =>
    cases second with
    | nullish head2 rest2 hn2 =>
      rw [same] at hn hu
      rcases hn2 with rfl | rfl
      · exact False.elim (hn rfl)
      · exact False.elim (hu rfl)
    | tagFailure hn2 hu2 failure2 =>
      rw [same] at failure
      exact congrArg ClassDispatchResult.tagFailure (expectedFailure_reason_unique _ _ _ _ failure failure2)
    | unknown enc2 major2 unknown2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc2 major2 same failure)
    | payload enc2 major2 valid2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc2 major2 same failure)
  | @unknown head payload rest tag enc major unknown =>
    cases second with
    | nullish head2 rest2 hn2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn2 with rfl | rfl <;> contradiction
    | tagFailure hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major same.symm failure2)
    | unknown enc2 major2 unknown2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      rfl
    | @payload head2 payload2 body2 result2 enc2 major2 valid2 =>
      obtain ⟨_, _, tagEq, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      exact False.elim (unknown (classPayloadVariant result2) tagEq)
  | @payload head payload body result enc major valid =>
    cases second with
    | nullish head2 rest2 hn2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn2 with rfl | rfl <;> contradiction
    | tagFailure hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major same.symm failure2)
    | unknown enc2 major2 unknown2 =>
      obtain ⟨_, _, tagEq, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      exact False.elim (unknown2 (classPayloadVariant result) tagEq.symm)
    | @payload head2 payload2 body2 result2 enc2 major2 valid2 =>
      obtain ⟨rfl, rfl, tagEq, rfl⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      have variantEq := classVariantTag_injective tags distinct _ _ tagEq
      obtain rfl := classPayloadSpec_unique d body result result2 variantEq valid valid2
      rfl

theorem classDispatchSpec_total_unique (d : Nat) (tags : CBORClassTags) (wire : List w8)
    (depth : d ≤ 32) (distinct : ClassTagsDistinct tags) :
    ∃ result, ClassDispatchSpec d tags wire result ∧
      ∀ other, ClassDispatchSpec d tags wire other → other = result := by
  obtain ⟨result, spec⟩ := classDispatch_classify d tags wire depth
  exact ⟨result, spec, fun other otherSpec => classDispatchSpec_unique d tags wire distinct other result otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classDispatchSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.classDispatchSpec_total_unique
