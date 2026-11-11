import ProofCheck.Int64Determinism
import ProofCheck.IntElementFailure

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- No independently specified integer/null element can begin a rejected input. -/
theorem intElementWire_not_rejected (item : IntElementWire) (rest : List w8)
    (consumed : Nat) (reason : IntElementFailureReason)
    (failure : IntElementRejected (item.bytes ++ rest) consumed reason) : False := by
  cases item with
  | nullish head hn =>
    generalize same : IntElementWire.bytes (.nullish head hn) ++ rest = wire at failure
    cases failure with
    | overflow tail enc major bound =>
      simp only [IntElementWire.bytes, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> word
    | fallback _ _ notNull notUndefined _ =>
      subst wire
      rcases hn with rfl | rfl
      · exact notNull rfl
      · exact notUndefined rfl
  | unsigned head payload argument enc major bound =>
    generalize same : IntElementWire.bytes (.unsigned head payload argument enc major bound) ++ rest = wire at failure
    cases failure with
    | @overflow other payload2 argument2 tail enc2 major2 bound2 =>
      obtain ⟨_, _, rfl, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      omega
    | fallback _ _ _ _ signed =>
      subst wire
      exact int64Valid_not_rejected _ rest _ _ (.unsigned enc major bound) signed
  | negative head payload argument enc major bound =>
    generalize same : IntElementWire.bytes (.negative head payload argument enc major bound) ++ rest = wire at failure
    cases failure with
    | overflow tail enc2 major2 bound2 =>
      simp only [IntElementWire.bytes, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      word
    | fallback _ _ _ _ signed =>
      subst wire
      exact int64Valid_not_rejected _ rest _ _ (.negative enc major bound) signed

/-- Every failed element determines its diagnostic and retained consumption;
    unsigned overflow and rolled-back signed fallback cannot both apply. -/
theorem intElementRejected_unique (wire : List w8) (consumedA consumedB : Nat)
    (a b : IntElementFailureReason) (first : IntElementRejected wire consumedA a)
    (second : IntElementRejected wire consumedB b) : consumedA = consumedB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | @overflow head payload argument tail enc major bound =>
    cases second with
    | overflow tail2 enc2 major2 bound2 =>
      obtain ⟨_, rfl, _, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      exact ⟨rfl, rfl⟩
    | fallback _ unsigned _ _ _ =>
      exact False.elim (expectedFailure_excludes_encoded (W8 0) head payload tail _ argument _ enc major same.symm unsigned)
  | fallback _ unsigned _ _ signed =>
    cases second with
    | @overflow head2 payload2 argument2 tail2 enc2 major2 bound2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 0) head2 payload2 tail2 _ argument2 _ enc2 major2 same unsigned)
    | fallback _ _ _ _ signed2 =>
      rw [← same] at signed2
      exact ⟨rfl, congrArg IntElementFailureReason.signed (int64Rejected_reason_unique _ _ _ signed signed2)⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.intElementWire_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.intElementRejected_unique
