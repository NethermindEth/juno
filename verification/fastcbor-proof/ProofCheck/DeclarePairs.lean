import ProofCheck.DeclareKeys

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive DeclareValueValid (d : Nat) : Option DeclareField → DeclareValue → List w8 → List w8 → DeclareValue → Prop where
  | selected {field old item rest final} (valid : DeclareFieldValid d field old item rest final) :
      DeclareValueValid d (some field) old item rest final
  | ignored {old : DeclareValue} {item rest} (valid : SkipValid d item) :
      DeclareValueValid d none old item rest old

inductive DeclareValueFailure where
  | field (reason : DeclareFieldFailure)
  | ignored (reason : SkipFailureReason)

inductive DeclareValueRejected (d : Nat) : Option DeclareField → DeclareValue → List w8 → DeclareValue →
    Nat → Nat → Nat → DeclareValueFailure → Prop where
  | selected {field old wire consumed offset extraDepth reason}
      (failure : DeclareFieldRejected d field wire consumed offset extraDepth reason) :
      DeclareValueRejected d (some field) old wire old consumed offset extraDepth (.field reason)
  | ignored {old : DeclareValue} {wire offset reason} (failure : SkipFailure d wire offset reason) :
      DeclareValueRejected d none old wire old 0 offset 0 (.ignored reason)

def DeclareChoiceFresh (seen : List DeclareField) : Option DeclareField → Prop
  | none => True
  | some field => field ∉ seen

def declareChoiceSeen (seen : List DeclareField) : Option DeclareField → List DeclareField
  | none => seen
  | some field => field :: seen

/-- Numeric failure positions are structured diagnostic offsets except for
    bare DA-mode overflow, where the field contract records only the consumed boundary. -/
inductive DeclareFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : KeyFailureReason)
  | duplicate (field : DeclareField)
  | value (reason : DeclareValueFailure)

/-- Successful pairs retain the actual suffix in every nested count guard.
    Known fields occur once; repeated unknown keys are permitted. -/
inductive DeclarePairsValid (d : Nat) : Nat → List DeclareField → DeclareValue → List w8 → List w8 → DeclareValue → Prop where
  | nil (seen : List DeclareField) (old : DeclareValue) (suffix : List w8) :
      DeclarePairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : DeclareKeyValid key choice) (fresh : DeclareChoiceFresh seen choice)
      (valueValid : DeclareValueValid d choice old value (tail ++ suffix) middle)
      (rest : DeclarePairsValid d n (declareChoiceSeen seen choice) middle tail suffix final) :
      DeclarePairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

/-- Rejection records the partial DeclareTransaction and separates consumed bytes from the
    diagnostic position. Failed fields preserve the declare but may retain private partial children and cursor levels; ignored values roll back;
    a duplicate is rejected immediately after its key is consumed. -/
inductive DeclarePairsRejected (d : Nat) : Nat → List DeclareField → DeclareValue → List w8 → DeclareValue →
    Nat → Nat → Nat → DeclareFailureReason → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      DeclarePairsRejected d (n + 1) seen old wire old 0 0 0 (.key reason)
  | duplicate {n seen old key tail field} (valid : DeclareKeyValid key (some field)) (duplicate : field ∈ seen) :
      DeclarePairsRejected d (n + 1) seen old (key ++ tail) old key.length key.length 0 (.duplicate field)
  | value {n seen choice old final key tail consumed offset extraDepth reason} (keyValid : DeclareKeyValid key choice)
      (fresh : DeclareChoiceFresh seen choice)
      (failure : DeclareValueRejected d choice old tail final consumed offset extraDepth reason) :
      DeclarePairsRejected d (n + 1) seen old (key ++ tail) final
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n seen choice old middle final key value tail consumed offset extraDepth reason}
      (keyValid : DeclareKeyValid key choice) (fresh : DeclareChoiceFresh seen choice)
      (valueValid : DeclareValueValid d choice old value tail middle)
      (failure : DeclarePairsRejected d n (declareChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      DeclarePairsRejected d (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

theorem declareValue_classify (d : Nat) (choice : Option DeclareField) (old : DeclareValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeclareValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, DeclareValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases declareField_classify d field old wire depth with ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨old, consumed, offset, extraDepth, _, .selected failure⟩

theorem declarePairs_classify (d n : Nat) (seen : List DeclareField) (old : DeclareValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeclarePairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, DeclarePairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases declareKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : DeclareChoiceFresh seen choice
      · rcases declareValue_classify d choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (declareChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using DeclarePairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [DeclareChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩

theorem declareValue_nonempty (d : Nat) (choice : Option DeclareField) (old final : DeclareValue)
    (item rest : List w8) (valid : DeclareValueValid d choice old item rest final) : 0 < item.length := by
  cases valid with
  | selected valid => exact declareField_nonempty d _ old final item rest valid
  | ignored valid => exact skipValid_nonempty d item valid

theorem declarePairs_count_bound (d n : Nat) (seen : List DeclareField) (old final : DeclareValue)
    (item rest : List w8) (valid : DeclarePairsValid d n seen old item rest final) : 2 * n ≤ item.length := by
  induction valid with
  | nil => simp
  | step keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty DeclareKeyMeaning _ _ keyValid
    have hv := declareValue_nonempty d _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

theorem declareValueRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option DeclareField)
    (old final : DeclareValue) (wire : List w8) (reason : DeclareValueFailure)
    (failure : DeclareValueRejected d choice old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | selected failure => exact declareField_rejected_bounds _ _ _ _ _ _ _ failure
  | ignored failure => exact ⟨Nat.zero_le _, skipFailure_offset _ _ _ _ failure⟩

theorem declareValueRejected_depth (d consumed offset extraDepth : Nat) (choice : Option DeclareField)
    (old final : DeclareValue) (wire : List w8) (reason : DeclareValueFailure) (depth : d ≤ 32)
    (failure : DeclareValueRejected d choice old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | selected failure => exact declareField_rejected_depth _ _ _ _ _ _ _ depth failure
  | ignored => omega

theorem declarePairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List DeclareField) (old final : DeclareValue)
    (wire : List w8) (reason : DeclareFailureReason)
    (failure : DeclarePairsRejected d n seen old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => omega
  | duplicate => simp only [List.length_append]; omega
  | value keyValid fresh failure =>
    have hb := declareValueRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append]; omega
  | next keyValid fresh valueValid failure ih => simp only [List.length_append]; omega

theorem declarePairsRejected_depth (d n consumed offset extraDepth : Nat) (seen : List DeclareField) (old final : DeclareValue)
    (wire : List w8) (reason : DeclareFailureReason) (depth : d ≤ 32)
    (failure : DeclarePairsRejected d n seen old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  induction failure with
  | key => omega
  | duplicate => omega
  | value keyValid fresh failure => exact declareValueRejected_depth _ _ _ _ _ _ _ _ _ depth failure
  | next keyValid fresh valueValid failure ih => exact ih

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declareValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.declarePairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.declareValue_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.declarePairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.declareValueRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.declareValueRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.declarePairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.declarePairsRejected_depth
