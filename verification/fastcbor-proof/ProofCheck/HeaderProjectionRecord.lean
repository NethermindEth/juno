import ProofCheck.HeaderProjectionPairs
import ProofCheck.ProjectionTagSpec
import ProofCheck.StructOpenSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Successful map/body decoding, before adding the optional projection tag. -/
inductive HeaderProjectionBodyValid (d : Nat) (kind : HeaderProjectionKind) (old : HeaderProjectionValue) :
    List w8 → HeaderProjectionValue → Prop where
  | nullish {head} (hn : head = W8 246 ∨ head = W8 247) :
      HeaderProjectionBodyValid d kind old [head] old
  | map {head payload body argument final} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32)
      (valid : HeaderProjectionPairsValid (d + 1) kind count [] old body final) :
      HeaderProjectionBodyValid d kind old ((head :: payload) ++ body) final

inductive HeaderProjectionRecordFailure where
  | tagMalformed (reason : ExpectedFailureReason)
  | tagUnsupported
  | opening (reason : FeltFailureReason)
  | pairs (reason : HeaderProjectionFailure)

/-- A body error retains its last published value and exact parser position.
    Once a map was entered, it remains open on rejection; selected child readers
    and Skip have already restored their own nested depth. -/
inductive HeaderProjectionBodyRejected (d : Nat) (kind : HeaderProjectionKind) (old : HeaderProjectionValue) :
    List w8 → HeaderProjectionValue → Nat → Nat → Bool → HeaderProjectionRecordFailure → Prop where
  | opening {wire reason} (failure : StructOpenFailure d wire reason) :
      HeaderProjectionBodyRejected d kind old wire old 0 0 false (.opening reason)
  | map {head payload body argument final consumed offset reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : HeaderProjectionPairsRejected (d + 1) kind count [] old body final consumed offset reason) :
      HeaderProjectionBodyRejected d kind old ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) true (.pairs reason)

/-- All-input body classification combines the actual application's opening
    policy with the independent first-key-wins pair interpretation. -/
theorem headerProjectionBody_classify (d : Nat) (kind : HeaderProjectionKind)
    (old : HeaderProjectionValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ HeaderProjectionBodyValid d kind old item final) ∨
    (∃ final consumed offset entered reason, HeaderProjectionBodyRejected d kind old wire final consumed offset entered reason) := by
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, old, split, .nullish nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have length : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases headerProjectionPairs_classify (d + 1) count kind [] old body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major length hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, true, _,
        .map enc major length hd countFit failure⟩
  · exact Or.inr ⟨old, 0, 0, false, _, .opening failure⟩

theorem headerProjectionBody_nonempty (d : Nat) (kind : HeaderProjectionKind)
    (old final : HeaderProjectionValue) (wire : List w8)
    (valid : HeaderProjectionBodyValid d kind old wire final) : 0 < wire.length := by
  cases valid <;> simp

inductive HeaderProjectionRecordResult where
  | success (final : HeaderProjectionValue) (rest : List w8)
  | failure (retained : HeaderProjectionValue) (consumed offset : Nat)
      (entered : Bool) (reason : HeaderProjectionRecordFailure)

/-- Complete record specification, including the projection-only tag prelude.
    Successful results specify the unread suffix; failures retain the last
    published fields until the outer API clears them. For bare tagUnsupported,
    offset is the rejection position; there is no allocated diagnostic object. -/
inductive HeaderProjectionRecordSpec (d : Nat) (kind : HeaderProjectionKind)
    (old : HeaderProjectionValue) (wire : List w8) : HeaderProjectionRecordResult → Prop where
  | success {tagConsumed tagRest body rest final}
      (tag : ProjectionTagSpec wire tagConsumed tagRest .success)
      (split : tagRest = body ++ rest) (valid : HeaderProjectionBodyValid d kind old body final) :
      HeaderProjectionRecordSpec d kind old wire (.success final rest)
  | malformed {tagConsumed tagRest reason}
      (tag : ProjectionTagSpec wire tagConsumed tagRest (.malformed reason)) :
      HeaderProjectionRecordSpec d kind old wire (.failure old 0 0 false (.tagMalformed reason))
  | unsupported {tagConsumed tagRest} (tag : ProjectionTagSpec wire tagConsumed tagRest .unsupported) :
      HeaderProjectionRecordSpec d kind old wire (.failure old tagConsumed tagConsumed false .tagUnsupported)
  | rejected {tagConsumed tagRest final consumed offset entered reason}
      (tag : ProjectionTagSpec wire tagConsumed tagRest .success)
      (failure : HeaderProjectionBodyRejected d kind old tagRest final consumed offset entered reason) :
      HeaderProjectionRecordSpec d kind old wire
        (.failure final (tagConsumed + consumed) (tagConsumed + offset) entered reason)

/-- Every input has a specified complete record result for each of the five
    Header projection shapes. This theorem assumes no implementation WP. -/
theorem headerProjectionRecord_classify (d : Nat) (kind : HeaderProjectionKind)
    (old : HeaderProjectionValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ result, HeaderProjectionRecordSpec d kind old wire result := by
  obtain ⟨tagConsumed, tagRest, status, tag⟩ := projectionTag_classify wire
  cases status with
  | success =>
    rcases headerProjectionBody_classify d kind old tagRest depth with
      ⟨body, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
    · exact ⟨_, .success tag split valid⟩
    · exact ⟨_, .rejected tag failure⟩
  | malformed reason => exact ⟨_, .malformed tag⟩
  | unsupported => exact ⟨_, .unsupported tag⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionBody_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionBody_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRecord_classify
