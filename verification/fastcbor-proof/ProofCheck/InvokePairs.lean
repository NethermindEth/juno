import ProofCheck.InvokeKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive InvokeValueValid (d : Nat) : Option InvokeField → InvokeValue → List w8 → List w8 → InvokeValue → Prop where
  | selected {field old item rest final} (valid : InvokeFieldValid d field old item rest final) :
      InvokeValueValid d (some field) old item rest final
  | ignored {old : InvokeValue} {item rest} (valid : SkipValid d item) :
      InvokeValueValid d none old item rest old

inductive InvokeValueFailure where
  | field (reason : InvokeFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive InvokeValueRejected (d : Nat) : Option InvokeField → InvokeValue → List w8 → InvokeValue →
    Nat → Nat → Nat → InvokeValueFailure → Prop where
  | selected {field old wire consumed offset extraDepth reason}
      (failure : InvokeFieldRejected d field wire consumed offset extraDepth reason) :
      InvokeValueRejected d (some field) old wire old consumed offset extraDepth (.field reason)
  | ignored {old : InvokeValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      InvokeValueRejected d none old wire old 0 offset 0 (.ignored reason)

def InvokeChoiceFresh (seen : List InvokeField) : Option InvokeField → Prop
  | none => True
  | some field => field ∉ seen

def invokeChoiceSeen (seen : List InvokeField) : Option InvokeField → List InvokeField
  | none => seen
  | some field => field :: seen

/-- Numeric failure positions are structured diagnostic offsets except for
    bare DA-mode overflow, where the field contract records only the consumed boundary. -/
inductive InvokeFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : InvokeField)
  | value (reason : InvokeValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive InvokePairsValid (d : Nat) : Nat → List InvokeField → InvokeValue → List w8 → List w8 → InvokeValue → Prop where
  | nil (seen : List InvokeField) (old : InvokeValue) (suffix : List w8) :
      InvokePairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : InvokeKeyValid key choice) (fresh : InvokeChoiceFresh seen choice)
      (valueValid : InvokeValueValid d choice old value (tail ++ suffix) middle)
      (rest : InvokePairsValid d n (invokeChoiceSeen seen choice) middle tail suffix final) :
      InvokePairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial InvokeTransaction and separates consumed bytes from the
    diagnostic position. Failed fields preserve the invoke but may retain private partial children and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive InvokePairsRejected (d : Nat) : Nat → List InvokeField → InvokeValue → List w8 → InvokeValue →
    Nat → Nat → Nat → InvokeFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      InvokePairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : InvokeKeyValid key (some field)) (duplicate : field ∈ seen) :
      InvokePairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : InvokeKeyValid key choice)
      (fresh : InvokeChoiceFresh seen choice)
      (failure : InvokeValueRejected d choice old tail final consumed offset extraDepth reason) :
      InvokePairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : InvokeKeyValid key choice) (fresh : InvokeChoiceFresh seen choice)
      (valueValid : InvokeValueValid d choice old value tail middle)
      (failure : InvokePairsRejected d n (invokeChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      InvokePairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem invokeValue_classify (d : Nat) (choice : Option InvokeField) (old : InvokeValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ InvokeValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, InvokeValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases invokeField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨old, consumed, offset, extraDepth, _, .selected failure⟩

theorem invokePairs_classify (d n : Nat) (seen : List InvokeField) (old : InvokeValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ InvokePairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, InvokePairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases invokeKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : InvokeChoiceFresh seen choice
      · rcases invokeValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (invokeChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using InvokePairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [InvokeChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem invokeValue_nonempty (d : Nat) (choice : Option InvokeField) (old final : InvokeValue)
    (item rest : List w8) (valid : InvokeValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact invokeField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem invokePairs_count_bound (d n : Nat) (seen : List InvokeField) (old final : InvokeValue)
    (item rest : List w8) (valid : InvokePairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty InvokeKeyMeaning _ _ keyValid
    have hv := invokeValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem invokeValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option InvokeField)
    (old final : InvokeValue) (wire : List w8) (reason : InvokeValueFailure)
    (failure : InvokeValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact invokeField_rejected_bounds _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem invokeValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option InvokeField)
    (old final : InvokeValue) (wire : List w8) (reason : InvokeValueFailure) (depth : d ≤ 32)
    (failure : InvokeValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact invokeField_rejected_depth _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem invokePairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List InvokeField) (old final : InvokeValue)
    (wire : List w8) (reason : InvokeFailureReason)
    (failure : InvokePairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := invokeValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem invokePairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List InvokeField) (old final : InvokeValue)
    (wire : List w8) (reason : InvokeFailureReason) (depth : d ≤ 32)
    (failure : InvokePairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact invokeValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.invokePairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.invokePairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.invokePairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.invokePairsRejected_depth
