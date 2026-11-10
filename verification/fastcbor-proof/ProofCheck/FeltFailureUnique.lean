import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every felt failure has one diagnostic offset and reason. The first four
    unsigned children and all recursively ignored extras have deterministic
    boundaries; an earlier limb failure cannot overlap a completed limb prefix. -/
theorem feltFailure_unique (d offsetA offsetB : Nat) (wire : List w8) (a b : FeltFailureReason)
    (first : FeltFailure d wire offsetA a) (second : FeltFailure d wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @outer d wire reason hn hu failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, congrArg (FeltFailureReason.expected (W8 128)) (expectedFailure_reason_unique _ _ _ _ failure failure2)⟩
    | @depth head2 payload2 tail2 count2 enc2 major2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 tail2
        _ count2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
    | @limb d2 head2 payload2 tail2 count2 items2 reason2 enc2 major2 bound2 short2 more2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 (unsignedBytes items2 ++ tail2)
        _ count2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
    | @extras d2 offset2 head2 payload2 tail2 count2 items2 extras2 reason2 enc2 major2 bound2 four2 more2 valid2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head2 payload2 (unsignedBytes items2 ++ extras2.flatten ++ tail2)
        _ count2 _ enc2 major2 (by simpa only [List.append_assoc] using eqWire) failure)
  | @depth head payload tail count enc major =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head payload tail
        _ count _ enc major (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | @depth head2 payload2 tail2 count2 enc2 major2 =>
      exact ⟨rfl, rfl⟩
    | @limb d2 head2 payload2 tail2 count2 items2 reason2 enc2 major2 bound2 short2 more2 failure2 =>
      omega
    | @extras d2 offset2 head2 payload2 tail2 count2 items2 extras2 reason2 enc2 major2 bound2 four2 more2 valid2 failure2 =>
      omega
  | @limb d head payload tail count items reason enc major bound short more failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head payload (unsignedBytes items ++ tail)
        _ count _ enc major (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | @depth head2 payload2 tail2 count2 enc2 major2 =>
      omega
    | @limb d2 head2 payload2 tail2 count2 items2 reason2 enc2 major2 bound2 short2 more2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (unsignedBytes items ++ tail) (unsignedBytes items2 ++ tail2) count count2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2; subst count2
      obtain ⟨prefixEq, tailEq⟩ := unsignedFailure_prefix_unique items items2 tail tail2 reason reason2 failure failure2 tailsEq
      subst tail2
      have prefixLength := congrArg List.length prefixEq
      exact ⟨by omega, congrArg (FeltFailureReason.expected (W8 0))
        (expectedFailure_reason_unique _ _ _ _ failure failure2)⟩
    | @extras d2 offset2 head2 payload2 tail2 count2 items2 extras2 reason2 enc2 major2 bound2 four2 more2 valid2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (unsignedBytes items ++ tail) (unsignedBytes items2 ++ extras2.flatten ++ tail2) count count2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2; subst count2
      have fewer : items.length < items2.length := by omega
      obtain ⟨next, suffix, member, split⟩ := wireSequence_after_prefix UnsignedWire.bytes items items2 tail
        (extras2.flatten ++ tail2) fewer
        (fun x _ y _ restX restY same => (unsignedWire_prefix_unique x y restX restY same).2.2)
        (by simpa only [unsignedBytes, List.append_assoc] using tailsEq)
      exact False.elim (expectedFailure_excludes_encoded (W8 0) next.head next.payload suffix tail next.value _
        next.argument next.major split failure)
  | @extras d offset head payload tail count items extras reason enc major bound four more valid failure =>
    cases second with
    | @outer d2 wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 128) head payload (unsignedBytes items ++ extras.flatten ++ tail)
        _ count _ enc major (by simpa only [List.append_assoc] using eqWire.symm) failure2)
    | @depth head2 payload2 tail2 count2 enc2 major2 =>
      omega
    | @limb d2 head2 payload2 tail2 count2 items2 reason2 enc2 major2 bound2 short2 more2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (unsignedBytes items ++ extras.flatten ++ tail) (unsignedBytes items2 ++ tail2) count count2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2; subst count2
      have fewer : items2.length < items.length := by omega
      obtain ⟨next, suffix, member, split⟩ := wireSequence_after_prefix UnsignedWire.bytes items2 items tail2
        (extras.flatten ++ tail) fewer
        (fun x _ y _ restX restY same => (unsignedWire_prefix_unique x y restX restY same).2.2)
        (by simpa only [unsignedBytes, List.append_assoc] using tailsEq.symm)
      exact False.elim (expectedFailure_excludes_encoded (W8 0) next.head next.payload suffix tail2 next.value _
        next.argument next.major split failure2)
    | @extras d2 offset2 head2 payload2 tail2 count2 items2 extras2 reason2 enc2 major2 bound2 four2 more2 valid2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (unsignedBytes items ++ extras.flatten ++ tail) (unsignedBytes items2 ++ extras2.flatten ++ tail2) count count2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst head2; subst payload2; subst count2
      have lengths : items.length = items2.length := by omega
      obtain ⟨prefixEq, _, extrasEq⟩ := wireSequence_prefix_unique UnsignedWire.bytes UnsignedWire.value
        items items2 (extras.flatten ++ tail) (extras2.flatten ++ tail2) lengths
        (fun x _ y _ => unsignedWire_prefix_unique x y)
        (by simpa only [unsignedBytes, List.append_assoc] using tailsEq)
      obtain ⟨extraWiresEq, tailEq⟩ := skipFailure_prefix_unique (d + 1) extras extras2 tail tail2 offset offset2
        reason reason2 valid valid2 failure failure2 extrasEq
      subst tail2
      have diagnostic := skipFailure_unique (d + 1) offset offset2 tail reason reason2 failure failure2
      have prefixLength : (unsignedBytes items).length = (unsignedBytes items2).length := congrArg List.length prefixEq
      have extrasLength := congrArg List.length extraWiresEq
      exact ⟨by have := diagnostic.1; omega, congrArg FeltFailureReason.skipped diagnostic.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltFailure_unique
