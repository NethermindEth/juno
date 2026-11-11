import ProofCheck.Uint64PointerSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- DAMode is a uint32-backed application value: all values through 4294967295 are
    accepted, not only named enum constants. Null and undefined preserve it. -/
inductive DAModeValid (old : w32) : List w8 → w32 → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : DAModeValid old [head] old
  | unsigned {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
      (fits : uint.Z argument ≤ 4294967295) : DAModeValid old (head :: payload) (W32 (uint.Z argument))

inductive DAModeFailureReason where
  | expected (reason : ExpectedFailureReason)
  | overflow

/-- Unsigned parse errors roll back. A value above 4294967295 is consumed before the
    32-bit width check returns the bare type-error sentinel, without assignment. -/
inductive DAModeRejected : List w8 → Nat → DAModeFailureReason → Prop where
  | expected {wire reason} (failure : ScalarFailure wire reason) : DAModeRejected wire 0 (.expected reason)
  | overflow {head payload rest argument}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
      (large : 4294967295 < uint.Z argument) :
      DAModeRejected ((head :: payload) ++ rest) (1 + payload.length) .overflow

theorem daMode_classify (old : w32) (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ DAModeValid old item value) ∨
    (∃ consumed reason, DAModeRejected wire consumed reason) := by
  rcases scalar_classify (W64 (uint.Z old)) wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
  · cases valid with
    | nullish hn => exact Or.inl ⟨_, rest, old, split, .nullish hn⟩
    | unsigned enc major =>
      by_cases fits : uint.Z value ≤ 4294967295
      · exact Or.inl ⟨_, rest, _, split, .unsigned enc major fits⟩
      · subst wire
        exact Or.inr ⟨_, .overflow, .overflow enc major (by omega)⟩
  · exact Or.inr ⟨0, .expected reason, .expected failure⟩

theorem daModeValid_nonempty (old value : w32) (wire : List w8)
    (valid : DAModeValid old wire value) : 0 < wire.length := by cases valid <;> simp

theorem daModeValid_prefix_unique (old a b : w32) (left right restA restB : List w8)
    (first : DAModeValid old left a) (second : DAModeValid old right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @nullish head hn =>
    cases second with
    | nullish ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | unsigned enc major fits =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @unsigned head payload argument enc major fits =>
    cases second with
    | nullish ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | unsigned enc2 major2 fits2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      exact ⟨rfl, rfl, rfl⟩

theorem daModeValid_not_rejected (old value : w32) (item rest : List w8)
    (consumed : Nat) (reason : DAModeFailureReason) (valid : DAModeValid old item value)
    (failure : DAModeRejected (item ++ rest) consumed reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases failure with
  | expected failure =>
    rw [← same] at failure
    cases valid with
    | nullish hn =>
      rcases hn with rfl | rfl
      · exact failure.1 rfl
      · exact failure.2.1 rfl
    | unsigned enc major fits =>
      exact expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major rfl failure.2.2
  | overflow enc major large =>
    cases valid with
    | nullish hn =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | unsigned enc2 major2 fits =>
      obtain ⟨_, _, argumentEq, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc2 enc same
      rw [argumentEq] at fits
      omega

theorem daModeRejected_unique (wire : List w8) (consumedA consumedB : Nat) (a b : DAModeFailureReason)
    (first : DAModeRejected wire consumedA a) (second : DAModeRejected wire consumedB b) :
    consumedA = consumedB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | expected failure =>
    cases second with
    | expected failure2 =>
      rw [← same] at failure2
      exact ⟨rfl, congrArg DAModeFailureReason.expected (expectedFailure_reason_unique _ _ _ _ failure.2.2 failure2.2.2)⟩
    | overflow enc major large =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major same failure.2.2)
  | overflow enc major large =>
    cases second with
    | expected failure2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major same.symm failure2.2.2)
    | overflow enc2 major2 large2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      exact ⟨rfl, rfl⟩

theorem daModeRejected_bound (wire : List w8) (consumed : Nat) (reason : DAModeFailureReason)
    (failure : DAModeRejected wire consumed reason) : consumed ≤ wire.length := by
  cases failure <;> simp <;> omega

inductive DAModeResult where
  | success (value : w32) (rest : List w8)
  | failure (consumed : Nat) (reason : DAModeFailureReason)

def DAModeSpec (old : w32) (wire : List w8) : DAModeResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ DAModeValid old item value
  | .failure consumed reason => DAModeRejected wire consumed reason

theorem daModeSpec_unique (old : w32) (wire : List w8) (a b : DAModeResult)
    (first : DAModeSpec old wire a) (second : DAModeSpec old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := daModeValid_prefix_unique old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure consumed reason =>
      subst wire
      exact False.elim (daModeValid_not_rejected _ _ _ _ _ _ valid second)
  | failure consumed reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (daModeValid_not_rejected _ _ _ _ _ _ valid first)
    | failure consumed2 reason2 =>
      obtain ⟨rfl, rfl⟩ := daModeRejected_unique _ _ _ _ _ first second
      rfl

theorem daModeSpec_total_unique (old : w32) (wire : List w8) :
    ∃ outcome, DAModeSpec old wire outcome ∧ ∀ other, DAModeSpec old wire other → other = outcome := by
  have existsOutcome : ∃ outcome, DAModeSpec old wire outcome := by
    rcases daMode_classify old wire with ⟨item, rest, value, split, valid⟩ | ⟨consumed, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure consumed reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => daModeSpec_unique old wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.daMode_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.daModeValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.daModeValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.daModeValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.daModeRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.daModeRejected_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.daModeSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.daModeSpec_total_unique
