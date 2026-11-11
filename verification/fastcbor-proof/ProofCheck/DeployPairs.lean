import ProofCheck.DeployKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive DeployValueValid (d : Nat) : Option DeployField → DeployValue → List w8 → List w8 → DeployValue → Prop where
  | selected {field old item rest final} (valid : DeployFieldValid d field old item rest final) :
      DeployValueValid d (some field) old item rest final
  | ignored {old : DeployValue} {item rest} (valid : SkipValid d item) :
      DeployValueValid d none old item rest old

inductive DeployValueFailure where
  | field (reason : DeployFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive DeployValueRejected (d : Nat) : Option DeployField → List w8 → Nat → DeployValueFailure → Prop where
  | selected {field wire offset reason} (failure : DeployFieldRejected d field wire offset reason) :
      DeployValueRejected d (some field) wire offset (.field reason)
  | ignored {wire offset reason} (failure : SkipFailure d wire offset reason) :
      DeployValueRejected d none wire offset (.ignored reason)

def DeployChoiceFresh (seen : List DeployField) : Option DeployField → Prop
  | none => True
  | some field => field ∉ seen

def deployChoiceSeen (seen : List DeployField) : Option DeployField → List DeployField
  | none => seen
  | some field => field :: seen

inductive DeployFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : DeployField)
  | value (reason : DeployValueFailure)

/-- Successful pairs retain the actual suffix in every nested slice guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive DeployPairsValid (d : Nat) : Nat → List DeployField → DeployValue → List w8 → List w8 → DeployValue → Prop where
  | nil (seen : List DeployField) (old : DeployValue) (suffix : List w8) :
      DeployPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : DeployKeyValid key choice) (fresh : DeployChoiceFresh seen choice)
      (valueValid : DeployValueValid d choice old value (tail ++ suffix) middle)
      (rest : DeployPairsValid d n (deployChoiceSeen seen choice) middle tail suffix final) :
      DeployPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial DeployTransaction and separates consumed bytes from the
    diagnostic position. Failed fields and ignored values roll back themselves;
    a duplicate is rejected immediately after its key is consumed. -/
inductive DeployPairsRejected (d : Nat) : Nat → List DeployField → DeployValue → List w8 → DeployValue →
    Nat → Nat → DeployFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      DeployPairsRejected d (n + 1) seen old wire old 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : DeployKeyValid key (some field)) (duplicate : field ∈ seen) :
      DeployPairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length (.duplicate field)
  | value {n seen choice old key tail offset reason} (keyValid : DeployKeyValid key choice)
      (fresh : DeployChoiceFresh seen choice) (failure : DeployValueRejected d choice tail offset reason) :
      DeployPairsRejected d (n + 1) seen old (key ++ tail) old key.length (key.length + offset) (.value reason)
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : DeployKeyValid key choice) (fresh : DeployChoiceFresh seen choice)
      (valueValid : DeployValueValid d choice old value tail middle)
      (failure : DeployPairsRejected d n (deployChoiceSeen seen choice) middle tail final consumed offset reason) :
      DeployPairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem deployValue_classify (d : Nat) (choice : Option DeployField) (old : DeployValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeployValueValid d choice old item rest final) ∨
    (∃ offset reason, DeployValueRejected d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨offset, _, .ignored failure⟩
  | some field =>
    rcases deployField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨offset, _, .selected failure⟩

theorem deployPairs_classify (d n : Nat) (seen : List DeployField) (old : DeployValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeployPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset reason, DeployPairsRejected d n seen old wire final consumed offset reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases deployKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : DeployChoiceFresh seen choice
      · rcases deployValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (deployChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using DeployPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨old, key.length, key.length + offset, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, _,
            .duplicate keyValid (by simpa only [DeployChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, _, .key failure⟩

theorem deployValue_nonempty (d : Nat) (choice : Option DeployField) (old final : DeployValue)
    (item rest : List w8) (valid : DeployValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact deployField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem deployPairs_count_bound (d n : Nat) (seen : List DeployField) (old final : DeployValue)
    (item rest : List w8) (valid : DeployPairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty DeployKeyMeaning _ _ keyValid
    have hv := deployValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem deployValueRejected_bound (d offset : Nat) (choice : Option DeployField) (wire : List w8)
    (reason : DeployValueFailure) (failure : DeployValueRejected d choice wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | selected failure => exact deployField_rejected_bound d _ wire offset _ failure
  | ignored failure => exact skipFailure_offset d offset wire _ failure

theorem deployPairsRejected_bounds (d n consumed offset : Nat) (seen : List DeployField) (old final : DeployValue)
    (wire : List w8) (reason : DeployFailureReason)
    (failure : DeployPairsRejected d n seen old wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := deployValueRejected_bound _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deployPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deployValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deployPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.deployValueRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.deployPairsRejected_bounds
