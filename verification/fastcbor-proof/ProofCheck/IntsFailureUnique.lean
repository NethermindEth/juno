import ProofCheck.IntsDisjoint
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Signed-array failures uniquely fix retained consumption, diagnostic offset,
    entered depth and reason, including failures after arbitrary valid children. -/
theorem intsRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (depthA depthB : Bool)
    (wire : List w8) (a b : IntsFailureReason) (bound : wire.length < 2^64)
    (first : IntsRejected d wire consumedA offsetA depthA a) (second : IntsRejected d wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @outer d wire reason hn hu failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, congrArg IntsFailureReason.outer
        (expectedFailure_reason_unique _ _ _ _ failure failure2)⟩
    | @count d2 head2 payload2 tail2 argument2 enc2 major2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
    | @depth head2 payload2 tail2 argument2 enc2 major2 fit2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
    | @element d2 count2 consumed2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 (items2.flatMap IntElementWire.bytes ++ tail2)
        _ (W64 count2) _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
  | @count d head payload tail argument enc major short =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head payload tail
        _ argument _ enc major (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | @count d2 head2 payload2 tail2 argument2 enc2 major2 short2 =>
      exact ⟨rfl, rfl, rfl, rfl⟩
    | @depth head2 payload2 tail2 argument2 enc2 major2 fit2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2
      cases argumentEq; cases tailsEq
      omega
    | @element d2 count2 consumed2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (items2.flatMap IntElementWire.bytes ++ tail2) argument (W64 count2) enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2
      have wrapped : uint.Z (W64 count2) ≤ count2 := by word
      have lengths := congrArg List.length tailsEq
      simp only [List.length_append] at lengths
      rw [argumentEq] at short
      simp only [List.length_append] at fit2
      omega
  | @depth head payload tail argument enc major fit =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head payload tail
        _ argument _ enc major (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | @count d2 head2 payload2 tail2 argument2 enc2 major2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2
      cases argumentEq; cases tailsEq
      omega
    | @depth head2 payload2 tail2 argument2 enc2 major2 fit2 =>
      exact ⟨rfl, rfl, rfl, rfl⟩
    | @element d2 count2 consumed2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      omega
  | @element d count consumed head payload tail items reason enc major depth more fit failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head payload (items.flatMap IntElementWire.bytes ++ tail)
        _ (W64 count) _ enc major (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | @count d2 head2 payload2 tail2 argument2 enc2 major2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (items.flatMap IntElementWire.bytes ++ tail) tail2 (W64 count) argument2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2
      have wrapped : uint.Z (W64 count) ≤ count := by word
      have lengths := congrArg List.length tailsEq
      simp only [List.length_append] at lengths
      rw [← argumentEq] at short2
      simp only [List.length_append] at fit
      omega
    | @depth head2 payload2 tail2 argument2 enc2 major2 fit2 =>
      omega
    | @element d2 count2 consumed2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (items.flatMap IntElementWire.bytes ++ tail) (items2.flatMap IntElementWire.bytes ++ tail2) (W64 count) (W64 count2) enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2
      obtain ⟨prefixEq, tailEq⟩ := wireFailure_prefix_unique IntElementWire.bytes items items2 tail tail2 (by
        intro x _ y _ restX restY same
        have result := intElementWire_prefix_unique x y restX restY same
        exact ⟨result.1, result.2.2⟩) (by
          intro next _ suffix split
          subst tail
          exact intElementWire_not_rejected next suffix consumed reason failure) (by
          intro next _ suffix split
          subst tail2
          exact intElementWire_not_rejected next suffix consumed2 reason2 failure2) tailsEq
      subst tail2
      have prefixLength := congrArg List.length prefixEq
      have diagnostic := intElementRejected_unique tail consumed consumed2 reason reason2 failure failure2
      exact ⟨by have := diagnostic.1; omega, by omega, rfl, congrArg IntsFailureReason.element diagnostic.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.intsRejected_unique
