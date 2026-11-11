import ProofCheck.AvailabilityKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive AvailabilityValueValid (d : Nat) : Option AvailabilityField → AvailabilityValue → List w8 → List w8 → AvailabilityValue → Prop where
  | selected {field old item rest final} (valid : AvailabilityFieldValid d field old item rest final) :
      AvailabilityValueValid d (some field) old item rest final
  | ignored {old : AvailabilityValue} {item rest} (valid : SkipValid d item) :
      AvailabilityValueValid d none old item rest old

inductive AvailabilityValueFailure where
  | field (reason : AvailabilityFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive AvailabilityValueRejected (d : Nat) : Option AvailabilityField → List w8 → Nat → AvailabilityValueFailure → Prop where
  | selected {field wire offset reason} (failure : AvailabilityFieldRejected d field wire offset reason) :
      AvailabilityValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      AvailabilityValueRejected d none wire offset (.ignored reason)

def AvailabilityChoiceFresh (seen : List AvailabilityField) : Option AvailabilityField → Prop
  | none => True
  | some field => field ∉ seen

def availabilityChoiceSeen (seen : List AvailabilityField) : Option AvailabilityField → List AvailabilityField
  | none => seen
  | some field => field :: seen

inductive AvailabilityFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : AvailabilityField)
  | value (reason : AvailabilityValueFailure)

/-- Successful pairs retain the unread suffix after their scalar values.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive AvailabilityPairsValid (d : Nat) : Nat → List AvailabilityField → AvailabilityValue → List w8 → List w8 → AvailabilityValue → Prop where
  | nil (seen : List AvailabilityField) (old : AvailabilityValue) (suffix : List w8) :
      AvailabilityPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : AvailabilityKeyValid key choice) (fresh : AvailabilityChoiceFresh seen choice)
      (valueValid : AvailabilityValueValid d choice old value (tail ++ suffix) middle)
      (rest : AvailabilityPairsValid d n (availabilityChoiceSeen seen choice) middle tail suffix final) :
      AvailabilityPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial DataAvailability and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive AvailabilityPairsRejected (d : Nat) : Nat → List AvailabilityField → AvailabilityValue → List w8 → AvailabilityValue →
    Nat → Nat → AvailabilityFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      AvailabilityPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : AvailabilityKeyValid key (some field)) (duplicate : field ∈ seen) :
      AvailabilityPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : AvailabilityKeyValid key choice)
      (fresh : AvailabilityChoiceFresh seen choice) (failure : AvailabilityValueRejected d choice tail offset reason) :
      AvailabilityPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : AvailabilityKeyValid key choice) (fresh : AvailabilityChoiceFresh seen choice)
      (valueValid : AvailabilityValueValid d choice old value tail middle)
      (failure : AvailabilityPairsRejected d n (availabilityChoiceSeen seen choice) middle tail final consumed offset reason) :
      AvailabilityPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem availabilityValue_classify (d : Nat) (choice : Option AvailabilityField) (old : AvailabilityValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ AvailabilityValueValid d choice old item rest final) ∨
    (∃ offset reason, AvailabilityValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases availabilityField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem availabilityPairs_classify (d n : Nat) (seen : List AvailabilityField) (old : AvailabilityValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ AvailabilityPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, AvailabilityPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases availabilityKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : AvailabilityChoiceFresh seen choice
      · rcases availabilityValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (availabilityChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using AvailabilityPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [AvailabilityChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem availabilityValue_nonempty (d : Nat) (choice : Option AvailabilityField) (old final : AvailabilityValue)
    (item rest : List w8) (valid : AvailabilityValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact availabilityField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem availabilityPairs_count_bound (d n : Nat) (seen : List AvailabilityField) (old final : AvailabilityValue)
    (item rest : List w8) (valid : AvailabilityPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty AvailabilityKeyMeaning _ _ keyValid
    have hv := availabilityValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem availabilityValueRejected_bound (d offset : Nat) (choice : Option AvailabilityField) (wire : List w8)
    (reason : AvailabilityValueFailure) (failure : AvailabilityValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact availabilityField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem availabilityPairsRejected_bounds (d n consumed offset : Nat) (seen : List AvailabilityField) (old final : AvailabilityValue)
    (wire : List w8) (reason : AvailabilityFailureReason)
    (failure : AvailabilityPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := availabilityValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityPairsRejected_bounds
