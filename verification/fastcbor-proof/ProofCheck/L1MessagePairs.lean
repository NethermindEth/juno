import ProofCheck.L1MessageKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive L1MessageValueValid (d : Nat) : Option L1MessageField → L1MessageValue → List w8 → List w8 → L1MessageValue → Prop where
  | selected {field old item rest final} (valid : L1MessageFieldValid d field old item rest final) :
      L1MessageValueValid d (some field) old item rest final
  | ignored {old : L1MessageValue} {item rest} (valid : SkipValid d item) :
      L1MessageValueValid d none old item rest old

inductive L1MessageValueFailure where
  | field (reason : L1MessageFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive L1MessageValueRejected (d : Nat) : Option L1MessageField → List w8 → Nat → L1MessageValueFailure → Prop where
  | selected {field wire offset reason} (failure : L1MessageFieldRejected d field wire offset reason) :
      L1MessageValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      L1MessageValueRejected d none wire offset (.ignored reason)

def L1MessageChoiceFresh (seen : List L1MessageField) : Option L1MessageField → Prop
  | none => True
  | some field => field ∉ seen

def l1MessageChoiceSeen (seen : List L1MessageField) : Option L1MessageField → List L1MessageField
  | none => seen
  | some field => field :: seen

inductive L1MessageFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : L1MessageField)
  | value (reason : L1MessageValueFailure)

/-- Successful pairs retain the actual suffix in every nested slice guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive L1MessagePairsValid (d : Nat) : Nat → List L1MessageField → L1MessageValue → List w8 → List w8 → L1MessageValue → Prop where
  | nil (seen : List L1MessageField) (old : L1MessageValue) (suffix : List w8) :
      L1MessagePairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : L1MessageKeyValid key choice) (fresh : L1MessageChoiceFresh seen choice)
      (valueValid : L1MessageValueValid d choice old value (tail ++ suffix) middle)
      (rest : L1MessagePairsValid d n (l1MessageChoiceSeen seen choice) middle tail suffix final) :
      L1MessagePairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial L1ToL2Message and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive L1MessagePairsRejected (d : Nat) : Nat → List L1MessageField → L1MessageValue → List w8 → L1MessageValue →
    Nat → Nat → L1MessageFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      L1MessagePairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : L1MessageKeyValid key (some field)) (duplicate : field ∈ seen) :
      L1MessagePairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : L1MessageKeyValid key choice)
      (fresh : L1MessageChoiceFresh seen choice) (failure : L1MessageValueRejected d choice tail offset reason) :
      L1MessagePairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : L1MessageKeyValid key choice) (fresh : L1MessageChoiceFresh seen choice)
      (valueValid : L1MessageValueValid d choice old value tail middle)
      (failure : L1MessagePairsRejected d n (l1MessageChoiceSeen seen choice) middle tail final consumed offset reason) :
      L1MessagePairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem l1MessageValue_classify (d : Nat) (choice : Option L1MessageField) (old : L1MessageValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L1MessageValueValid d choice old item rest final) ∨
    (∃ offset reason, L1MessageValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases l1MessageField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem l1MessagePairs_classify (d n : Nat) (seen : List L1MessageField) (old : L1MessageValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L1MessagePairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, L1MessagePairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases l1MessageKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : L1MessageChoiceFresh seen choice
      · rcases l1MessageValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (l1MessageChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using L1MessagePairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [L1MessageChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem l1MessageValue_nonempty (d : Nat) (choice : Option L1MessageField) (old final : L1MessageValue)
    (item rest : List w8) (valid : L1MessageValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact l1MessageField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem l1MessagePairs_count_bound (d n : Nat) (seen : List L1MessageField) (old final : L1MessageValue)
    (item rest : List w8) (valid : L1MessagePairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty L1MessageKeyMeaning _ _ keyValid
    have hv := l1MessageValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem l1MessageValueRejected_bound (d offset : Nat) (choice : Option L1MessageField) (wire : List w8)
    (reason : L1MessageValueFailure) (failure : L1MessageValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact l1MessageField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem l1MessagePairsRejected_bounds (d n consumed offset : Nat) (seen : List L1MessageField) (old final : L1MessageValue)
    (wire : List w8) (reason : L1MessageFailureReason)
    (failure : L1MessagePairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := l1MessageValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePairsRejected_bounds
