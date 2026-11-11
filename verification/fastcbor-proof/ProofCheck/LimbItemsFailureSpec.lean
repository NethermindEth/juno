import ProofCheck.LimbItemsWire

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every independent sequence rejection consists of a successful prefix and
    one failed nonnullable felt. Both consumed and diagnostic offsets are retained. -/
theorem limbItems_rejected_witness (depth count consumed offset : Nat) (wire : List w8)
    (done : List (GoArray w64 4)) (reason : FeltFailureReason)
    (failure : LimbItemsRejected depth count wire done consumed offset reason) :
    ∃ (items : List (LimbValueWire depth)) (rest : List w8) (inner : Nat),
      items.length < count ∧ wire = items.flatMap LimbValueWire.bytes ++ rest ∧
      done = items.map LimbValueWire.value ∧ consumed = (items.flatMap LimbValueWire.bytes).length ∧
      offset = (items.flatMap LimbValueWire.bytes).length + inner ∧ LimbsFailure depth rest inner reason := by
  induction failure with
  | first failure => exact ⟨[], _, _, by simp, rfl, rfl, rfl, by simp, failure⟩
  | @next n item tail value done consumed offset reason valid failure ih =>
    obtain ⟨items, rest, inner, more, split, values, used, error, rejected⟩ := ih
    refine ⟨⟨item, value, valid⟩ :: items, rest, inner, ?_, ?_, ?_, ?_, ?_, rejected⟩
    · simpa using Nat.succ_lt_succ more
    · simp only [List.flatMap_cons, List.append_assoc]; rw [split]
    · simp only [List.map_cons]; rw [values]
    · simp only [List.flatMap_cons, List.length_append]; rw [used]
    · simp only [List.flatMap_cons, List.length_append]; rw [error]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.limbItems_rejected_witness
