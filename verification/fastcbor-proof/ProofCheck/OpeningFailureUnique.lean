import ProofCheck.ExpectedFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Complete string errors have a unique reason, with expectation, payload
    truncation and UTF8 rejection remaining distinct and ordered. -/
theorem stringRejected_reason_unique (wire : List w8) (a b : StringFailureReason)
    (first : StringRejected wire a) (second : StringRejected wire b) : a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | expected hn hu failure =>
    cases second with
    | expected hn2 hu2 failure2 =>
      rw [eqWire] at failure
      exact congrArg StringFailureReason.expected (expectedFailure_reason_unique _ _ _ _ failure failure2)
    | truncated enc2 major2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc2 major2 eqWire failure)
    | invalidUTF8 enc2 major2 count2 invalid2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc2 major2
        (by simpa only [List.append_assoc] using eqWire) failure)
  | truncated enc major short =>
    cases second with
    | expected hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major eqWire.symm failure2)
    | truncated => rfl
    | invalidUTF8 enc2 major2 count2 invalid2 =>
      obtain ⟨_, _, argumentEq, tailsEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      cases argumentEq
      have lengths := congrArg List.length tailsEq
      simp only [List.length_append] at lengths
      omega
  | invalidUTF8 enc major count invalid =>
    cases second with
    | expected hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major
        (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | truncated enc2 major2 short2 =>
      obtain ⟨_, _, argumentEq, tailsEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      cases argumentEq
      have lengths := congrArg List.length tailsEq
      simp only [List.length_append] at lengths
      omega
    | invalidUTF8 => rfl

/-- Typed map-opening errors have a unique reason. In particular, count
    truncation cannot also be a depth error, since depth requires count fit. -/
theorem structOpenFailure_reason_unique (d : Nat) (wire : List w8) (a b : FeltFailureReason)
    (first : StructOpenFailure d wire a) (second : StructOpenFailure d wire b) : a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | outer hn hu failure =>
    cases second with
    | outer hn2 hu2 failure2 =>
      rw [eqWire] at failure
      exact congrArg (FeltFailureReason.expected (W8 160)) (expectedFailure_reason_unique _ _ _ _ failure failure2)
    | count enc2 major2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc2 major2 eqWire failure)
    | depth enc2 major2 fit2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc2 major2 eqWire failure)
  | count enc major short =>
    cases second with
    | outer hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major eqWire.symm failure2)
    | count => rfl
    | depth enc2 major2 fit2 =>
      obtain ⟨_, _, argumentEq, tailsEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
      cases argumentEq
      cases tailsEq
      omega
  | depth enc major fit =>
    cases second with
    | outer hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major eqWire.symm failure2)
    | count enc2 major2 short2 =>
      obtain ⟨_, _, argumentEq, tailsEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
      cases argumentEq
      cases tailsEq
      omega
    | depth => rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stringRejected_reason_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.structOpenFailure_reason_unique
