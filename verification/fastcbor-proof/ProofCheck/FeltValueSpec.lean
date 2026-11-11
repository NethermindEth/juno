import ProofCheck.FeltFailureUnique
import ProofCheck.FeltDeterminism
import ProofCheck.FeltDisjoint

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
/-- A by-value felt preserves the prior limbs on null/undefined; an array
    replaces all four limbs, including zero fill and supported ignored extras. -/
def FeltValueValid (d : Nat) (old : GoArray w64 4) (wire : List w8) (value : GoArray w64 4) : Prop :=
  ∃ parsed : Option (GoArray w64 4), FeltValid d wire parsed ∧ parsed.getD old = value

theorem feltValue_classify (d : Nat) (old : GoArray w64 4) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ FeltValueValid d old item value) ∨
    (∃ offset reason, FeltFailure d wire offset reason) := by
  rcases felt_classify d wire depth with ⟨item, rest, parsed, split, valid⟩ | ⟨offset, reason, failure⟩
  · exact Or.inl ⟨item, rest, parsed.getD old, split, parsed, valid, rfl⟩
  · exact Or.inr ⟨offset, reason, failure⟩

theorem feltValueValid_nonempty (d : Nat) (old value : GoArray w64 4) (wire : List w8)
    (valid : FeltValueValid d old wire value) : 0 < wire.length := by
  obtain ⟨parsed, valid, _⟩ := valid
  cases valid <;> simp

theorem feltValueValid_prefix_unique (d : Nat) (old a b : GoArray w64 4)
    (left right restA restB : List w8) (first : FeltValueValid d old left a)
    (second : FeltValueValid d old right b) (same : left ++ restA = right ++ restB) :
    left = right ∧ a = b ∧ restA = restB := by
  obtain ⟨parsed, valid, rfl⟩ := first
  obtain ⟨parsed2, valid2, rfl⟩ := second
  obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d parsed parsed2 left right restA restB valid valid2 same
  exact ⟨rfl, rfl, rfl⟩

theorem feltValueValid_not_failure (d offset : Nat) (old value : GoArray w64 4)
    (item rest : List w8) (reason : FeltFailureReason) (valid : FeltValueValid d old item value)
    (failure : FeltFailure d (item ++ rest) offset reason) : False := by
  obtain ⟨parsed, valid, _⟩ := valid
  exact feltValid_not_failure d offset item rest parsed reason valid failure

inductive FeltValueResult where
  | success (value : GoArray w64 4) (rest : List w8)
  | failure (offset : Nat) (reason : FeltFailureReason)

def FeltValueSpec (d : Nat) (old : GoArray w64 4) (wire : List w8) : FeltValueResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ FeltValueValid d old item value
  | .failure offset reason => FeltFailure d wire offset reason

theorem feltValueSpec_unique (d : Nat) (old : GoArray w64 4) (wire : List w8) (a b : FeltValueResult)
    (first : FeltValueSpec d old wire a) (second : FeltValueSpec d old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := feltValueValid_prefix_unique d old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure offset reason =>
      subst wire
      exact False.elim (feltValueValid_not_failure _ _ _ _ _ _ _ valid second)
  | failure offset reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (feltValueValid_not_failure _ _ _ _ _ _ _ valid first)
    | failure offset2 reason2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ first second
      rfl

theorem feltValueSpec_total_unique (d : Nat) (old : GoArray w64 4) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, FeltValueSpec d old wire outcome ∧
      ∀ other, FeltValueSpec d old wire other → other = outcome := by
  have existsOutcome : ∃ outcome, FeltValueSpec d old wire outcome := by
    rcases feltValue_classify d old wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure offset reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => feltValueSpec_unique d old wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValueValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValueValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValueValid_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValueSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValueSpec_total_unique
