import ProofCheck.ReceiptStatusValues
import ProofCheck.WantedSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def receiptStatusNextSeen (seen : List Nat) (choice : Option Nat) : List Nat :=
  match wantedSelect seen choice with
  | none => seen
  | some index => index :: seen

inductive ReceiptStatusItemValid (d : Nat)
    (seen : List Nat) (choice : Option Nat) (old : ReceiptStatusValue) : List w8 → ReceiptStatusValue → Prop where
  | ignored {wire} (ignored : wantedSelect seen choice = none) (valid : SkipValid d wire) :
      ReceiptStatusItemValid d seen choice old wire old
  | selected {index field wire final} (fresh : wantedSelect seen choice = some index)
      (lookup : receiptStatusFields[index]? = some field)
      (valid : ReceiptStatusFieldValid field old wire final) :
      ReceiptStatusItemValid d seen choice old wire final

inductive ReceiptStatusFailure where
  | key (reason : KeyFailureReason)
  | ignored (reason : SkipFailureReason)
  | field (reason : ReceiptStatusFieldFailure)

inductive ReceiptStatusItemRejected (d : Nat)
    (seen : List Nat) (choice : Option Nat) : List w8 → Nat → ReceiptStatusFailure → Prop where
  | ignored {wire offset reason} (ignored : wantedSelect seen choice = none) (failure : SkipFailure d wire offset reason) :
      ReceiptStatusItemRejected d seen choice wire offset (.ignored reason)
  | selected {index field wire offset reason} (fresh : wantedSelect seen choice = some index)
      (lookup : receiptStatusFields[index]? = some field)
      (failure : ReceiptStatusFieldRejected field wire offset reason) :
      ReceiptStatusItemRejected d seen choice wire offset (.field reason)

theorem receiptStatusItem_classify (d : Nat) (seen : List Nat)
    (choice : Option Nat) (old : ReceiptStatusValue) (key wire : List w8)
    (keyValid : WantedKeyValid receiptStatusNames key choice) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptStatusItemValid d seen choice old item final) ∨
    (∃ offset reason, ReceiptStatusItemRejected d seen choice wire offset reason) := by
  cases selected : wantedSelect seen choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored selected valid⟩
    · exact Or.inr ⟨offset, _, .ignored selected failure⟩
  | some index =>
    have bound := (wantedValid_selected d 1 index 0 receiptStatusNames seen key (.selected keyValid selected)).1
    obtain ⟨field, lookup⟩ := receiptStatusField_exists index bound
    rcases receiptStatusField_classify field old wire with
      ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected selected lookup valid⟩
    · exact Or.inr ⟨offset, _, .selected selected lookup failure⟩

theorem receiptStatusItem_nonempty (d : Nat) (seen : List Nat)
    (choice : Option Nat) (old final : ReceiptStatusValue) (wire : List w8)
    (valid : ReceiptStatusItemValid d seen choice old wire final) : 0 < wire.length := by
  cases valid with
  | ignored ignored valid => exact skipValid_nonempty d wire valid
  | selected fresh lookup valid => exact receiptStatusField_nonempty _ _ _ _ valid

/-- Independent wire-order interpretation: consume exactly n pairs, skip unknown
    and already-seen fields, and publish only successful first occurrences. -/
inductive ReceiptStatusPairsValid (d : Nat) :
    Nat → List Nat → ReceiptStatusValue → List w8 → ReceiptStatusValue → Prop where
  | nil (seen : List Nat) (old : ReceiptStatusValue) : ReceiptStatusPairsValid d 0 seen old [] old
  | step {n seen choice old middle final key value tail}
      (keyValid : WantedKeyValid receiptStatusNames key choice)
      (valueValid : ReceiptStatusItemValid d seen choice old value middle)
      (rest : ReceiptStatusPairsValid d n (receiptStatusNextSeen seen choice) middle tail final) :
      ReceiptStatusPairsValid d (n + 1) seen old (key ++ value ++ tail) final

/-- Rejection retains earlier published fields. Both Skip and the selected field
    helpers roll back to the value start; their diagnostic may point deeper. -/
inductive ReceiptStatusPairsRejected (d : Nat) :
    Nat → List Nat → ReceiptStatusValue → List w8 → ReceiptStatusValue → Nat → Nat → ReceiptStatusFailure → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      ReceiptStatusPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | value {n seen choice old key tail offset reason}
      (keyValid : WantedKeyValid receiptStatusNames key choice)
      (failure : ReceiptStatusItemRejected d seen choice tail offset reason) :
      ReceiptStatusPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) reason
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : WantedKeyValid receiptStatusNames key choice)
      (valueValid : ReceiptStatusItemValid d seen choice old value middle)
      (failure : ReceiptStatusPairsRejected d n (receiptStatusNextSeen seen choice) middle tail final consumed offset reason) :
      ReceiptStatusPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem receiptStatusPairs_classify (d n : Nat) (seen : List Nat)
    (old : ReceiptStatusValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptStatusPairsValid d n seen old item final) ∨
    (∃ final consumed offset reason, ReceiptStatusPairsRejected d n seen old wire final consumed offset reason) := by
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old⟩
  | succ n ih =>
    rcases schemaKey_classify (WantedKeyMeaning receiptStatusNames)
      (wantedKeyMeaning_total receiptStatusNames) wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      rcases receiptStatusItem_classify d seen choice old key tail keyValid depth with
        ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
      · subst tail
        rcases ih (receiptStatusNextSeen seen choice) middle rest with
          ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
        · subst rest
          exact Or.inl ⟨key ++ value ++ items, suffix, final,
            by simp only [List.append_assoc], .step keyValid valueValid validRest⟩
        · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
          simpa only [List.append_assoc] using ReceiptStatusPairsRejected.next keyValid valueValid failure
      · exact Or.inr ⟨old, key.length, key.length + offset, reason, .value keyValid failure⟩
    · exact Or.inr ⟨old, 0, 0, .key reason, .key failure⟩

theorem receiptStatusPairs_count_bound (d n : Nat) (seen : List Nat)
    (old final : ReceiptStatusValue) (wire : List w8)
    (valid : ReceiptStatusPairsValid d n seen old wire final) : 2 * n ≤ wire.length := by
  induction valid with
  | nil => simp
  | step keyValid valueValid rest ih =>
    have hk := schemaKey_nonempty (WantedKeyMeaning receiptStatusNames) _ _ keyValid
    have hv := receiptStatusItem_nonempty _ _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusItem_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusItem_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusPairs_count_bound
