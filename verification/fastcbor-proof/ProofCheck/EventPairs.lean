import ProofCheck.EventKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive EventValueValid (d : Nat) : Option EventField → EventValue → List w8 → List w8 → EventValue → Prop where
  | selected {field old item rest final} (valid : EventFieldValid d field old item rest final) :
      EventValueValid d (some field) old item rest final
  | ignored {old : EventValue} {item rest} (valid : SkipValid d item) :
      EventValueValid d none old item rest old

inductive EventValueFailure where
  | field (reason : EventFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive EventValueRejected (d : Nat) : Option EventField → List w8 → Nat → EventValueFailure → Prop where
  | selected {field wire offset reason} (failure : EventFieldRejected d field wire offset reason) :
      EventValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      EventValueRejected d none wire offset (.ignored reason)

def EventChoiceFresh (seen : List EventField) : Option EventField → Prop
  | none => True
  | some field => field ∉ seen

def eventChoiceSeen (seen : List EventField) : Option EventField → List EventField
  | none => seen
  | some field => field :: seen

inductive EventFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : EventField)
  | value (reason : EventValueFailure)

/-- Successful pairs retain the actual suffix in every nested slice guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive EventPairsValid (d : Nat) : Nat → List EventField → EventValue → List w8 → List w8 → EventValue → Prop where
  | nil (seen : List EventField) (old : EventValue) (suffix : List w8) :
      EventPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : EventKeyValid key choice) (fresh : EventChoiceFresh seen choice)
      (valueValid : EventValueValid d choice old value (tail ++ suffix) middle)
      (rest : EventPairsValid d n (eventChoiceSeen seen choice) middle tail suffix final) :
      EventPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial Event and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive EventPairsRejected (d : Nat) : Nat → List EventField → EventValue → List w8 → EventValue →
    Nat → Nat → EventFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      EventPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : EventKeyValid key (some field)) (duplicate : field ∈ seen) :
      EventPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : EventKeyValid key choice)
      (fresh : EventChoiceFresh seen choice) (failure : EventValueRejected d choice tail offset reason) :
      EventPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : EventKeyValid key choice) (fresh : EventChoiceFresh seen choice)
      (valueValid : EventValueValid d choice old value tail middle)
      (failure : EventPairsRejected d n (eventChoiceSeen seen choice) middle tail final consumed offset reason) :
      EventPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem eventValue_classify (d : Nat) (choice : Option EventField) (old : EventValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ EventValueValid d choice old item rest final) ∨
    (∃ offset reason, EventValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases eventField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem eventPairs_classify (d n : Nat) (seen : List EventField) (old : EventValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ EventPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, EventPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases eventKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : EventChoiceFresh seen choice
      · rcases eventValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (eventChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using EventPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [EventChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem eventValue_nonempty (d : Nat) (choice : Option EventField) (old final : EventValue)
    (item rest : List w8) (valid : EventValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact eventField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem eventPairs_count_bound (d n : Nat) (seen : List EventField) (old final : EventValue)
    (item rest : List w8) (valid : EventPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty EventKeyMeaning _ _ keyValid
    have hv := eventValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem eventValueRejected_bound (d offset : Nat) (choice : Option EventField) (wire : List w8)
    (reason : EventValueFailure) (failure : EventValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact eventField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem eventPairsRejected_bounds (d n consumed offset : Nat) (seen : List EventField) (old final : EventValue)
    (wire : List w8) (reason : EventFailureReason)
    (failure : EventPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := eventValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.eventValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.eventValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.eventPairsRejected_bounds
