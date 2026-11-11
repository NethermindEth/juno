import ProofCheck.CasmClassKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive CasmClassValueValid (d : Nat) : Option CasmClassField → CasmClassValue → List w8 → List w8 → CasmClassValue → Prop where
  | selected {field old item rest final} (valid : CasmClassFieldValid d field old item rest final) :
      CasmClassValueValid d (some field) old item rest final
  | ignored {old : CasmClassValue} {item rest} (valid : SkipValid d item) :
      CasmClassValueValid d none old item rest old

inductive CasmClassValueFailure where
  | field (reason : CasmClassFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive CasmClassValueRejected (d : Nat) : Option CasmClassField → CasmClassValue → List w8 → CasmClassValue →
    Nat → Nat → Nat → CasmClassValueFailure → Prop where
  | selected {field old wire final consumed offset extraDepth reason}
      (failure : CasmClassFieldRejected d field old wire final consumed offset extraDepth reason) :
      CasmClassValueRejected d (some field) old wire final consumed offset extraDepth (.field reason)
  | ignored {old : CasmClassValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      CasmClassValueRejected d none old wire old 0 offset 0 (.ignored reason)

def CasmClassChoiceFresh (seen : List CasmClassField) : Option CasmClassField → Prop
  | none => True
  | some field => field ∉ seen

def casmClassChoiceSeen (seen : List CasmClassField) : Option CasmClassField → List CasmClassField
  | none => seen
  | some field => field :: seen

inductive CasmClassFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : CasmClassField)
  | value (reason : CasmClassValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive CasmClassPairsValid (d : Nat) : Nat → List CasmClassField → CasmClassValue → List w8 → List w8 → CasmClassValue → Prop where
  | nil (seen : List CasmClassField) (old : CasmClassValue) (suffix : List w8) :
      CasmClassPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : CasmClassKeyValid key choice) (fresh : CasmClassChoiceFresh seen choice)
      (valueValid : CasmClassValueValid d choice old value (tail ++ suffix) middle)
      (rest : CasmClassPairsValid d n (casmClassChoiceSeen seen choice) middle tail suffix final) :
      CasmClassPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial CasmClass and separates consumed bytes from the
    diagnostic position. Nested field failures retain partial values and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive CasmClassPairsRejected (d : Nat) : Nat → List CasmClassField → CasmClassValue → List w8 → CasmClassValue →
    Nat → Nat → Nat → CasmClassFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      CasmClassPairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : CasmClassKeyValid key (some field)) (duplicate : field ∈ seen) :
      CasmClassPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : CasmClassKeyValid key choice)
      (fresh : CasmClassChoiceFresh seen choice)
      (failure : CasmClassValueRejected d choice old tail final consumed offset extraDepth reason) :
      CasmClassPairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : CasmClassKeyValid key choice) (fresh : CasmClassChoiceFresh seen choice)
      (valueValid : CasmClassValueValid d choice old value tail middle)
      (failure : CasmClassPairsRejected d n (casmClassChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      CasmClassPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem casmClassValue_classify (d : Nat) (choice : Option CasmClassField) (old : CasmClassValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ CasmClassValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, CasmClassValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases casmClassField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨final, consumed, offset, extraDepth, _, .selected failure⟩

theorem casmClassPairs_classify (d n : Nat) (seen : List CasmClassField) (old : CasmClassValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ CasmClassPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, CasmClassPairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases casmClassKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : CasmClassChoiceFresh seen choice
      · rcases casmClassValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (casmClassChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using CasmClassPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [CasmClassChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem casmClassValue_nonempty (d : Nat) (choice : Option CasmClassField) (old final : CasmClassValue)
    (item rest : List w8) (valid : CasmClassValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact casmClassField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem casmClassPairs_count_bound (d n : Nat) (seen : List CasmClassField) (old final : CasmClassValue)
    (item rest : List w8) (valid : CasmClassPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty CasmClassKeyMeaning _ _ keyValid
    have hv := casmClassValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem casmClassValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option CasmClassField)
    (old final : CasmClassValue) (wire : List w8) (reason : CasmClassValueFailure)
    (failure : CasmClassValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact casmClassField_rejected_bounds _ _ _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem casmClassValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option CasmClassField)
    (old final : CasmClassValue) (wire : List w8) (reason : CasmClassValueFailure) (depth : d ≤ 32)
    (failure : CasmClassValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact casmClassField_rejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem casmClassPairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List CasmClassField) (old final : CasmClassValue)
    (wire : List w8) (reason : CasmClassFailureReason)
    (failure : CasmClassPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := casmClassValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem casmClassPairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List CasmClassField) (old final : CasmClassValue)
    (wire : List w8) (reason : CasmClassFailureReason) (depth : d ≤ 32)
    (failure : CasmClassPairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact casmClassValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassPairsRejected_depth
