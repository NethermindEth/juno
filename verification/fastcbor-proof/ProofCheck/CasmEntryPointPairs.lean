import ProofCheck.CasmEntryPointKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive CasmEntryPointValueValid (d : Nat) : Option CasmEntryPointField → CasmEntryPointValue → List w8 → List w8 → CasmEntryPointValue → Prop where
  | selected {field old item rest final} (valid : CasmEntryPointFieldValid d field old item rest final) :
      CasmEntryPointValueValid d (some field) old item rest final
  | ignored {old : CasmEntryPointValue} {item rest} (valid : SkipValid d item) :
      CasmEntryPointValueValid d none old item rest old

inductive CasmEntryPointValueFailure where
  | field (reason : CasmEntryPointFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive CasmEntryPointValueRejected (d : Nat) : Option CasmEntryPointField → CasmEntryPointValue → List w8 → CasmEntryPointValue →
    Nat → Nat → Nat → CasmEntryPointValueFailure → Prop where
  | selected {field old wire consumed offset extraDepth reason}
      (failure : CasmEntryPointFieldRejected d field wire consumed offset extraDepth reason) :
      CasmEntryPointValueRejected d (some field) old wire old consumed offset extraDepth (.field reason)
  | ignored {old : CasmEntryPointValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      CasmEntryPointValueRejected d none old wire old 0 offset 0 (.ignored reason)

def CasmEntryPointChoiceFresh (seen : List CasmEntryPointField) : Option CasmEntryPointField → Prop
  | none => True
  | some field => field ∉ seen

def casmEntryPointChoiceSeen (seen : List CasmEntryPointField) : Option CasmEntryPointField → List CasmEntryPointField
  | none => seen
  | some field => field :: seen

/-- Failure reasons distinguish opening, key, duplicate and value diagnostics. -/
inductive CasmEntryPointFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : CasmEntryPointField)
  | value (reason : CasmEntryPointValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive CasmEntryPointPairsValid (d : Nat) : Nat → List CasmEntryPointField → CasmEntryPointValue → List w8 → List w8 → CasmEntryPointValue → Prop where
  | nil (seen : List CasmEntryPointField) (old : CasmEntryPointValue) (suffix : List w8) :
      CasmEntryPointPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : CasmEntryPointKeyValid key choice) (fresh : CasmEntryPointChoiceFresh seen choice)
      (valueValid : CasmEntryPointValueValid d choice old value (tail ++ suffix) middle)
      (rest : CasmEntryPointPairsValid d n (casmEntryPointChoiceSeen seen choice) middle tail suffix final) :
      CasmEntryPointPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial CasmEntryPoint and separates consumed bytes from the
    diagnostic position. Failed fields preserve the casmEntryPoint but may retain private partial children and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive CasmEntryPointPairsRejected (d : Nat) : Nat → List CasmEntryPointField → CasmEntryPointValue → List w8 → CasmEntryPointValue →
    Nat → Nat → Nat → CasmEntryPointFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      CasmEntryPointPairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : CasmEntryPointKeyValid key (some field)) (duplicate : field ∈ seen) :
      CasmEntryPointPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : CasmEntryPointKeyValid key choice)
      (fresh : CasmEntryPointChoiceFresh seen choice)
      (failure : CasmEntryPointValueRejected d choice old tail final consumed offset extraDepth reason) :
      CasmEntryPointPairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : CasmEntryPointKeyValid key choice) (fresh : CasmEntryPointChoiceFresh seen choice)
      (valueValid : CasmEntryPointValueValid d choice old value tail middle)
      (failure : CasmEntryPointPairsRejected d n (casmEntryPointChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      CasmEntryPointPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem casmEntryPointValue_classify (d : Nat) (choice : Option CasmEntryPointField) (old : CasmEntryPointValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ CasmEntryPointValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, CasmEntryPointValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases casmEntryPointField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨old, consumed, offset, extraDepth, _, .selected failure⟩

theorem casmEntryPointPairs_classify (d n : Nat) (seen : List CasmEntryPointField) (old : CasmEntryPointValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ CasmEntryPointPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, CasmEntryPointPairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases casmEntryPointKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : CasmEntryPointChoiceFresh seen choice
      · rcases casmEntryPointValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (casmEntryPointChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using CasmEntryPointPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [CasmEntryPointChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem casmEntryPointValue_nonempty (d : Nat) (choice : Option CasmEntryPointField) (old final : CasmEntryPointValue)
    (item rest : List w8) (valid : CasmEntryPointValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact casmEntryPointField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem casmEntryPointPairs_count_bound (d n : Nat) (seen : List CasmEntryPointField) (old final : CasmEntryPointValue)
    (item rest : List w8) (valid : CasmEntryPointPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty CasmEntryPointKeyMeaning _ _ keyValid
    have hv := casmEntryPointValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem casmEntryPointValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option CasmEntryPointField)
    (old final : CasmEntryPointValue) (wire : List w8) (reason : CasmEntryPointValueFailure)
    (failure : CasmEntryPointValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact casmEntryPointField_rejected_bounds _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem casmEntryPointValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option CasmEntryPointField)
    (old final : CasmEntryPointValue) (wire : List w8) (reason : CasmEntryPointValueFailure) (depth : d ≤ 32)
    (failure : CasmEntryPointValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact casmEntryPointField_rejected_depth _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem casmEntryPointPairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List CasmEntryPointField) (old final : CasmEntryPointValue)
    (wire : List w8) (reason : CasmEntryPointFailureReason)
    (failure : CasmEntryPointPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := casmEntryPointValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem casmEntryPointPairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List CasmEntryPointField) (old final : CasmEntryPointValue)
    (wire : List w8) (reason : CasmEntryPointFailureReason) (depth : d ≤ 32)
    (failure : CasmEntryPointPairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact casmEntryPointValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointPairsRejected_depth
