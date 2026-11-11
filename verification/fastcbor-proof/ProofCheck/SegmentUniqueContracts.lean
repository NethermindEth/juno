import ProofCheck.SegmentProgress
import ProofCheck.WireDeterminism
import ProofCheck.ExpectedFailureUnique
import ProofCheck.SkipFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Induction predicates, proved together below; these are pure specification
    properties, not production-helper correctness assumptions. -/
structure SegmentRecordDeterministic (d : Nat) : Prop where
  successful : ∀ (old a b : SegmentValue) (left right restA restB : List w8),
    SegmentRecordValid d old left restA a → SegmentRecordValid d old right restB b →
    left ++ restA = right ++ restB → left = right ∧ a = b ∧ restA = restB
  disjoint : ∀ (consumed offset : Nat) (old final retained : SegmentValue) (item rest : List w8)
    (extraDepth : Nat) (reason : SegmentFailureReason),
    SegmentRecordValid d old item rest final →
    SegmentRecordRejected d old (item ++ rest) retained consumed offset extraDepth reason → False
  rejected : ∀ (consumedA offsetA consumedB offsetB : Nat) (old finalA finalB : SegmentValue)
    (wire : List w8) (depthA depthB : Nat) (a b : SegmentFailureReason),
    SegmentRecordRejected d old wire finalA consumedA offsetA depthA a →
    SegmentRecordRejected d old wire finalB consumedB offsetB depthB b →
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b

def SegmentArrayDeterministic (d : Nat) : Prop :=
  ∀ (wire : List w8) (a b : SegmentArrayOutcome),
    SegmentArraySpec d wire a → SegmentArraySpec d wire b → a = b

end github_com.NethermindEth.juno.core
end Perennial
