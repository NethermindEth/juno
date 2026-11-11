import ProofCheck.IntegerKeySpec
import ProofCheck.KeyFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Integer-key dispatch accepts exactly a subset of the independent Key wire
    relation; selecting a numeric field adds no alternative byte boundary. -/
theorem integerKeyValid_key (wire : List w8) (key : Option w64) (valid : IntegerKeyValid wire key) :
    ∃ value, KeyValid wire value := by
  cases valid with
  | unsigned enc major bound => exact ⟨none, .integer enc (Or.inl major) bound⟩
  | unknown valid _ => exact ⟨_, valid⟩

theorem integerKeyValid_prefix_unique (left right restLeft restRight : List w8) (a b : Option w64)
    (first : IntegerKeyValid left a) (second : IntegerKeyValid right b)
    (same : left ++ restLeft = right ++ restRight) : left = right ∧ a = b ∧ restLeft = restRight := by
  obtain ⟨x, hx⟩ := integerKeyValid_key left a first
  obtain ⟨y, hy⟩ := integerKeyValid_key right b second
  obtain ⟨rfl, _, rfl⟩ := keyValid_prefix_unique left right restLeft restRight x y hx hy same
  refine ⟨rfl, ?_, rfl⟩
  cases first with
  | unsigned enc major bound =>
    cases second with
    | unsigned enc2 _ _ => exact congrArg some (headArgument_value_unique _ _ _ _ enc enc2)
    | unknown _ notUnsigned => exact False.elim (notUnsigned (congrArg some major))
  | unknown valid notUnsigned =>
    cases second with
    | unsigned enc major bound => exact False.elim (notUnsigned (congrArg some major))
    | unknown _ _ => rfl

/-- Every dispatch failure is also a key wire rejection, although overflow has
    a different concrete return representation and retained consumption. -/
theorem integerKeyRejected_key (wire : List w8) (consumed : Nat) (reason : IntegerKeyFailureReason)
    (failure : IntegerKeyRejected wire consumed reason) : ∃ keyReason, KeyRejected wire keyReason := by
  cases failure with
  | unsigned major failure =>
    cases failure with
    | head tail fault => exact ⟨_, .head _ fault⟩
    | incomplete tail info sizeEq short => exact ⟨_, .incomplete _ info sizeEq short⟩
    | typeMismatch tail enc mismatch notTag => exact False.elim (mismatch major)
    | tag tail enc mismatch isTag => exact False.elim (mismatch major)
  | overflow enc major bound => exact ⟨.integerRange, .integerRange enc (Or.inl major) bound⟩
  | other _ failure => exact ⟨_, failure⟩

theorem integerKeyValid_not_rejected (item rest : List w8) (key : Option w64) (consumed : Nat)
    (reason : IntegerKeyFailureReason) (valid : IntegerKeyValid item key)
    (failure : IntegerKeyRejected (item ++ rest) consumed reason) : False := by
  obtain ⟨value, keyValid⟩ := integerKeyValid_key item key valid
  obtain ⟨keyReason, keyFailure⟩ := integerKeyRejected_key (item ++ rest) consumed reason failure
  exact keyValid_not_rejected item rest value keyReason keyValid keyFailure

/-- Rejections determine both the error class and the exact retained cursor,
    including overflow's consumed head versus rollback in every other case. -/
theorem integerKeyRejected_unique (wire : List w8) (consumedA consumedB : Nat) (a b : IntegerKeyFailureReason)
    (first : IntegerKeyRejected wire consumedA a) (second : IntegerKeyRejected wire consumedB b) :
    consumedA = consumedB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @unsigned head tail reason major failure =>
    cases second with
    | unsigned major2 failure2 =>
      rw [← eqWire] at failure2
      exact ⟨rfl, congrArg IntegerKeyFailureReason.unsigned (expectedFailure_reason_unique _ _ _ _ failure failure2)⟩
    | @overflow head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 0) head2 payload2 tail2 _ argument2 reason enc2 major2 eqWire failure)
    | other notUnsigned failure2 =>
      subst wire2
      exact False.elim (notUnsigned (congrArg some major))
  | @overflow head payload tail argument enc major bound =>
    cases second with
    | unsigned major2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 0) head payload tail _ argument _ enc major eqWire.symm failure2)
    | @overflow head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      obtain ⟨rfl, rfl, _, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
      exact ⟨rfl, rfl⟩
    | other notUnsigned failure2 =>
      subst wire2
      exact False.elim (notUnsigned (congrArg some major))
  | @other wire reason notUnsigned failure =>
    cases second with
    | unsigned major2 failure2 =>
      subst wire
      exact False.elim (notUnsigned (congrArg some major2))
    | overflow enc2 major2 bound2 =>
      subst wire
      exact False.elim (notUnsigned (congrArg some major2))
    | other notUnsigned2 failure2 =>
      subst wire2
      exact ⟨rfl, congrArg IntegerKeyFailureReason.other (keyRejected_reason_unique _ _ _ failure failure2)⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKeyValid_key
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKeyValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKeyRejected_key
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKeyValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKeyRejected_unique
