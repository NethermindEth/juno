import ProofCheck.LimbWideSpec
import ProofCheck.LimbItemsSpec
import ProofCheck.FeltFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Overlapping outer/typed in-place rejection constructors still determine
    exactly one diagnostic. -/
theorem limbsFailure_unique (d offsetA offsetB : Nat) (wire : List w8) (a b : FeltFailureReason)
    (first : LimbsFailure d wire offsetA a) (second : LimbsFailure d wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | typed first =>
    cases second with
    | typed second => exact feltFailure_unique d offsetA offsetB wire a b first second
    | outer second =>
      have hn := feltFailure_not_nullish d offsetA wire a first
      exact feltFailure_unique d offsetA 0 wire a _ first (.outer hn.1 hn.2 second)
  | outer first =>
    cases second with
    | typed second =>
      have hn := feltFailure_not_nullish d offsetB wire b second
      exact feltFailure_unique d 0 offsetB wire _ b (.outer hn.1 hn.2 first) second
    | outer second =>
      exact ⟨rfl, congrArg (FeltFailureReason.expected (W8 128)) (expectedFailure_reason_unique _ wire _ _ first second)⟩

/-- No successful requested element sequence can also reject, even after an
    arbitrary prefix or inside an extended felt's ignored contents. -/
theorem limbItems_valid_not_rejected (d n consumed offset : Nat) (item rest : List w8)
    (values done : List (GoArray w64 4)) (reason : FeltFailureReason)
    (valid : LimbItemsValid d n item values)
    (failure : LimbItemsRejected d n (item ++ rest) done consumed offset reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing wire rest done consumed offset reason with
  | nil => cases failure
  | @cons n item tail value values head valid ih =>
    cases failure with
    | first rejected =>
      apply limbsFailure_not_valid d _ item (tail ++ rest) value _ head
      rw [← same] at rejected
      simpa only [List.append_assoc] using rejected
    | @next n2 item2 tail2 value2 done2 consumed2 offset2 reason2 head2 rejected =>
      obtain ⟨_, _, tailsEq⟩ := feltValid_prefix_unique d _ _ item item2 (tail ++ rest) tail2 head head2
        (by simpa only [List.append_assoc] using same)
      exact ih (wire := tail2) (rest := rest) (done := done2) (consumed := consumed2) (offset := offset2) (reason := reason) tailsEq rejected

/-- Rejected sequences uniquely determine all successful writes, cursor
    consumption, diagnostic offset and reason. -/
theorem limbItems_rejected_unique (d n usedA usedB offsetA offsetB : Nat) (wire : List w8)
    (doneA doneB : List (GoArray w64 4)) (a b : FeltFailureReason)
    (first : LimbItemsRejected d n wire doneA usedA offsetA a)
    (second : LimbItemsRejected d n wire doneB usedB offsetB b) :
    doneA = doneB ∧ usedA = usedB ∧ offsetA = offsetB ∧ a = b := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 doneB usedB offsetB b with
  | @first n wire offset reason failure =>
    cases second with
    | first failure2 =>
      rw [same] at failure
      obtain ⟨rfl, rfl⟩ := limbsFailure_unique d _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
    | @next n2 item2 tail2 value2 done2 consumed2 offset2 reason2 head2 rejected =>
      rw [same] at failure
      exact False.elim (limbsFailure_not_valid d offset item2 tail2 value2 reason head2 failure)
  | @next n item tail value done consumed offset reason head failure ih =>
    cases second with
    | first failure2 =>
      rw [← same] at failure2
      exact False.elim (limbsFailure_not_valid d _ item tail value _ head failure2)
    | @next n2 item2 tail2 value2 done2 consumed2 offset2 reason2 head2 failure2 =>
      obtain ⟨rfl, valuesEq, tailsEq⟩ := feltValid_prefix_unique d _ _ item item2 tail tail2 head head2 same
      have valueEq := Option.some.inj valuesEq
      subst value2
      obtain ⟨rfl, rfl, rfl, rfl⟩ := ih (wire2 := tail2) (doneB := done2) (usedB := consumed2) (offsetB := offset2) (b := b) tailsEq failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.limbsFailure_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_valid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_rejected_unique
