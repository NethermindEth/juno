import ProofCheck.DeprecatedClassKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive DeprecatedClassValueValid (d : Nat) : Option DeprecatedClassField → DeprecatedClassValue → List w8 → List w8 → DeprecatedClassValue → Prop where
  | selected {field old item rest final} (valid : DeprecatedClassFieldValid d field old item rest final) :
      DeprecatedClassValueValid d (some field) old item rest final
  | ignored {old : DeprecatedClassValue} {item rest} (valid : SkipValid d item) :
      DeprecatedClassValueValid d none old item rest old

inductive DeprecatedClassValueFailure where
  | field (reason : DeprecatedClassFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive DeprecatedClassValueRejected (d : Nat) : Option DeprecatedClassField → DeprecatedClassValue → List w8 → DeprecatedClassValue →
    Nat → Nat → Nat → DeprecatedClassValueFailure → Prop where
  | selected {field old wire consumed offset extraDepth reason}
      (failure : DeprecatedClassFieldRejected d field wire consumed offset extraDepth reason) :
      DeprecatedClassValueRejected d (some field) old wire old consumed offset extraDepth (.field reason)
  | ignored {old : DeprecatedClassValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      DeprecatedClassValueRejected d none old wire old 0 offset 0 (.ignored reason)

def DeprecatedClassChoiceFresh (seen : List DeprecatedClassField) : Option DeprecatedClassField → Prop
  | none => True
  | some field => field ∉ seen

def deprecatedClassChoiceSeen (seen : List DeprecatedClassField) : Option DeprecatedClassField → List DeprecatedClassField
  | none => seen
  | some field => field :: seen

/-- Failure reasons distinguish opening, key, duplicate and value diagnostics. -/
inductive DeprecatedClassFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : DeprecatedClassField)
  | value (reason : DeprecatedClassValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive DeprecatedClassPairsValid (d : Nat) : Nat → List DeprecatedClassField → DeprecatedClassValue → List w8 → List w8 → DeprecatedClassValue → Prop where
  | nil (seen : List DeprecatedClassField) (old : DeprecatedClassValue) (suffix : List w8) :
      DeprecatedClassPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : DeprecatedClassKeyValid key choice) (fresh : DeprecatedClassChoiceFresh seen choice)
      (valueValid : DeprecatedClassValueValid d choice old value (tail ++ suffix) middle)
      (rest : DeprecatedClassPairsValid d n (deprecatedClassChoiceSeen seen choice) middle tail suffix final) :
      DeprecatedClassPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial DeprecatedClass and separates consumed bytes from the
    diagnostic position. Failed fields preserve the deprecatedClass but may retain private partial children and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive DeprecatedClassPairsRejected (d : Nat) : Nat → List DeprecatedClassField → DeprecatedClassValue → List w8 → DeprecatedClassValue →
    Nat → Nat → Nat → DeprecatedClassFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      DeprecatedClassPairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : DeprecatedClassKeyValid key (some field)) (duplicate : field ∈ seen) :
      DeprecatedClassPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : DeprecatedClassKeyValid key choice)
      (fresh : DeprecatedClassChoiceFresh seen choice)
      (failure : DeprecatedClassValueRejected d choice old tail final consumed offset extraDepth reason) :
      DeprecatedClassPairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : DeprecatedClassKeyValid key choice) (fresh : DeprecatedClassChoiceFresh seen choice)
      (valueValid : DeprecatedClassValueValid d choice old value tail middle)
      (failure : DeprecatedClassPairsRejected d n (deprecatedClassChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      DeprecatedClassPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem deprecatedClassValue_classify (d : Nat) (choice : Option DeprecatedClassField) (old : DeprecatedClassValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeprecatedClassValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, DeprecatedClassValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases deprecatedClassField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨old, consumed, offset, extraDepth, _, .selected failure⟩

theorem deprecatedClassPairs_classify (d n : Nat) (seen : List DeprecatedClassField) (old : DeprecatedClassValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeprecatedClassPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, DeprecatedClassPairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases deprecatedClassKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : DeprecatedClassChoiceFresh seen choice
      · rcases deprecatedClassValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (deprecatedClassChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using DeprecatedClassPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [DeprecatedClassChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem deprecatedClassValue_nonempty (d : Nat) (choice : Option DeprecatedClassField) (old final : DeprecatedClassValue)
    (item rest : List w8) (valid : DeprecatedClassValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact deprecatedClassField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem deprecatedClassPairs_count_bound (d n : Nat) (seen : List DeprecatedClassField) (old final : DeprecatedClassValue)
    (item rest : List w8) (valid : DeprecatedClassPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty DeprecatedClassKeyMeaning _ _ keyValid
    have hv := deprecatedClassValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem deprecatedClassValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option DeprecatedClassField)
    (old final : DeprecatedClassValue) (wire : List w8) (reason : DeprecatedClassValueFailure)
    (failure : DeprecatedClassValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact deprecatedClassField_rejected_bounds _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem deprecatedClassValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option DeprecatedClassField)
    (old final : DeprecatedClassValue) (wire : List w8) (reason : DeprecatedClassValueFailure) (depth : d ≤ 32)
    (failure : DeprecatedClassValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact deprecatedClassField_rejected_depth _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem deprecatedClassPairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List DeprecatedClassField) (old final : DeprecatedClassValue)
    (wire : List w8) (reason : DeprecatedClassFailureReason)
    (failure : DeprecatedClassPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := deprecatedClassValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem deprecatedClassPairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List DeprecatedClassField) (old final : DeprecatedClassValue)
    (wire : List w8) (reason : DeprecatedClassFailureReason) (depth : d ≤ 32)
    (failure : DeprecatedClassPairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact deprecatedClassValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassPairsRejected_depth
