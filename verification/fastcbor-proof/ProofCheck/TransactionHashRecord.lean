import ProofCheck.TransactionHashPairs
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
inductive TransactionHashBodyValid (d : Nat) (old : TransactionHashValue) :
    List w8 → TransactionHashValue → Prop where
  | nullish {head} (hn : head = W8 246 ∨ head = W8 247) :
      TransactionHashBodyValid d old [head] old
  | map {head payload body argument final} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32)
      (valid : TransactionHashPairsValid (d + 1) count [] old body final) :
      TransactionHashBodyValid d old ((head :: payload) ++ body) final

inductive TransactionHashRecordFailure where
  | tagMalformed (reason : ExpectedFailureReason)
  | tagUnsupported
  | opening (reason : FeltFailureReason)
  | pairs (reason : TransactionHashFailure)

/-- A body error retains its last published value and exact parser position.
    Once a map was entered, it remains open on rejection; selected child readers
    and Skip have already restored their own nested depth. -/
inductive TransactionHashBodyRejected (d : Nat) (old : TransactionHashValue) :
    List w8 → TransactionHashValue → Nat → Nat → Bool → TransactionHashRecordFailure → Prop where
  | opening {wire reason} (failure : StructOpenFailure d wire reason) :
      TransactionHashBodyRejected d old wire old 0 0 false (.opening reason)
  | map {head payload body argument final consumed offset reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : TransactionHashPairsRejected (d + 1) count [] old body final consumed offset reason) :
      TransactionHashBodyRejected d old ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) true (.pairs reason)

/-- All-input body classification combines the actual application's opening
    policy with the independent first-key-wins pair interpretation. -/
theorem transactionHashBody_classify (d : Nat) 
    (old : TransactionHashValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ TransactionHashBodyValid d old item final) ∨
    (∃ final consumed offset entered reason, TransactionHashBodyRejected d old wire final consumed offset entered reason) := by
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, old, split, .nullish nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have length : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases transactionHashPairs_classify (d + 1) count [] old body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major length hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, true, _,
        .map enc major length hd countFit failure⟩
  · exact Or.inr ⟨old, 0, 0, false, _, .opening failure⟩

theorem transactionHashBody_nonempty (d : Nat) 
    (old final : TransactionHashValue) (wire : List w8)
    (valid : TransactionHashBodyValid d old wire final) : 0 < wire.length := by
  cases valid <;> simp

inductive TransactionHashRecordResult where
  | success (final : TransactionHashValue) (rest : List w8)
  | failure (retained : TransactionHashValue) (consumed offset : Nat)
      (entered : Bool) (reason : TransactionHashRecordFailure)

/-- Complete record specification, including the projection-only tag prelude.
    Successful results specify the unread suffix; failures retain the last
    published fields until the outer API clears them. For bare tagUnsupported,
    offset is the rejection position; there is no allocated diagnostic object. -/
inductive TransactionHashRecordSpec (d : Nat) 
    (old : TransactionHashValue) (wire : List w8) : TransactionHashRecordResult → Prop where
  | success {tagConsumed tagRest body rest final}
      (tag : ProjectionTagSpec wire tagConsumed tagRest .success)
      (split : tagRest = body ++ rest) (valid : TransactionHashBodyValid d old body final) :
      TransactionHashRecordSpec d old wire (.success final rest)
  | malformed {tagConsumed tagRest reason}
      (tag : ProjectionTagSpec wire tagConsumed tagRest (.malformed reason)) :
      TransactionHashRecordSpec d old wire (.failure old 0 0 false (.tagMalformed reason))
  | unsupported {tagConsumed tagRest} (tag : ProjectionTagSpec wire tagConsumed tagRest .unsupported) :
      TransactionHashRecordSpec d old wire (.failure old tagConsumed tagConsumed false .tagUnsupported)
  | rejected {tagConsumed tagRest final consumed offset entered reason}
      (tag : ProjectionTagSpec wire tagConsumed tagRest .success)
      (failure : TransactionHashBodyRejected d old tagRest final consumed offset entered reason) :
      TransactionHashRecordSpec d old wire
        (.failure final (tagConsumed + consumed) (tagConsumed + offset) entered reason)

/-- Every input has a specified complete transaction hash projection result. This theorem assumes no implementation WP. -/
theorem transactionHashRecord_classify (d : Nat) 
    (old : TransactionHashValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ result, TransactionHashRecordSpec d old wire result := by
  obtain ⟨tagConsumed, tagRest, status, tag⟩ := projectionTag_classify wire
  cases status with
  | success =>
    rcases transactionHashBody_classify d old tagRest depth with
      ⟨body, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
    · exact ⟨_, .success tag split valid⟩
    · exact ⟨_, .rejected tag failure⟩
  | malformed reason => exact ⟨_, .malformed tag⟩
  | unsupported => exact ⟨_, .unsupported tag⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashBody_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashBody_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashRecord_classify
