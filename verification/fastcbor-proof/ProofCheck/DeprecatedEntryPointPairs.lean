import ProofCheck.DeprecatedEntryPointKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive DeprecatedEntryPointValueValid (d : Nat) : Option DeprecatedEntryPointField → DeprecatedEntryPointValue → List w8 → List w8 → DeprecatedEntryPointValue → Prop where
  | selected {field old item rest final} (valid : DeprecatedEntryPointFieldValid d field old item rest final) :
      DeprecatedEntryPointValueValid d (some field) old item rest final
  | ignored {old : DeprecatedEntryPointValue} {item rest} (valid : SkipValid d item) :
      DeprecatedEntryPointValueValid d none old item rest old

inductive DeprecatedEntryPointValueFailure where
  | field (reason : DeprecatedEntryPointFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive DeprecatedEntryPointValueRejected (d : Nat) : Option DeprecatedEntryPointField → List w8 → Nat → DeprecatedEntryPointValueFailure → Prop where
  | selected {field wire offset reason} (failure : DeprecatedEntryPointFieldRejected d field wire offset reason) :
      DeprecatedEntryPointValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      DeprecatedEntryPointValueRejected d none wire offset (.ignored reason)

def DeprecatedEntryPointChoiceFresh (seen : List DeprecatedEntryPointField) : Option DeprecatedEntryPointField → Prop
  | none => True
  | some field => field ∉ seen

def deprecatedEntryPointChoiceSeen (seen : List DeprecatedEntryPointField) : Option DeprecatedEntryPointField → List DeprecatedEntryPointField
  | none => seen
  | some field => field :: seen

inductive DeprecatedEntryPointFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : DeprecatedEntryPointField)
  | value (reason : DeprecatedEntryPointValueFailure)

/-- Successful pairs retain the unread suffix after their field values.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive DeprecatedEntryPointPairsValid (d : Nat) : Nat → List DeprecatedEntryPointField → DeprecatedEntryPointValue → List w8 → List w8 → DeprecatedEntryPointValue → Prop where
  | nil (seen : List DeprecatedEntryPointField) (old : DeprecatedEntryPointValue) (suffix : List w8) :
      DeprecatedEntryPointPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : DeprecatedEntryPointKeyValid key choice) (fresh : DeprecatedEntryPointChoiceFresh seen choice)
      (valueValid : DeprecatedEntryPointValueValid d choice old value (tail ++ suffix) middle)
      (rest : DeprecatedEntryPointPairsValid d n (deprecatedEntryPointChoiceSeen seen choice) middle tail suffix final) :
      DeprecatedEntryPointPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial DeprecatedEntryPoint and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive DeprecatedEntryPointPairsRejected (d : Nat) : Nat → List DeprecatedEntryPointField → DeprecatedEntryPointValue → List w8 → DeprecatedEntryPointValue →
    Nat → Nat → DeprecatedEntryPointFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      DeprecatedEntryPointPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : DeprecatedEntryPointKeyValid key (some field)) (duplicate : field ∈ seen) :
      DeprecatedEntryPointPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : DeprecatedEntryPointKeyValid key choice)
      (fresh : DeprecatedEntryPointChoiceFresh seen choice) (failure : DeprecatedEntryPointValueRejected d choice tail offset reason) :
      DeprecatedEntryPointPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : DeprecatedEntryPointKeyValid key choice) (fresh : DeprecatedEntryPointChoiceFresh seen choice)
      (valueValid : DeprecatedEntryPointValueValid d choice old value tail middle)
      (failure : DeprecatedEntryPointPairsRejected d n (deprecatedEntryPointChoiceSeen seen choice) middle tail final consumed offset reason) :
      DeprecatedEntryPointPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem deprecatedEntryPointValue_classify (d : Nat) (choice : Option DeprecatedEntryPointField) (old : DeprecatedEntryPointValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeprecatedEntryPointValueValid d choice old item rest final) ∨
    (∃ offset reason, DeprecatedEntryPointValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases deprecatedEntryPointField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem deprecatedEntryPointPairs_classify (d n : Nat) (seen : List DeprecatedEntryPointField) (old : DeprecatedEntryPointValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeprecatedEntryPointPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, DeprecatedEntryPointPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases deprecatedEntryPointKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : DeprecatedEntryPointChoiceFresh seen choice
      · rcases deprecatedEntryPointValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (deprecatedEntryPointChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using DeprecatedEntryPointPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [DeprecatedEntryPointChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem deprecatedEntryPointValue_nonempty (d : Nat) (choice : Option DeprecatedEntryPointField) (old final : DeprecatedEntryPointValue)
    (item rest : List w8) (valid : DeprecatedEntryPointValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact deprecatedEntryPointField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem deprecatedEntryPointPairs_count_bound (d n : Nat) (seen : List DeprecatedEntryPointField) (old final : DeprecatedEntryPointValue)
    (item rest : List w8) (valid : DeprecatedEntryPointPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty DeprecatedEntryPointKeyMeaning _ _ keyValid
    have hv := deprecatedEntryPointValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem deprecatedEntryPointValueRejected_bound (d offset : Nat) (choice : Option DeprecatedEntryPointField) (wire : List w8)
    (reason : DeprecatedEntryPointValueFailure) (failure : DeprecatedEntryPointValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact deprecatedEntryPointField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem deprecatedEntryPointPairsRejected_bounds (d n consumed offset : Nat) (seen : List DeprecatedEntryPointField) (old final : DeprecatedEntryPointValue)
    (wire : List w8) (reason : DeprecatedEntryPointFailureReason)
    (failure : DeprecatedEntryPointPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := deprecatedEntryPointValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointPairsRejected_bounds
