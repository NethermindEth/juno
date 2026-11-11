import ProofCheck.ReceiptEventsPairs
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
inductive ReceiptEventsBodyValid (d : Nat) (old : ReceiptEventsValue) :
    List w8 → List w8 → ReceiptEventsValue → Prop where
  | nullish {head} (suffix : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      ReceiptEventsBodyValid d old [head] suffix old
  | map {head payload body suffix argument final} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32)
      (valid : ReceiptEventsPairsValid (d + 1) count [] old body suffix final) :
      ReceiptEventsBodyValid d old ((head :: payload) ++ body) suffix final

inductive ReceiptEventsRecordFailure where
  | tagMalformed (reason : ExpectedFailureReason)
  | tagUnsupported
  | opening (reason : FeltFailureReason)
  | pairs (reason : ReceiptEventsFailure)

/-- A body error retains its last published value and exact parser position.
    An entered map remains open; an Events failure can retain further array/map
    levels. Ignored values and hash failures restore their own nested depth. -/
inductive ReceiptEventsBodyRejected (d : Nat) (old : ReceiptEventsValue) :
    List w8 → ReceiptEventsValue → Nat → Nat → Nat → ReceiptEventsRecordFailure → Prop where
  | opening {wire reason} (failure : StructOpenFailure d wire reason) :
      ReceiptEventsBodyRejected d old wire old 0 0 0 (.opening reason)
  | map {head payload body argument final consumed offset extraDepth reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : ReceiptEventsPairsRejected (d + 1) count [] old body final consumed offset extraDepth reason) :
      ReceiptEventsBodyRejected d old ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) (1 + extraDepth) (.pairs reason)

/-- All-input body classification combines the actual application's opening
    policy with the independent first-key-wins pair interpretation. -/
theorem receiptEventsBody_classify (d : Nat)
    (old : ReceiptEventsValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptEventsBodyValid d old item rest final) ∨
    (∃ final consumed offset extraDepth reason, ReceiptEventsBodyRejected d old wire final consumed offset extraDepth reason) := by
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, old, split, .nullish tail nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have length : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases receiptEventsPairs_classify (d + 1) count [] old body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major length hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, 1 + extraDepth, _,
        .map enc major length hd countFit failure⟩
  · exact Or.inr ⟨old, 0, 0, 0, _, .opening failure⟩

theorem receiptEventsBody_nonempty (d : Nat)
    (old final : ReceiptEventsValue) (wire rest : List w8)
    (valid : ReceiptEventsBodyValid d old wire rest final) : 0 < wire.length := by
  cases valid <;> simp

inductive ReceiptEventsRecordResult where
  | success (final : ReceiptEventsValue) (rest : List w8)
  | failure (retained : ReceiptEventsValue) (consumed offset : Nat)
      (extraDepth : Nat) (reason : ReceiptEventsRecordFailure)

/-- Complete record specification, including the projection-only tag prelude.
    Successful results specify the unread suffix; failures retain the last
    published fields until the outer API clears them. For bare tagUnsupported,
    offset is the rejection position; there is no allocated diagnostic object. -/
inductive ReceiptEventsRecordSpec (d : Nat)
    (old : ReceiptEventsValue) (wire : List w8) : ReceiptEventsRecordResult → Prop where
  | success {tagConsumed tagRest body rest final}
      (tag : ProjectionTagSpec wire tagConsumed tagRest .success)
      (split : tagRest = body ++ rest) (valid : ReceiptEventsBodyValid d old body rest final) :
      ReceiptEventsRecordSpec d old wire (.success final rest)
  | malformed {tagConsumed tagRest reason}
      (tag : ProjectionTagSpec wire tagConsumed tagRest (.malformed reason)) :
      ReceiptEventsRecordSpec d old wire (.failure old 0 0 0 (.tagMalformed reason))
  | unsupported {tagConsumed tagRest} (tag : ProjectionTagSpec wire tagConsumed tagRest .unsupported) :
      ReceiptEventsRecordSpec d old wire (.failure old tagConsumed tagConsumed 0 .tagUnsupported)
  | rejected {tagConsumed tagRest final consumed offset extraDepth reason}
      (tag : ProjectionTagSpec wire tagConsumed tagRest .success)
      (failure : ReceiptEventsBodyRejected d old tagRest final consumed offset extraDepth reason) :
      ReceiptEventsRecordSpec d old wire
        (.failure final (tagConsumed + consumed) (tagConsumed + offset) extraDepth reason)

/-- Every input has a specified receipt events projection result.
    This theorem assumes no implementation WP. -/
theorem receiptEventsRecord_classify (d : Nat)
    (old : ReceiptEventsValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ result, ReceiptEventsRecordSpec d old wire result := by
  obtain ⟨tagConsumed, tagRest, status, tag⟩ := projectionTag_classify wire
  cases status with
  | success =>
    rcases receiptEventsBody_classify d old tagRest depth with
      ⟨body, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨_, .success tag split valid⟩
    · exact ⟨_, .rejected tag failure⟩
  | malformed reason => exact ⟨_, .malformed tag⟩
  | unsupported => exact ⟨_, .unsupported tag⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsBody_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsBody_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsRecord_classify
