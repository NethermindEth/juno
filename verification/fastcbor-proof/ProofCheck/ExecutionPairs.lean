import ProofCheck.ExecutionKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ExecutionValueValid (d : Nat) : Option ExecutionField → ExecutionValue → List w8 → List w8 → ExecutionValue → Prop where
  | selected {field old item rest final} (valid : ExecutionFieldValid d field old item rest final) :
      ExecutionValueValid d (some field) old item rest final
  | ignored {old : ExecutionValue} {item rest} (valid : SkipValid d item) :
      ExecutionValueValid d none old item rest old

inductive ExecutionValueFailure where
  | field (reason : ExecutionFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive ExecutionValueRejected (d : Nat) : Option ExecutionField → ExecutionValue → List w8 → ExecutionValue →
    Nat → Nat → Nat → ExecutionValueFailure → Prop where
  | selected {field old wire final consumed offset extraDepth reason}
      (failure : ExecutionFieldRejected d field old wire final consumed offset extraDepth reason) :
      ExecutionValueRejected d (some field) old wire final consumed offset extraDepth (.field reason)
  | ignored {old : ExecutionValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      ExecutionValueRejected d none old wire old 0 offset 0 (.ignored reason)

def ExecutionChoiceFresh (seen : List ExecutionField) : Option ExecutionField → Prop
  | none => True
  | some field => field ∉ seen

def executionChoiceSeen (seen : List ExecutionField) : Option ExecutionField → List ExecutionField
  | none => seen
  | some field => field :: seen

inductive ExecutionFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : ExecutionField)
  | value (reason : ExecutionValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive ExecutionPairsValid (d : Nat) : Nat → List ExecutionField → ExecutionValue → List w8 → List w8 → ExecutionValue → Prop where
  | nil (seen : List ExecutionField) (old : ExecutionValue) (suffix : List w8) :
      ExecutionPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : ExecutionKeyValid key choice) (fresh : ExecutionChoiceFresh seen choice)
      (valueValid : ExecutionValueValid d choice old value (tail ++ suffix) middle)
      (rest : ExecutionPairsValid d n (executionChoiceSeen seen choice) middle tail suffix final) :
      ExecutionPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial ExecutionResources and separates consumed bytes from the
    diagnostic position. Nested field failures retain partial values and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive ExecutionPairsRejected (d : Nat) : Nat → List ExecutionField → ExecutionValue → List w8 → ExecutionValue →
    Nat → Nat → Nat → ExecutionFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      ExecutionPairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : ExecutionKeyValid key (some field)) (duplicate : field ∈ seen) :
      ExecutionPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : ExecutionKeyValid key choice)
      (fresh : ExecutionChoiceFresh seen choice)
      (failure : ExecutionValueRejected d choice old tail final consumed offset extraDepth reason) :
      ExecutionPairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : ExecutionKeyValid key choice) (fresh : ExecutionChoiceFresh seen choice)
      (valueValid : ExecutionValueValid d choice old value tail middle)
      (failure : ExecutionPairsRejected d n (executionChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      ExecutionPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem executionValue_classify (d : Nat) (choice : Option ExecutionField) (old : ExecutionValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ExecutionValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, ExecutionValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases executionField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨final, consumed, offset, extraDepth, _, .selected failure⟩

theorem executionPairs_classify (d n : Nat) (seen : List ExecutionField) (old : ExecutionValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ExecutionPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, ExecutionPairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases executionKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : ExecutionChoiceFresh seen choice
      · rcases executionValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (executionChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using ExecutionPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [ExecutionChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem executionValue_nonempty (d : Nat) (choice : Option ExecutionField) (old final : ExecutionValue)
    (item rest : List w8) (valid : ExecutionValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact executionField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem executionPairs_count_bound (d n : Nat) (seen : List ExecutionField) (old final : ExecutionValue)
    (item rest : List w8) (valid : ExecutionPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty ExecutionKeyMeaning _ _ keyValid
    have hv := executionValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem executionValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option ExecutionField)
    (old final : ExecutionValue) (wire : List w8) (reason : ExecutionValueFailure)
    (failure : ExecutionValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact executionField_rejected_bounds _ _ _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem executionValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option ExecutionField)
    (old final : ExecutionValue) (wire : List w8) (reason : ExecutionValueFailure) (depth : d ≤ 32)
    (failure : ExecutionValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact executionField_rejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem executionPairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List ExecutionField) (old final : ExecutionValue)
    (wire : List w8) (reason : ExecutionFailureReason)
    (failure : ExecutionPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := executionValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem executionPairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List ExecutionField) (old final : ExecutionValue)
    (wire : List w8) (reason : ExecutionFailureReason) (depth : d ≤ 32)
    (failure : ExecutionPairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact executionValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.executionValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.executionPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.executionValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.executionPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.executionValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.executionValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.executionPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.executionPairsRejected_depth
