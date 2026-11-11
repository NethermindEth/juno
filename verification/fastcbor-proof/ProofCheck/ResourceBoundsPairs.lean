import ProofCheck.ResourceBoundsKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ResourceBoundsValueValid (d : Nat) : Option ResourceBoundsField → ResourceBoundsValue → List w8 → List w8 → ResourceBoundsValue → Prop where
  | selected {field old item rest final} (valid : ResourceBoundsFieldValid d field old item rest final) :
      ResourceBoundsValueValid d (some field) old item rest final
  | ignored {old : ResourceBoundsValue} {item rest} (valid : SkipValid d item) :
      ResourceBoundsValueValid d none old item rest old

inductive ResourceBoundsValueFailure where
  | field (reason : ResourceBoundsFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive ResourceBoundsValueRejected (d : Nat) : Option ResourceBoundsField → List w8 → Nat → ResourceBoundsValueFailure → Prop where
  | selected {field wire offset reason} (failure : ResourceBoundsFieldRejected d field wire offset reason) :
      ResourceBoundsValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      ResourceBoundsValueRejected d none wire offset (.ignored reason)

def ResourceBoundsChoiceFresh (seen : List ResourceBoundsField) : Option ResourceBoundsField → Prop
  | none => True
  | some field => field ∉ seen

def resourceBoundsChoiceSeen (seen : List ResourceBoundsField) : Option ResourceBoundsField → List ResourceBoundsField
  | none => seen
  | some field => field :: seen

inductive ResourceBoundsFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : ResourceBoundsField)
  | value (reason : ResourceBoundsValueFailure)

/-- Successful pairs retain the unread suffix after their field values.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive ResourceBoundsPairsValid (d : Nat) : Nat → List ResourceBoundsField → ResourceBoundsValue → List w8 → List w8 → ResourceBoundsValue → Prop where
  | nil (seen : List ResourceBoundsField) (old : ResourceBoundsValue) (suffix : List w8) :
      ResourceBoundsPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : ResourceBoundsKeyValid key choice) (fresh : ResourceBoundsChoiceFresh seen choice)
      (valueValid : ResourceBoundsValueValid d choice old value (tail ++ suffix) middle)
      (rest : ResourceBoundsPairsValid d n (resourceBoundsChoiceSeen seen choice) middle tail suffix final) :
      ResourceBoundsPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial ResourceBounds and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive ResourceBoundsPairsRejected (d : Nat) : Nat → List ResourceBoundsField → ResourceBoundsValue → List w8 → ResourceBoundsValue →
    Nat → Nat → ResourceBoundsFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      ResourceBoundsPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : ResourceBoundsKeyValid key (some field)) (duplicate : field ∈ seen) :
      ResourceBoundsPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : ResourceBoundsKeyValid key choice)
      (fresh : ResourceBoundsChoiceFresh seen choice) (failure : ResourceBoundsValueRejected d choice tail offset reason) :
      ResourceBoundsPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : ResourceBoundsKeyValid key choice) (fresh : ResourceBoundsChoiceFresh seen choice)
      (valueValid : ResourceBoundsValueValid d choice old value tail middle)
      (failure : ResourceBoundsPairsRejected d n (resourceBoundsChoiceSeen seen choice) middle tail final consumed offset reason) :
      ResourceBoundsPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem resourceBoundsValue_classify (d : Nat) (choice : Option ResourceBoundsField) (old : ResourceBoundsValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ResourceBoundsValueValid d choice old item rest final) ∨
    (∃ offset reason, ResourceBoundsValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases resourceBoundsField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem resourceBoundsPairs_classify (d n : Nat) (seen : List ResourceBoundsField) (old : ResourceBoundsValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ResourceBoundsPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, ResourceBoundsPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases resourceBoundsKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : ResourceBoundsChoiceFresh seen choice
      · rcases resourceBoundsValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (resourceBoundsChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using ResourceBoundsPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [ResourceBoundsChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem resourceBoundsValue_nonempty (d : Nat) (choice : Option ResourceBoundsField) (old final : ResourceBoundsValue)
    (item rest : List w8) (valid : ResourceBoundsValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact resourceBoundsField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem resourceBoundsPairs_count_bound (d n : Nat) (seen : List ResourceBoundsField) (old final : ResourceBoundsValue)
    (item rest : List w8) (valid : ResourceBoundsPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty ResourceBoundsKeyMeaning _ _ keyValid
    have hv := resourceBoundsValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem resourceBoundsValueRejected_bound (d offset : Nat) (choice : Option ResourceBoundsField) (wire : List w8)
    (reason : ResourceBoundsValueFailure) (failure : ResourceBoundsValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact resourceBoundsField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem resourceBoundsPairsRejected_bounds (d n consumed offset : Nat) (seen : List ResourceBoundsField) (old final : ResourceBoundsValue)
    (wire : List w8) (reason : ResourceBoundsFailureReason)
    (failure : ResourceBoundsPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := resourceBoundsValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsPairsRejected_bounds
