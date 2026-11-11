import ProofCheck.BuiltinsKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive BuiltinsValueValid (d : Nat) : Option BuiltinsField → BuiltinsValue → List w8 → List w8 → BuiltinsValue → Prop where
  | selected {field old item rest final} (valid : BuiltinsFieldValid d field old item rest final) :
      BuiltinsValueValid d (some field) old item rest final
  | ignored {old : BuiltinsValue} {item rest} (valid : SkipValid d item) :
      BuiltinsValueValid d none old item rest old

inductive BuiltinsValueFailure where
  | field (reason : BuiltinsFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive BuiltinsValueRejected (d : Nat) : Option BuiltinsField → List w8 → Nat → BuiltinsValueFailure → Prop where
  | selected {field wire offset reason} (failure : BuiltinsFieldRejected d field wire offset reason) :
      BuiltinsValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      BuiltinsValueRejected d none wire offset (.ignored reason)

def BuiltinsChoiceFresh (seen : List BuiltinsField) : Option BuiltinsField → Prop
  | none => True
  | some field => field ∉ seen

def builtinsChoiceSeen (seen : List BuiltinsField) : Option BuiltinsField → List BuiltinsField
  | none => seen
  | some field => field :: seen

inductive BuiltinsFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : BuiltinsField)
  | value (reason : BuiltinsValueFailure)

/-- Successful pairs retain the unread suffix after their scalar values.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive BuiltinsPairsValid (d : Nat) : Nat → List BuiltinsField → BuiltinsValue → List w8 → List w8 → BuiltinsValue → Prop where
  | nil (seen : List BuiltinsField) (old : BuiltinsValue) (suffix : List w8) :
      BuiltinsPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : BuiltinsKeyValid key choice) (fresh : BuiltinsChoiceFresh seen choice)
      (valueValid : BuiltinsValueValid d choice old value (tail ++ suffix) middle)
      (rest : BuiltinsPairsValid d n (builtinsChoiceSeen seen choice) middle tail suffix final) :
      BuiltinsPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial BuiltinInstanceCounter and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive BuiltinsPairsRejected (d : Nat) : Nat → List BuiltinsField → BuiltinsValue → List w8 → BuiltinsValue →
    Nat → Nat → BuiltinsFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      BuiltinsPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : BuiltinsKeyValid key (some field)) (duplicate : field ∈ seen) :
      BuiltinsPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : BuiltinsKeyValid key choice)
      (fresh : BuiltinsChoiceFresh seen choice) (failure : BuiltinsValueRejected d choice tail offset reason) :
      BuiltinsPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : BuiltinsKeyValid key choice) (fresh : BuiltinsChoiceFresh seen choice)
      (valueValid : BuiltinsValueValid d choice old value tail middle)
      (failure : BuiltinsPairsRejected d n (builtinsChoiceSeen seen choice) middle tail final consumed offset reason) :
      BuiltinsPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem builtinsValue_classify (d : Nat) (choice : Option BuiltinsField) (old : BuiltinsValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ BuiltinsValueValid d choice old item rest final) ∨
    (∃ offset reason, BuiltinsValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases builtinsField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem builtinsPairs_classify (d n : Nat) (seen : List BuiltinsField) (old : BuiltinsValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ BuiltinsPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, BuiltinsPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases builtinsKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : BuiltinsChoiceFresh seen choice
      · rcases builtinsValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (builtinsChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using BuiltinsPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [BuiltinsChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem builtinsValue_nonempty (d : Nat) (choice : Option BuiltinsField) (old final : BuiltinsValue)
    (item rest : List w8) (valid : BuiltinsValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact builtinsField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem builtinsPairs_count_bound (d n : Nat) (seen : List BuiltinsField) (old final : BuiltinsValue)
    (item rest : List w8) (valid : BuiltinsPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty BuiltinsKeyMeaning _ _ keyValid
    have hv := builtinsValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem builtinsValueRejected_bound (d offset : Nat) (choice : Option BuiltinsField) (wire : List w8)
    (reason : BuiltinsValueFailure) (failure : BuiltinsValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact builtinsField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem builtinsPairsRejected_bounds (d n consumed offset : Nat) (seen : List BuiltinsField) (old final : BuiltinsValue)
    (wire : List w8) (reason : BuiltinsFailureReason)
    (failure : BuiltinsPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := builtinsValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsPairsRejected_bounds
