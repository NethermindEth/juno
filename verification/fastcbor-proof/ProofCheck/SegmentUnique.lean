import ProofCheck.SegmentRecordUniqueStep
import ProofCheck.SegmentArrayUniqueStep

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Record and array determinism are proved together. Recursive premises in
    the step lemmas are discharged by the actual shared nesting budget. -/
theorem segment_deterministic_depth (fuel : Nat) : ∀ d : Nat, d ≤ 32 → 32 - d = fuel →
    SegmentRecordDeterministic d ∧ SegmentArrayDeterministic d := by
  induction fuel with
  | zero =>
    intro d depth remaining
    have impossible : ¬ d < 32 := by omega
    exact ⟨segmentRecord_deterministic_of_array d (fun inside => False.elim (impossible inside)),
      segmentArraySpec_unique_of_record d (fun inside => False.elim (impossible inside))⟩
  | succ fuel ih =>
    intro d depth remaining
    have next : d < 32 → SegmentRecordDeterministic (d + 1) ∧ SegmentArrayDeterministic (d + 1) := by
      intro inside
      exact ih (d + 1) (by omega) (by omega)
    exact ⟨segmentRecord_deterministic_of_array d (fun inside => (next inside).2),
      segmentArraySpec_unique_of_record d (fun inside => (next inside).1)⟩

theorem segmentRecord_deterministic (d : Nat) (depth : d ≤ 32) : SegmentRecordDeterministic d :=
  (segment_deterministic_depth (32 - d) d depth rfl).1

theorem segmentArray_deterministic (d : Nat) (depth : d ≤ 32) : SegmentArrayDeterministic d :=
  (segment_deterministic_depth (32 - d) d depth rfl).2

theorem segmentRecord_prefix_unique (d : Nat) (depth : d ≤ 32)
    (old a b : SegmentValue) (left right restA restB : List w8)
    (first : SegmentRecordValid d old left restA a)
    (second : SegmentRecordValid d old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB :=
  (segmentRecord_deterministic d depth).successful old a b left right restA restB first second same

theorem segmentRecord_not_rejected (d : Nat) (depth : d ≤ 32) (consumed offset : Nat)
    (old final retained : SegmentValue) (item rest : List w8) (extraDepth : Nat) (reason : SegmentFailureReason)
    (valid : SegmentRecordValid d old item rest final)
    (failure : SegmentRecordRejected d old (item ++ rest) retained consumed offset extraDepth reason) : False :=
  (segmentRecord_deterministic d depth).disjoint consumed offset old final retained item rest extraDepth reason valid failure

theorem segmentRecordRejected_unique (d : Nat) (depth : d ≤ 32) (consumedA offsetA consumedB offsetB : Nat)
    (old finalA finalB : SegmentValue) (wire : List w8) (depthA depthB : Nat) (a b : SegmentFailureReason)
    (first : SegmentRecordRejected d old wire finalA consumedA offsetA depthA a)
    (second : SegmentRecordRejected d old wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b :=
  (segmentRecord_deterministic d depth).rejected consumedA offsetA consumedB offsetB old finalA finalB wire depthA depthB a b first second

theorem segmentArraySpec_unique (d : Nat) (depth : d ≤ 32) (wire : List w8) (a b : SegmentArrayOutcome)
    (first : SegmentArraySpec d wire a) (second : SegmentArraySpec d wire b) : a = b :=
  segmentArray_deterministic d depth wire a b first second

/-- Every supported-depth input has exactly one array value/suffix or failure. -/
theorem segmentArraySpec_total_unique (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, SegmentArraySpec d wire outcome ∧
      ∀ other, SegmentArraySpec d wire other → other = outcome := by
  obtain ⟨outcome, spec⟩ := segmentArray_classify d wire depth
  exact ⟨outcome, spec, fun other otherSpec => segmentArraySpec_unique d depth wire other outcome otherSpec spec⟩

/-- One independent result contains either the complete value/suffix or the
    retained value, consumed position, diagnostic offset, depth and reason. -/
inductive SegmentRecordResult where
  | success (value : SegmentValue) (rest : List w8)
  | failure (retained : SegmentValue) (consumed offset : Nat) (extraDepth : Nat) (reason : SegmentFailureReason)

def SegmentRecordSpec (d : Nat) (old : SegmentValue) (wire : List w8) : SegmentRecordResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ SegmentRecordValid d old item rest value
  | .failure retained consumed offset extraDepth reason => SegmentRecordRejected d old wire retained consumed offset extraDepth reason

theorem segmentRecordSpec_unique (d : Nat) (depth : d ≤ 32) (old : SegmentValue) (wire : List w8) (a b : SegmentRecordResult)
    (first : SegmentRecordSpec d old wire a) (second : SegmentRecordSpec d old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := segmentRecord_prefix_unique d depth old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset extraDepth reason =>
      rw [split] at second
      exact False.elim (segmentRecord_not_rejected d depth consumed offset old value retained item rest extraDepth reason valid second)
  | failure retained consumed offset extraDepth reason =>
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      rw [split2] at first
      exact False.elim (segmentRecord_not_rejected d depth consumed offset old value2 retained item2 rest2 extraDepth reason valid2 first)
    | failure retained2 consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := segmentRecordRejected_unique d depth consumed offset consumed2 offset2 old retained retained2 wire
        extraDepth extraDepth2 reason reason2 first second
      rfl

/-- Classification plus independent determinism gives exactly one result at
    every supported depth, without assuming any production helper contract. -/
theorem segmentRecordSpec_total_unique (d : Nat) (old : SegmentValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, SegmentRecordSpec d old wire outcome ∧
      ∀ other, SegmentRecordSpec d old wire other → other = outcome := by
  rcases segmentRecord_classify d old wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
  · refine ⟨.success final rest, ⟨item, split, valid⟩, ?_⟩
    intro other spec
    exact segmentRecordSpec_unique d depth old wire other _ spec ⟨item, split, valid⟩
  · refine ⟨.failure final consumed offset extraDepth reason, failure, ?_⟩
    intro other spec
    exact segmentRecordSpec_unique d depth old wire other _ spec failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segment_deterministic_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_deterministic
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArray_deterministic
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecordRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArraySpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArraySpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecordSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecordSpec_total_unique
