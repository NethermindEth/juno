import ProofCheck.GasUsedKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive GasUsedValueValid (d : Nat) : Option GasUsedField → GasUsedValue → List w8 → List w8 → GasUsedValue → Prop where
  | selected {field old item rest final} (valid : GasUsedFieldValid d field old item rest final) :
      GasUsedValueValid d (some field) old item rest final
  | ignored {old : GasUsedValue} {item rest} (valid : SkipValid d item) :
      GasUsedValueValid d none old item rest old

inductive GasUsedValueFailure where
  | field (reason : GasUsedFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive GasUsedValueRejected (d : Nat) : Option GasUsedField → List w8 → Nat → GasUsedValueFailure → Prop where
  | selected {field wire offset reason} (failure : GasUsedFieldRejected d field wire offset reason) :
      GasUsedValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      GasUsedValueRejected d none wire offset (.ignored reason)

def GasUsedChoiceFresh (seen : List GasUsedField) : Option GasUsedField → Prop
  | none => True
  | some field => field ∉ seen

def gasUsedChoiceSeen (seen : List GasUsedField) : Option GasUsedField → List GasUsedField
  | none => seen
  | some field => field :: seen

inductive GasUsedFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : GasUsedField)
  | value (reason : GasUsedValueFailure)

/-- Successful pairs retain the unread suffix after their scalar values.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive GasUsedPairsValid (d : Nat) : Nat → List GasUsedField → GasUsedValue → List w8 → List w8 → GasUsedValue → Prop where
  | nil (seen : List GasUsedField) (old : GasUsedValue) (suffix : List w8) :
      GasUsedPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : GasUsedKeyValid key choice) (fresh : GasUsedChoiceFresh seen choice)
      (valueValid : GasUsedValueValid d choice old value (tail ++ suffix) middle)
      (rest : GasUsedPairsValid d n (gasUsedChoiceSeen seen choice) middle tail suffix final) :
      GasUsedPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial GasConsumed and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive GasUsedPairsRejected (d : Nat) : Nat → List GasUsedField → GasUsedValue → List w8 → GasUsedValue →
    Nat → Nat → GasUsedFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      GasUsedPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : GasUsedKeyValid key (some field)) (duplicate : field ∈ seen) :
      GasUsedPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : GasUsedKeyValid key choice)
      (fresh : GasUsedChoiceFresh seen choice) (failure : GasUsedValueRejected d choice tail offset reason) :
      GasUsedPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : GasUsedKeyValid key choice) (fresh : GasUsedChoiceFresh seen choice)
      (valueValid : GasUsedValueValid d choice old value tail middle)
      (failure : GasUsedPairsRejected d n (gasUsedChoiceSeen seen choice) middle tail final consumed offset reason) :
      GasUsedPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem gasUsedValue_classify (d : Nat) (choice : Option GasUsedField) (old : GasUsedValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ GasUsedValueValid d choice old item rest final) ∨
    (∃ offset reason, GasUsedValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases gasUsedField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem gasUsedPairs_classify (d n : Nat) (seen : List GasUsedField) (old : GasUsedValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ GasUsedPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, GasUsedPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases gasUsedKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : GasUsedChoiceFresh seen choice
      · rcases gasUsedValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (gasUsedChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using GasUsedPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [GasUsedChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem gasUsedValue_nonempty (d : Nat) (choice : Option GasUsedField) (old final : GasUsedValue)
    (item rest : List w8) (valid : GasUsedValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact gasUsedField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem gasUsedPairs_count_bound (d n : Nat) (seen : List GasUsedField) (old final : GasUsedValue)
    (item rest : List w8) (valid : GasUsedPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty GasUsedKeyMeaning _ _ keyValid
    have hv := gasUsedValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem gasUsedValueRejected_bound (d offset : Nat) (choice : Option GasUsedField) (wire : List w8)
    (reason : GasUsedValueFailure) (failure : GasUsedValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact gasUsedField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem gasUsedPairsRejected_bounds (d n consumed offset : Nat) (seen : List GasUsedField) (old final : GasUsedValue)
    (wire : List w8) (reason : GasUsedFailureReason)
    (failure : GasUsedPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := gasUsedValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedPairsRejected_bounds
