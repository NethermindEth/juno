import ProofCheck.StateUpdateRecordSpec

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A syntactic restriction of the independent pair relation: no selected
    StateDiff field. Felt fields and ignored values retain their full contracts.
    This is an input restriction, not an assumed decoder-correctness fact. -/
inductive StateUpdateScalarPairsValid (d : Nat) : Nat → List StateUpdateField → StateUpdateValue →
    List w8 → List w8 → StateUpdateValue → Prop where
  | nil (seen : List StateUpdateField) (old : StateUpdateValue) (suffix : List w8) :
      StateUpdateScalarPairsValid d 0 seen old [] suffix old
  | step {n seen choice old middle final key value tail suffix}
      (keyValid : StateUpdateKeyValid key choice) (fresh : StateUpdateChoiceFresh seen choice)
      (scalar : choice ≠ some .diff)
      (valueValid : StateUpdateValueValid d choice old value (tail ++ suffix) middle)
      (rest : StateUpdateScalarPairsValid d n (stateUpdateChoiceSeen seen choice) middle tail suffix final) :
      StateUpdateScalarPairsValid d (n + 1) seen old (key ++ value ++ tail) suffix final

theorem stateUpdateScalarPairs_valid (d n : Nat) (seen : List StateUpdateField)
    (old final : StateUpdateValue) (body suffix : List w8)
    (valid : StateUpdateScalarPairsValid d n seen old body suffix final) :
    StateUpdatePairsValid d n seen old body suffix final := by
  induction valid with
  | nil seen old suffix => exact .nil seen old suffix
  | step key fresh scalar value rest ih => exact .step key fresh value ih

/-- Arbitrary initial StateDiff contents are preserved throughout the restricted
    sequence, including allocated children; other fields may change. -/
theorem stateUpdateScalarPairs_preserve_diff (d n : Nat) (seen : List StateUpdateField)
    (old final : StateUpdateValue) (body suffix : List w8)
    (valid : StateUpdateScalarPairsValid d n seen old body suffix final) : final.diff = old.diff := by
  induction valid with
  | nil => rfl
  | step key fresh scalar value rest ih =>
    rw [ih]
    cases value with
    | ignored => rfl
    | selected fieldValid =>
      cases fieldValid with
      | blockHash => rfl
      | newRoot => rfl
      | oldRoot => rfl
      | diff => exact False.elim (scalar rfl)

/-- Successful records in the supported partial-proof scope, including nullish
    records and arbitrary field order, known-key spelling and ignored values. -/
inductive StateUpdateScalarRecordValid (d : Nat) (old : StateUpdateValue) :
    List w8 → List w8 → StateUpdateValue → Prop where
  | nullish {head : w8} (suffix : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      StateUpdateScalarRecordValid d old [head] suffix old
  | map {head payload body suffix argument final} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (depth : d < 32)
      (valid : StateUpdateScalarPairsValid (d + 1) count [] old body suffix final) :
      StateUpdateScalarRecordValid d old ((head :: payload) ++ body) suffix final

theorem stateUpdateScalarRecord_valid (d : Nat) (old final : StateUpdateValue) (item suffix : List w8)
    (valid : StateUpdateScalarRecordValid d old item suffix final) :
    StateUpdateRecordValid d old item suffix final := by
  cases valid with
  | nullish suffix hn => exact .nullish suffix hn
  | map enc major size depth pairs =>
    exact .map enc major size depth (stateUpdateScalarPairs_valid _ _ _ _ _ _ _ pairs)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateScalarPairs_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateScalarPairs_preserve_diff
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateScalarRecord_valid
