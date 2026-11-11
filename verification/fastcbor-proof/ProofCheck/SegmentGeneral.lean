import ProofCheck.SegmentBounds

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Determinism also holds above the supported depth: no container can open,
    so only nullish successes and opening failures remain. This pure extension
    does not enlarge the supported domain of any production WP contract. -/
theorem segmentRecord_deterministic_any_depth (d : Nat) : SegmentRecordDeterministic d := by
  by_cases depth : d ≤ 32
  · exact segmentRecord_deterministic d depth
  · exact segmentRecord_deterministic_of_array d (fun inside => by omega)

theorem segmentRecordRejected_position_bounds (d consumed offset extraDepth : Nat)
    (old final : SegmentValue) (wire : List w8) (reason : SegmentFailureReason)
    (failure : SegmentRecordRejected d old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  by_cases depth : d ≤ 32
  · have bound := segmentRecordRejected_bounds d depth consumed offset extraDepth old final wire reason failure
    exact ⟨bound.1, bound.2.1⟩
  · cases failure with
    | opening => omega
    | map enc major size inside fit failure => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_deterministic_any_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecordRejected_position_bounds
