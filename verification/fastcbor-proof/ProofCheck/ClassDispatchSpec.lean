import ProofCheck.ClassPayloadSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ClassDispatchResult where
  | nullish (rest : List w8)
  | tagFailure (reason : ExpectedFailureReason)
  | unknown (tag : w64) (consumed : Nat) (rest : List w8)
  | payload (tagConsumed : Nat) (result : ClassPayloadResult)

/-- Untagged null/undefined clears the interface. Every other input requires a
    complete tag; recognized tags decode a fresh zero record without incrementing
    cursor depth for the tag itself. The record result preserves exact failure
    state, and unknown tags consume only their tag bytes. -/
inductive ClassDispatchSpec (d : Nat) (tags : CBORClassTags) : List w8 → ClassDispatchResult → Prop where
  | nullish (head : w8) (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      ClassDispatchSpec d tags (head :: rest) (.nullish rest)
  | tagFailure {wire reason} (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 192) wire reason) :
      ClassDispatchSpec d tags wire (.tagFailure reason)
  | unknown {head payload rest tag} (enc : HeadArgument head payload tag)
      (major : head &&& W8 224 = W8 192) (unknown : ClassTagMeaning tags tag none) :
      ClassDispatchSpec d tags ((head :: payload) ++ rest) (.unknown tag (1 + payload.length) rest)
  | payload {head payload body result}
      (enc : HeadArgument head payload (classVariantTag tags (classPayloadVariant result)))
      (major : head &&& W8 224 = W8 192) (valid : ClassPayloadSpec d body result) :
      ClassDispatchSpec d tags ((head :: payload) ++ body) (.payload (1 + payload.length) result)

/-- All inputs are classified without assuming any production decoder helper.
    Distinct profiles are needed for uniqueness, not existence. -/
theorem classDispatch_classify (d : Nat) (tags : CBORClassTags) (wire : List w8) (depth : d ≤ 32) :
    ∃ result, ClassDispatchSpec d tags wire result := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact ⟨.nullish tail, .nullish head tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases expected_classify (W8 192) wire with
    ⟨head, payload, body, tag, split, enc, major⟩ | ⟨reason, failure⟩
  · subst wire
    obtain ⟨choice, meaning⟩ := classTagMeaning_total tags tag
    cases choice with
    | none => exact ⟨.unknown tag (1 + payload.length) body, .unknown enc major meaning⟩
    | some variant =>
      obtain ⟨result, selected, spec⟩ := classPayload_classify d body variant depth
      have encoded : HeadArgument head payload (classVariantTag tags (classPayloadVariant result)) := by
        rw [selected, ← meaning]
        exact enc
      exact ⟨.payload (1 + payload.length) result, .payload encoded major spec⟩
  · exact ⟨.tagFailure reason, .tagFailure hn hu failure⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classDispatch_classify
