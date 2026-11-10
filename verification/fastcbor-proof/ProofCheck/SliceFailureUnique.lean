import ProofCheck.FeltFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- feltPointers failures uniquely fix retained consumption, diagnostic offset,
    entered depth and reason, including failures after arbitrary valid children. -/
theorem feltPointersFailure_unique (d consumedA offsetA consumedB offsetB : Nat) (depthA depthB : Bool)
    (wire : List w8) (a b : FeltFailureReason) (bound : wire.length < 2^64)
    (first : FeltPointersFailure d wire consumedA offsetA depthA a) (second : FeltPointersFailure d wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @outer d wire reason hn hu failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, congrArg (FeltFailureReason.expected (W8 128))
        (expectedFailure_reason_unique _ _ _ _ failure failure2)⟩
    | @count d2 head2 payload2 tail2 argument2 enc2 major2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
    | @depth head2 payload2 tail2 argument2 enc2 major2 fit2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
    | @child d2 count2 offset2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 (items2.flatMap FeltWire.bytes ++ tail2)
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
    | @child d2 count2 offset2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (items2.flatMap FeltWire.bytes ++ tail2) argument (W64 count2) enc enc2
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
    | @child d2 count2 offset2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      omega
  | @child d count offset head payload tail items reason enc major depth more fit failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head payload (items.flatMap FeltWire.bytes ++ tail)
        _ (W64 count) _ enc major (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | @count d2 head2 payload2 tail2 argument2 enc2 major2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (items.flatMap FeltWire.bytes ++ tail) tail2 (W64 count) argument2 enc enc2
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
    | @child d2 count2 offset2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (items.flatMap FeltWire.bytes ++ tail) (items2.flatMap FeltWire.bytes ++ tail2) (W64 count) (W64 count2) enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2
      have tailBound : tail.length < 2^64 := by
        simp only [List.length_append, List.length_cons] at bound; omega
      have tailBound2 : tail2.length < 2^64 := by
        have otherBound : (((head :: payload) ++ items2.flatMap FeltWire.bytes) ++ tail2).length < 2^64 := by
          rw [← eqWire]; exact bound
        simp only [List.length_append, List.length_cons] at otherBound; omega
      obtain ⟨prefixEq, tailEq⟩ := wireFailure_prefix_unique FeltWire.bytes items items2 tail tail2 (by
        intro x _ y _ restX restY same
        have result := feltValid_prefix_unique (d + 1) x.value y.value x.bytes y.bytes restX restY x.valid y.valid same
        exact ⟨result.1, result.2.2⟩) (by
          intro next _ suffix split
          subst tail
          exact feltValid_not_failure (d + 1) offset next.bytes suffix next.value a next.valid failure) (by
          intro next _ suffix split
          subst tail2
          exact feltValid_not_failure (d + 1) offset2 next.bytes suffix next.value b next.valid failure2) tailsEq
      subst tail2
      have prefixLength := congrArg List.length prefixEq
      have diagnostic := feltFailure_unique (d + 1) offset offset2 tail a b failure failure2
      exact ⟨by omega, by have := diagnostic.1; omega, rfl, diagnostic.2⟩

/-- signatures failures uniquely fix retained consumption, diagnostic offset,
    entered depth and reason, including failures after arbitrary valid children. -/
theorem signaturesFailure_unique (d consumedA offsetA consumedB offsetB : Nat) (depthA depthB : Nat)
    (wire : List w8) (a b : FeltFailureReason) (bound : wire.length < 2^64)
    (first : SignaturesFailure d wire consumedA offsetA depthA a) (second : SignaturesFailure d wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @outer d wire reason hn hu failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, congrArg (FeltFailureReason.expected (W8 128))
        (expectedFailure_reason_unique _ _ _ _ failure failure2)⟩
    | @count d2 head2 payload2 tail2 argument2 enc2 major2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
    | @depth head2 payload2 tail2 argument2 enc2 major2 fit2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
    | @child d2 count2 consumed2 offset2 entered2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 (items2.flatMap SignatureWire.bytes ++ tail2)
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
    | @child d2 count2 consumed2 offset2 entered2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (items2.flatMap SignatureWire.bytes ++ tail2) argument (W64 count2) enc enc2
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
    | @child d2 count2 consumed2 offset2 entered2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      omega
  | @child d count consumed offset entered head payload tail items reason enc major depth more fit failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head payload (items.flatMap SignatureWire.bytes ++ tail)
        _ (W64 count) _ enc major (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | @count d2 head2 payload2 tail2 argument2 enc2 major2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (items.flatMap SignatureWire.bytes ++ tail) tail2 (W64 count) argument2 enc enc2
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
    | @child d2 count2 consumed2 offset2 entered2 head2 payload2 tail2 items2 reason2 enc2 major2 depth2 more2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (items.flatMap SignatureWire.bytes ++ tail) (items2.flatMap SignatureWire.bytes ++ tail2) (W64 count) (W64 count2) enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2
      have tailBound : tail.length < 2^64 := by
        simp only [List.length_append, List.length_cons] at bound; omega
      have tailBound2 : tail2.length < 2^64 := by
        have otherBound : (((head :: payload) ++ items2.flatMap SignatureWire.bytes) ++ tail2).length < 2^64 := by
          rw [← eqWire]; exact bound
        simp only [List.length_append, List.length_cons] at otherBound; omega
      obtain ⟨prefixEq, tailEq⟩ := wireFailure_prefix_unique SignatureWire.bytes items items2 tail tail2 (by
        intro x _ y _ restX restY same
        have result := feltPointersValid_prefix_unique (d + 1) x.value y.value x.bytes y.bytes restX restY x.valid y.valid same
        exact ⟨result.1, result.2.2⟩) (by
          intro next _ suffix split
          subst tail
          exact feltPointersValid_not_failure (d + 1) consumed offset entered next.bytes suffix next.value a tailBound next.valid failure) (by
          intro next _ suffix split
          subst tail2
          exact feltPointersValid_not_failure (d + 1) consumed2 offset2 entered2 next.bytes suffix next.value b tailBound2 next.valid failure2) tailsEq
      subst tail2
      have prefixLength := congrArg List.length prefixEq
      have diagnostic := feltPointersFailure_unique (d + 1) consumed offset consumed2 offset2 entered entered2
        tail a b tailBound failure failure2
      exact ⟨by have := diagnostic.1; omega, by have := diagnostic.2.1; omega,
        by rw [diagnostic.2.2.1], diagnostic.2.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointersFailure_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.signaturesFailure_unique
