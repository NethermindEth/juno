import ProofCheck.L2MessageKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive L2MessageValueValid (d : Nat) : Option L2MessageField → L2MessageValue → List w8 → List w8 → L2MessageValue → Prop where
  | selected {field old item rest final} (valid : L2MessageFieldValid d field old item rest final) :
      L2MessageValueValid d (some field) old item rest final
  | ignored {old : L2MessageValue} {item rest} (valid : SkipValid d item) :
      L2MessageValueValid d none old item rest old

inductive L2MessageValueFailure where
  | field (reason : L2MessageFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive L2MessageValueRejected (d : Nat) : Option L2MessageField → List w8 → Nat → L2MessageValueFailure → Prop where
  | selected {field wire offset reason} (failure : L2MessageFieldRejected d field wire offset reason) :
      L2MessageValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      L2MessageValueRejected d none wire offset (.ignored reason)

def L2MessageChoiceFresh (seen : List L2MessageField) : Option L2MessageField → Prop
  | none => True
  | some field => field ∉ seen

def l2MessageChoiceSeen (seen : List L2MessageField) : Option L2MessageField → List L2MessageField
  | none => seen
  | some field => field :: seen

inductive L2MessageFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : L2MessageField)
  | value (reason : L2MessageValueFailure)

/-- Successful pairs retain the actual suffix in every nested slice guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive L2MessagePairsValid (d : Nat) : Nat → List L2MessageField → L2MessageValue → List w8 → List w8 → L2MessageValue → Prop where
  | nil (seen : List L2MessageField) (old : L2MessageValue) (suffix : List w8) :
      L2MessagePairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : L2MessageKeyValid key choice) (fresh : L2MessageChoiceFresh seen choice)
      (valueValid : L2MessageValueValid d choice old value (tail ++ suffix) middle)
      (rest : L2MessagePairsValid d n (l2MessageChoiceSeen seen choice) middle tail suffix final) :
      L2MessagePairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial L2ToL1Message and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive L2MessagePairsRejected (d : Nat) : Nat → List L2MessageField → L2MessageValue → List w8 → L2MessageValue →
    Nat → Nat → L2MessageFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      L2MessagePairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : L2MessageKeyValid key (some field)) (duplicate : field ∈ seen) :
      L2MessagePairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : L2MessageKeyValid key choice)
      (fresh : L2MessageChoiceFresh seen choice) (failure : L2MessageValueRejected d choice tail offset reason) :
      L2MessagePairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : L2MessageKeyValid key choice) (fresh : L2MessageChoiceFresh seen choice)
      (valueValid : L2MessageValueValid d choice old value tail middle)
      (failure : L2MessagePairsRejected d n (l2MessageChoiceSeen seen choice) middle tail final consumed offset reason) :
      L2MessagePairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem l2MessageValue_classify (d : Nat) (choice : Option L2MessageField) (old : L2MessageValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L2MessageValueValid d choice old item rest final) ∨
    (∃ offset reason, L2MessageValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases l2MessageField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem l2MessagePairs_classify (d n : Nat) (seen : List L2MessageField) (old : L2MessageValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L2MessagePairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, L2MessagePairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases l2MessageKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : L2MessageChoiceFresh seen choice
      · rcases l2MessageValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (l2MessageChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using L2MessagePairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [L2MessageChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem l2MessageValue_nonempty (d : Nat) (choice : Option L2MessageField) (old final : L2MessageValue)
    (item rest : List w8) (valid : L2MessageValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact l2MessageField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem l2MessagePairs_count_bound (d n : Nat) (seen : List L2MessageField) (old final : L2MessageValue)
    (item rest : List w8) (valid : L2MessagePairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty L2MessageKeyMeaning _ _ keyValid
    have hv := l2MessageValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem l2MessageValueRejected_bound (d offset : Nat) (choice : Option L2MessageField) (wire : List w8)
    (reason : L2MessageValueFailure) (failure : L2MessageValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact l2MessageField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem l2MessagePairsRejected_bounds (d n consumed offset : Nat) (seen : List L2MessageField) (old final : L2MessageValue)
    (wire : List w8) (reason : L2MessageFailureReason)
    (failure : L2MessagePairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := l2MessageValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePairsRejected_bounds
