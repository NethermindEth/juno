import ProofCheck.SierraEntryPointKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive SierraEntryPointValueValid (d : Nat) : Option SierraEntryPointField → SierraEntryPointValue → List w8 → List w8 → SierraEntryPointValue → Prop where
  | selected {field old item rest final} (valid : SierraEntryPointFieldValid d field old item rest final) :
      SierraEntryPointValueValid d (some field) old item rest final
  | ignored {old : SierraEntryPointValue} {item rest} (valid : SkipValid d item) :
      SierraEntryPointValueValid d none old item rest old

inductive SierraEntryPointValueFailure where
  | field (reason : SierraEntryPointFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive SierraEntryPointValueRejected (d : Nat) : Option SierraEntryPointField → List w8 → Nat → SierraEntryPointValueFailure → Prop where
  | selected {field wire offset reason} (failure : SierraEntryPointFieldRejected d field wire offset reason) :
      SierraEntryPointValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      SierraEntryPointValueRejected d none wire offset (.ignored reason)

def SierraEntryPointChoiceFresh (seen : List SierraEntryPointField) : Option SierraEntryPointField → Prop
  | none => True
  | some field => field ∉ seen

def sierraEntryPointChoiceSeen (seen : List SierraEntryPointField) : Option SierraEntryPointField → List SierraEntryPointField
  | none => seen
  | some field => field :: seen

inductive SierraEntryPointFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : SierraEntryPointField)
  | value (reason : SierraEntryPointValueFailure)

/-- Successful pairs retain the unread suffix after their field values.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive SierraEntryPointPairsValid (d : Nat) : Nat → List SierraEntryPointField → SierraEntryPointValue → List w8 → List w8 → SierraEntryPointValue → Prop where
  | nil (seen : List SierraEntryPointField) (old : SierraEntryPointValue) (suffix : List w8) :
      SierraEntryPointPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : SierraEntryPointKeyValid key choice) (fresh : SierraEntryPointChoiceFresh seen choice)
      (valueValid : SierraEntryPointValueValid d choice old value (tail ++ suffix) middle)
      (rest : SierraEntryPointPairsValid d n (sierraEntryPointChoiceSeen seen choice) middle tail suffix final) :
      SierraEntryPointPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial SierraEntryPoint and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive SierraEntryPointPairsRejected (d : Nat) : Nat → List SierraEntryPointField → SierraEntryPointValue → List w8 → SierraEntryPointValue →
    Nat → Nat → SierraEntryPointFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      SierraEntryPointPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : SierraEntryPointKeyValid key (some field)) (duplicate : field ∈ seen) :
      SierraEntryPointPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : SierraEntryPointKeyValid key choice)
      (fresh : SierraEntryPointChoiceFresh seen choice) (failure : SierraEntryPointValueRejected d choice tail offset reason) :
      SierraEntryPointPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : SierraEntryPointKeyValid key choice) (fresh : SierraEntryPointChoiceFresh seen choice)
      (valueValid : SierraEntryPointValueValid d choice old value tail middle)
      (failure : SierraEntryPointPairsRejected d n (sierraEntryPointChoiceSeen seen choice) middle tail final consumed offset reason) :
      SierraEntryPointPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem sierraEntryPointValue_classify (d : Nat) (choice : Option SierraEntryPointField) (old : SierraEntryPointValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SierraEntryPointValueValid d choice old item rest final) ∨
    (∃ offset reason, SierraEntryPointValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases sierraEntryPointField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem sierraEntryPointPairs_classify (d n : Nat) (seen : List SierraEntryPointField) (old : SierraEntryPointValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SierraEntryPointPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, SierraEntryPointPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases sierraEntryPointKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : SierraEntryPointChoiceFresh seen choice
      · rcases sierraEntryPointValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (sierraEntryPointChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using SierraEntryPointPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [SierraEntryPointChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem sierraEntryPointValue_nonempty (d : Nat) (choice : Option SierraEntryPointField) (old final : SierraEntryPointValue)
    (item rest : List w8) (valid : SierraEntryPointValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact sierraEntryPointField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem sierraEntryPointPairs_count_bound (d n : Nat) (seen : List SierraEntryPointField) (old final : SierraEntryPointValue)
    (item rest : List w8) (valid : SierraEntryPointPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty SierraEntryPointKeyMeaning _ _ keyValid
    have hv := sierraEntryPointValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem sierraEntryPointValueRejected_bound (d offset : Nat) (choice : Option SierraEntryPointField) (wire : List w8)
    (reason : SierraEntryPointValueFailure) (failure : SierraEntryPointValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact sierraEntryPointField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem sierraEntryPointPairsRejected_bounds (d n consumed offset : Nat) (seen : List SierraEntryPointField) (old final : SierraEntryPointValue)
    (wire : List w8) (reason : SierraEntryPointFailureReason)
    (failure : SierraEntryPointPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := sierraEntryPointValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointPairsRejected_bounds
