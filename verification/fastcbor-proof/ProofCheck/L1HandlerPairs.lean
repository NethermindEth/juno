import ProofCheck.L1HandlerKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive L1HandlerValueValid (d : Nat) : Option L1HandlerField → L1HandlerValue → List w8 → List w8 → L1HandlerValue → Prop where
  | selected {field old item rest final} (valid : L1HandlerFieldValid d field old item rest final) :
      L1HandlerValueValid d (some field) old item rest final
  | ignored {old : L1HandlerValue} {item rest} (valid : SkipValid d item) :
      L1HandlerValueValid d none old item rest old

inductive L1HandlerValueFailure where
  | field (reason : L1HandlerFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive L1HandlerValueRejected (d : Nat) : Option L1HandlerField → List w8 → Nat → L1HandlerValueFailure → Prop where
  | selected {field wire offset reason} (failure : L1HandlerFieldRejected d field wire offset reason) :
      L1HandlerValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      L1HandlerValueRejected d none wire offset (.ignored reason)

def L1HandlerChoiceFresh (seen : List L1HandlerField) : Option L1HandlerField → Prop
  | none => True
  | some field => field ∉ seen

def l1HandlerChoiceSeen (seen : List L1HandlerField) : Option L1HandlerField → List L1HandlerField
  | none => seen
  | some field => field :: seen

inductive L1HandlerFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : L1HandlerField)
  | value (reason : L1HandlerValueFailure)

/-- Successful pairs retain the actual suffix in every nested slice guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive L1HandlerPairsValid (d : Nat) : Nat → List L1HandlerField → L1HandlerValue → List w8 → List w8 → L1HandlerValue → Prop where
  | nil (seen : List L1HandlerField) (old : L1HandlerValue) (suffix : List w8) :
      L1HandlerPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : L1HandlerKeyValid key choice) (fresh : L1HandlerChoiceFresh seen choice)
      (valueValid : L1HandlerValueValid d choice old value (tail ++ suffix) middle)
      (rest : L1HandlerPairsValid d n (l1HandlerChoiceSeen seen choice) middle tail suffix final) :
      L1HandlerPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial L1HandlerTransaction and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive L1HandlerPairsRejected (d : Nat) : Nat → List L1HandlerField → L1HandlerValue → List w8 → L1HandlerValue →
    Nat → Nat → L1HandlerFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      L1HandlerPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : L1HandlerKeyValid key (some field)) (duplicate : field ∈ seen) :
      L1HandlerPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : L1HandlerKeyValid key choice)
      (fresh : L1HandlerChoiceFresh seen choice) (failure : L1HandlerValueRejected d choice tail offset reason) :
      L1HandlerPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : L1HandlerKeyValid key choice) (fresh : L1HandlerChoiceFresh seen choice)
      (valueValid : L1HandlerValueValid d choice old value tail middle)
      (failure : L1HandlerPairsRejected d n (l1HandlerChoiceSeen seen choice) middle tail final consumed offset reason) :
      L1HandlerPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem l1HandlerValue_classify (d : Nat) (choice : Option L1HandlerField) (old : L1HandlerValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L1HandlerValueValid d choice old item rest final) ∨
    (∃ offset reason, L1HandlerValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases l1HandlerField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem l1HandlerPairs_classify (d n : Nat) (seen : List L1HandlerField) (old : L1HandlerValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L1HandlerPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, L1HandlerPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases l1HandlerKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : L1HandlerChoiceFresh seen choice
      · rcases l1HandlerValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (l1HandlerChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using L1HandlerPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [L1HandlerChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem l1HandlerValue_nonempty (d : Nat) (choice : Option L1HandlerField) (old final : L1HandlerValue)
    (item rest : List w8) (valid : L1HandlerValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact l1HandlerField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem l1HandlerPairs_count_bound (d n : Nat) (seen : List L1HandlerField) (old final : L1HandlerValue)
    (item rest : List w8) (valid : L1HandlerPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty L1HandlerKeyMeaning _ _ keyValid
    have hv := l1HandlerValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem l1HandlerValueRejected_bound (d offset : Nat) (choice : Option L1HandlerField) (wire : List w8)
    (reason : L1HandlerValueFailure) (failure : L1HandlerValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact l1HandlerField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem l1HandlerPairsRejected_bounds (d n consumed offset : Nat) (seen : List L1HandlerField) (old final : L1HandlerValue)
    (wire : List w8) (reason : L1HandlerFailureReason)
    (failure : L1HandlerPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := l1HandlerValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerPairsRejected_bounds
