import ProofCheck.ReceiptKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ReceiptValueValid (d : Nat) : Option ReceiptField → ReceiptValue → List w8 → List w8 → ReceiptValue → Prop where
  | selected {field old item rest final} (valid : ReceiptFieldValid d field old item rest final) :
      ReceiptValueValid d (some field) old item rest final
  | ignored {old : ReceiptValue} {item rest} (valid : SkipValid d item) :
      ReceiptValueValid d none old item rest old

inductive ReceiptValueFailure where
  | field (reason : ReceiptFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive ReceiptValueRejected (d : Nat) : Option ReceiptField → ReceiptValue → List w8 → ReceiptValue →
    Nat → Nat → Nat → ReceiptValueFailure → Prop where
  | selected {field old wire consumed offset extraDepth reason}
      (failure : ReceiptFieldRejected d field wire consumed offset extraDepth reason) :
      ReceiptValueRejected d (some field) old wire old consumed offset extraDepth (.field reason)
  | ignored {old : ReceiptValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      ReceiptValueRejected d none old wire old 0 offset 0 (.ignored reason)

def ReceiptChoiceFresh (seen : List ReceiptField) : Option ReceiptField → Prop
  | none => True
  | some field => field ∉ seen

def receiptChoiceSeen (seen : List ReceiptField) : Option ReceiptField → List ReceiptField
  | none => seen
  | some field => field :: seen

/-- Numeric failure positions are structured diagnostic offsets except for
    bare FeeUnit overflow, where the field contract records only the consumed boundary. -/
inductive ReceiptFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : ReceiptField)
  | value (reason : ReceiptValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive ReceiptPairsValid (d : Nat) : Nat → List ReceiptField → ReceiptValue → List w8 → List w8 → ReceiptValue → Prop where
  | nil (seen : List ReceiptField) (old : ReceiptValue) (suffix : List w8) :
      ReceiptPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : ReceiptKeyValid key choice) (fresh : ReceiptChoiceFresh seen choice)
      (valueValid : ReceiptValueValid d choice old value (tail ++ suffix) middle)
      (rest : ReceiptPairsValid d n (receiptChoiceSeen seen choice) middle tail suffix final) :
      ReceiptPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial TransactionReceipt and separates consumed bytes from the
    diagnostic position. Failed fields preserve the receipt but may retain private partial children and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive ReceiptPairsRejected (d : Nat) : Nat → List ReceiptField → ReceiptValue → List w8 → ReceiptValue →
    Nat → Nat → Nat → ReceiptFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      ReceiptPairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : ReceiptKeyValid key (some field)) (duplicate : field ∈ seen) :
      ReceiptPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : ReceiptKeyValid key choice)
      (fresh : ReceiptChoiceFresh seen choice)
      (failure : ReceiptValueRejected d choice old tail final consumed offset extraDepth reason) :
      ReceiptPairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : ReceiptKeyValid key choice) (fresh : ReceiptChoiceFresh seen choice)
      (valueValid : ReceiptValueValid d choice old value tail middle)
      (failure : ReceiptPairsRejected d n (receiptChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      ReceiptPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem receiptValue_classify (d : Nat) (choice : Option ReceiptField) (old : ReceiptValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, ReceiptValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases receiptField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨old, consumed, offset, extraDepth, _, .selected failure⟩

theorem receiptPairs_classify (d n : Nat) (seen : List ReceiptField) (old : ReceiptValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, ReceiptPairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases receiptKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : ReceiptChoiceFresh seen choice
      · rcases receiptValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (receiptChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using ReceiptPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [ReceiptChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem receiptValue_nonempty (d : Nat) (choice : Option ReceiptField) (old final : ReceiptValue)
    (item rest : List w8) (valid : ReceiptValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact receiptField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem receiptPairs_count_bound (d n : Nat) (seen : List ReceiptField) (old final : ReceiptValue)
    (item rest : List w8) (valid : ReceiptPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty ReceiptKeyMeaning _ _ keyValid
    have hv := receiptValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem receiptValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option ReceiptField)
    (old final : ReceiptValue) (wire : List w8) (reason : ReceiptValueFailure)
    (failure : ReceiptValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact receiptField_rejected_bounds _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem receiptValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option ReceiptField)
    (old final : ReceiptValue) (wire : List w8) (reason : ReceiptValueFailure) (depth : d ≤ 32)
    (failure : ReceiptValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact receiptField_rejected_depth _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem receiptPairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List ReceiptField) (old final : ReceiptValue)
    (wire : List w8) (reason : ReceiptFailureReason)
    (failure : ReceiptPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := receiptValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem receiptPairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List ReceiptField) (old final : ReceiptValue)
    (wire : List w8) (reason : ReceiptFailureReason) (depth : d ≤ 32)
    (failure : ReceiptPairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact receiptValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPairsRejected_depth
