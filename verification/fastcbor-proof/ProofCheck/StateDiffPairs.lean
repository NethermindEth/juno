import ProofCheck.StateDiffKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive StateDiffValueValid (d : Nat) : Option StateDiffField → StateDiffValue → List w8 → List w8 → StateDiffValue → Prop where
  | selected {field old item rest final} (valid : StateDiffFieldValid d field old item rest final) :
      StateDiffValueValid d (some field) old item rest final
  | ignored {old : StateDiffValue} {item rest} (valid : SkipValid d item) :
      StateDiffValueValid d none old item rest old

inductive StateDiffValueFailure where
  | field (reason : StateDiffFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive StateDiffValueRejected (d : Nat) : Option StateDiffField → StateDiffValue → List w8 → StateDiffValue →
    Nat → Nat → Nat → StateDiffValueFailure → Prop where
  | selected {field old wire consumed offset extraDepth reason}
      (failure : StateDiffFieldRejected d field wire consumed offset extraDepth reason) :
      StateDiffValueRejected d (some field) old wire old consumed offset extraDepth (.field reason)
  | ignored {old : StateDiffValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      StateDiffValueRejected d none old wire old 0 offset 0 (.ignored reason)

def StateDiffChoiceFresh (seen : List StateDiffField) : Option StateDiffField → Prop
  | none => True
  | some field => field ∉ seen

def stateDiffChoiceSeen (seen : List StateDiffField) : Option StateDiffField → List StateDiffField
  | none => seen
  | some field => field :: seen

/-- Failure reasons distinguish opening, key, duplicate and value diagnostics. -/
inductive StateDiffFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : StateDiffField)
  | value (reason : StateDiffValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive StateDiffPairsValid (d : Nat) : Nat → List StateDiffField → StateDiffValue → List w8 → List w8 → StateDiffValue → Prop where
  | nil (seen : List StateDiffField) (old : StateDiffValue) (suffix : List w8) :
      StateDiffPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : StateDiffKeyValid key choice) (fresh : StateDiffChoiceFresh seen choice)
      (valueValid : StateDiffValueValid d choice old value (tail ++ suffix) middle)
      (rest : StateDiffPairsValid d n (stateDiffChoiceSeen seen choice) middle tail suffix final) :
      StateDiffPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial StateDiff and separates consumed bytes from the
    diagnostic position. Failed fields preserve the stateDiff but may retain private partial children and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive StateDiffPairsRejected (d : Nat) : Nat → List StateDiffField → StateDiffValue → List w8 → StateDiffValue →
    Nat → Nat → Nat → StateDiffFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      StateDiffPairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : StateDiffKeyValid key (some field)) (duplicate : field ∈ seen) :
      StateDiffPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : StateDiffKeyValid key choice)
      (fresh : StateDiffChoiceFresh seen choice)
      (failure : StateDiffValueRejected d choice old tail final consumed offset extraDepth reason) :
      StateDiffPairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : StateDiffKeyValid key choice) (fresh : StateDiffChoiceFresh seen choice)
      (valueValid : StateDiffValueValid d choice old value tail middle)
      (failure : StateDiffPairsRejected d n (stateDiffChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      StateDiffPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem stateDiffValue_classify (d : Nat) (choice : Option StateDiffField) (old : StateDiffValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ StateDiffValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, StateDiffValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases stateDiffField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨old, consumed, offset, extraDepth, _, .selected failure⟩

theorem stateDiffPairs_classify (d n : Nat) (seen : List StateDiffField) (old : StateDiffValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ StateDiffPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, StateDiffPairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases stateDiffKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : StateDiffChoiceFresh seen choice
      · rcases stateDiffValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (stateDiffChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using StateDiffPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [StateDiffChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem stateDiffValue_nonempty (d : Nat) (choice : Option StateDiffField) (old final : StateDiffValue)
    (item rest : List w8) (valid : StateDiffValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact stateDiffField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem stateDiffPairs_count_bound (d n : Nat) (seen : List StateDiffField) (old final : StateDiffValue)
    (item rest : List w8) (valid : StateDiffPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty StateDiffKeyMeaning _ _ keyValid
    have hv := stateDiffValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem stateDiffValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option StateDiffField)
    (old final : StateDiffValue) (wire : List w8) (reason : StateDiffValueFailure)
    (failure : StateDiffValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact stateDiffField_rejected_bounds _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem stateDiffValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option StateDiffField)
    (old final : StateDiffValue) (wire : List w8) (reason : StateDiffValueFailure) (depth : d ≤ 32)
    (failure : StateDiffValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact stateDiffField_rejected_depth _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem stateDiffPairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List StateDiffField) (old final : StateDiffValue)
    (wire : List w8) (reason : StateDiffFailureReason)
    (failure : StateDiffPairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := stateDiffValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem stateDiffPairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List StateDiffField) (old final : StateDiffValue)
    (wire : List w8) (reason : StateDiffFailureReason) (depth : d ≤ 32)
    (failure : StateDiffPairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact stateDiffValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPairsRejected_depth
