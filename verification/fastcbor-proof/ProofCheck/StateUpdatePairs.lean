import ProofCheck.StateUpdateKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive StateUpdateValueValid (d : Nat) : Option StateUpdateField → StateUpdateValue → List w8 → List w8 → StateUpdateValue → Prop where
  | selected {field old item rest final} (valid : StateUpdateFieldValid d field old item rest final) :
      StateUpdateValueValid d (some field) old item rest final
  | ignored {old : StateUpdateValue} {item rest} (valid : SkipValid d item) :
      StateUpdateValueValid d none old item rest old

inductive StateUpdateValueFailure where
  | field (reason : StateUpdateFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive StateUpdateValueRejected (d : Nat) : Option StateUpdateField → StateUpdateValue → List w8 → StateUpdateValue →
    Nat → Nat → Nat → StateUpdateValueFailure → Prop where
  | selected {field old wire consumed offset extraDepth reason}
      (failure : StateUpdateFieldRejected d field wire consumed offset extraDepth reason) :
      StateUpdateValueRejected d (some field) old wire old consumed offset extraDepth (.field reason)
  | ignored {old : StateUpdateValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      StateUpdateValueRejected d none old wire old 0 offset 0 (.ignored reason)

def StateUpdateChoiceFresh (seen : List StateUpdateField) : Option StateUpdateField → Prop
  | none => True
  | some field => field ∉ seen

def stateUpdateChoiceSeen (seen : List StateUpdateField) : Option StateUpdateField → List StateUpdateField
  | none => seen
  | some field => field :: seen

/-- Failure reasons distinguish opening, key, duplicate and value diagnostics. -/
inductive StateUpdateFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : StateUpdateField)
  | value (reason : StateUpdateValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive StateUpdatePairsValid (d : Nat) : Nat → List StateUpdateField → StateUpdateValue → List w8 → List w8 → StateUpdateValue → Prop where
  | nil (seen : List StateUpdateField) (old : StateUpdateValue) (suffix : List w8) :
      StateUpdatePairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : StateUpdateKeyValid key choice) (fresh : StateUpdateChoiceFresh seen choice)
      (valueValid : StateUpdateValueValid d choice old value (tail ++ suffix) middle)
      (rest : StateUpdatePairsValid d n (stateUpdateChoiceSeen seen choice) middle tail suffix final) :
      StateUpdatePairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial StateUpdate and separates consumed bytes from the
    diagnostic position. Failed fields preserve the stateUpdate but may retain private partial children and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive StateUpdatePairsRejected (d : Nat) : Nat → List StateUpdateField → StateUpdateValue → List w8 → StateUpdateValue →
    Nat → Nat → Nat → StateUpdateFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      StateUpdatePairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : StateUpdateKeyValid key (some field)) (duplicate : field ∈ seen) :
      StateUpdatePairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : StateUpdateKeyValid key choice)
      (fresh : StateUpdateChoiceFresh seen choice)
      (failure : StateUpdateValueRejected d choice old tail final consumed offset extraDepth reason) :
      StateUpdatePairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : StateUpdateKeyValid key choice) (fresh : StateUpdateChoiceFresh seen choice)
      (valueValid : StateUpdateValueValid d choice old value tail middle)
      (failure : StateUpdatePairsRejected d n (stateUpdateChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      StateUpdatePairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem stateUpdateValue_classify (d : Nat) (choice : Option StateUpdateField) (old : StateUpdateValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ StateUpdateValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, StateUpdateValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases stateUpdateField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨old, consumed, offset, extraDepth, _, .selected failure⟩

theorem stateUpdatePairs_classify (d n : Nat) (seen : List StateUpdateField) (old : StateUpdateValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ StateUpdatePairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, StateUpdatePairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases stateUpdateKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : StateUpdateChoiceFresh seen choice
      · rcases stateUpdateValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (stateUpdateChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using StateUpdatePairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [StateUpdateChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem stateUpdateValue_nonempty (d : Nat) (choice : Option StateUpdateField) (old final : StateUpdateValue)
    (item rest : List w8) (valid : StateUpdateValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact stateUpdateField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem stateUpdatePairs_count_bound (d n : Nat) (seen : List StateUpdateField) (old final : StateUpdateValue)
    (item rest : List w8) (valid : StateUpdatePairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty StateUpdateKeyMeaning _ _ keyValid
    have hv := stateUpdateValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem stateUpdateValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option StateUpdateField)
    (old final : StateUpdateValue) (wire : List w8) (reason : StateUpdateValueFailure)
    (failure : StateUpdateValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact stateUpdateField_rejected_bounds _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem stateUpdateValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option StateUpdateField)
    (old final : StateUpdateValue) (wire : List w8) (reason : StateUpdateValueFailure) (depth : d ≤ 32)
    (failure : StateUpdateValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact stateUpdateField_rejected_depth _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem stateUpdatePairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List StateUpdateField) (old final : StateUpdateValue)
    (wire : List w8) (reason : StateUpdateFailureReason)
    (failure : StateUpdatePairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := stateUpdateValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem stateUpdatePairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List StateUpdateField) (old final : StateUpdateValue)
    (wire : List w8) (reason : StateUpdateFailureReason) (depth : d ≤ 32)
    (failure : StateUpdatePairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact stateUpdateValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdatePairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdatePairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdatePairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdatePairsRejected_depth
