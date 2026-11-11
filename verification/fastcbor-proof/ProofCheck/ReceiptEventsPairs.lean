import ProofCheck.ReceiptEventsValues
import ProofCheck.WantedSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def receiptEventsNextSeen (seen : List Nat) (choice : Option Nat) : List Nat :=
  match wantedSelect seen choice with
  | none => seen
  | some index => index :: seen

inductive ReceiptEventsItemValid (d : Nat) (seen : List Nat) (choice : Option Nat)
    (old : ReceiptEventsValue) : List w8 → List w8 → ReceiptEventsValue → Prop where
  | ignored {item rest} (ignored : wantedSelect seen choice = none) (valid : SkipValid d item) :
      ReceiptEventsItemValid d seen choice old item rest old
  | selected {index field item rest final} (fresh : wantedSelect seen choice = some index)
      (lookup : receiptEventsFields[index]? = some field)
      (valid : ReceiptEventsFieldValid d field old item rest final) :
      ReceiptEventsItemValid d seen choice old item rest final

inductive ReceiptEventsFailure where
  | key (reason : KeyFailureReason)
  | ignored (reason : SkipFailureReason)
  | field (reason : ReceiptEventsFieldFailure)

/-- Ignored values roll back; selected Events may retain nested consumption and
    depth. Neither failure changes the previously published projection fields. -/
inductive ReceiptEventsItemRejected (d : Nat) (seen : List Nat) (choice : Option Nat) :
    List w8 → Nat → Nat → Nat → ReceiptEventsFailure → Prop where
  | ignored {wire offset reason} (ignored : wantedSelect seen choice = none) (failure : SkipFailure d wire offset reason) :
      ReceiptEventsItemRejected d seen choice wire 0 offset 0 (.ignored reason)
  | selected {index field wire consumed offset extraDepth reason} (fresh : wantedSelect seen choice = some index)
      (lookup : receiptEventsFields[index]? = some field)
      (failure : ReceiptEventsFieldRejected d field wire consumed offset extraDepth reason) :
      ReceiptEventsItemRejected d seen choice wire consumed offset extraDepth (.field reason)

theorem receiptEventsItem_classify (d : Nat) (seen : List Nat) (choice : Option Nat)
    (old : ReceiptEventsValue) (key wire : List w8)
    (keyValid : WantedKeyValid receiptEventsNames key choice) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptEventsItemValid d seen choice old item rest final) ∨
    (∃ consumed offset extraDepth reason, ReceiptEventsItemRejected d seen choice wire consumed offset extraDepth reason) := by
  cases selected : wantedSelect seen choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored selected valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .ignored selected failure⟩
  | some index =>
    have bound := (wantedValid_selected d 1 index 0 receiptEventsNames seen key (.selected keyValid selected)).1
    obtain ⟨field, lookup⟩ := receiptEventsField_exists index bound
    rcases receiptEventsField_classify d field old wire depth with
      ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected selected lookup valid⟩
    · exact Or.inr ⟨consumed, offset, extraDepth, _, .selected selected lookup failure⟩

theorem receiptEventsItem_nonempty (d : Nat) (seen : List Nat) (choice : Option Nat)
    (old final : ReceiptEventsValue) (item rest : List w8)
    (valid : ReceiptEventsItemValid d seen choice old item rest final) : 0 < item.length := by
  cases valid with
  | ignored ignored valid => exact skipValid_nonempty d item valid
  | selected fresh lookup valid => exact receiptEventsField_nonempty _ _ _ _ _ _ valid

/-- Wire-order interpretation with first-key-wins selection. The actual suffix
    remains available to each selected Events field's nested count guards. -/
inductive ReceiptEventsPairsValid (d : Nat) :
    Nat → List Nat → ReceiptEventsValue → List w8 → List w8 → ReceiptEventsValue → Prop where
  | nil (seen : List Nat) (old : ReceiptEventsValue) (suffix : List w8) :
      ReceiptEventsPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : WantedKeyValid receiptEventsNames key choice)
      (valueValid : ReceiptEventsItemValid d seen choice old value (tail ++ suffix) middle)
      (rest : ReceiptEventsPairsValid d n (receiptEventsNextSeen seen choice) middle tail suffix final) :
      ReceiptEventsPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

inductive ReceiptEventsPairsRejected (d : Nat) :
    Nat → List Nat → ReceiptEventsValue → List w8 → ReceiptEventsValue → Nat → Nat → Nat → ReceiptEventsFailure → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      ReceiptEventsPairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | value {n seen choice old key tail consumed offset extraDepth reason}
      (keyValid : WantedKeyValid receiptEventsNames key choice)
      (failure : ReceiptEventsItemRejected d seen choice tail consumed offset extraDepth reason) :
      ReceiptEventsPairsRejected d (n + 1) seen old (key ++ tail) old
        (key.length + consumed) (key.length + offset) extraDepth reason
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : WantedKeyValid receiptEventsNames key choice)
      (valueValid : ReceiptEventsItemValid d seen choice old value tail middle)
      (failure : ReceiptEventsPairsRejected d n (receiptEventsNextSeen seen choice) middle tail final consumed offset extraDepth reason) :
      ReceiptEventsPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem receiptEventsPairs_classify (d n : Nat) (seen : List Nat)
    (old : ReceiptEventsValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptEventsPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, ReceiptEventsPairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases schemaKey_classify (WantedKeyMeaning receiptEventsNames)
      (wantedKeyMeaning_total receiptEventsNames) wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      rcases receiptEventsItem_classify d seen choice old key tail keyValid depth with
        ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
      · subst tail
        rcases ih (receiptEventsNextSeen seen choice) middle rest with
          ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
        · subst rest
          exact Or.inl ⟨key ++ value ++ items, suffix, final,
            by simp only [List.append_assoc], .step keyValid valueValid validRest⟩
        · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
          simpa only [List.append_assoc] using ReceiptEventsPairsRejected.next keyValid valueValid failure
      · exact Or.inr ⟨old, key.length + consumed, key.length + offset, extraDepth, reason, .value keyValid failure⟩
    · exact Or.inr ⟨old, 0, 0, 0, .key reason, .key failure⟩

theorem receiptEventsPairs_count_bound (d n : Nat) (seen : List Nat)
    (old final : ReceiptEventsValue) (item rest : List w8)
    (valid : ReceiptEventsPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid valueValid rest ih =>
    have hk := schemaKey_nonempty (WantedKeyMeaning receiptEventsNames) _ _ keyValid
    have hv := receiptEventsItem_nonempty _ _ _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsItem_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsItem_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsPairs_count_bound
