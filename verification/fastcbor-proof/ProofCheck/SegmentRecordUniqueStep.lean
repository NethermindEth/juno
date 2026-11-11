import ProofCheck.SegmentPairsUnique
import ProofCheck.StructOpenDisjoint
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem segmentRecord_prefix_unique_of_array (d : Nat) (next : d < 32 → SegmentArrayDeterministic (d + 1))
    (old a b : SegmentValue) (left right restA restB : List w8)
    (first : SegmentRecordValid d old left restA a)
    (second : SegmentRecordValid d old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @nullish _ _ head suffix hn =>
    cases second with
    | @nullish _ _ other suffix2 ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | map enc major size depth valid =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @map _ _ head payload body suffix argument final count enc major size depth valid =>
    cases second with
    | @nullish _ _ other suffix2 ho =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @map _ _ other payload2 body2 suffix2 argument2 final2 count2 enc2 major2 size2 depth2 valid2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head other payload payload2
        (body ++ restA) (body2 ++ restB) argument argument2 enc enc2 (by simpa only [List.append_assoc] using same)
      subst other; subst payload2; subst argument2
      have counts : count = count2 := by omega
      subst count2
      obtain ⟨bodyEq, valueEq, restEq⟩ := segmentPairs_prefix_unique_of_array (d + 1) (next depth) count [] old _ _
        body body2 restA restB valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

theorem segmentRecord_not_open_failure (d : Nat)
    (old final : SegmentValue) (item rest : List w8) (reason : FeltFailureReason)
    (valid : SegmentRecordValid d old item rest final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish suffix hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := segmentPairs_count_bound _ _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

theorem segmentRecord_not_rejected_of_array (d : Nat) (next : d < 32 → SegmentArrayDeterministic (d + 1)) (consumed offset : Nat)
    (old final retained : SegmentValue) (item rest : List w8) (extraDepth : Nat) (reason : SegmentFailureReason)
    (valid : SegmentRecordValid d old item rest final)
    (failure : SegmentRecordRejected d old (item ++ rest) retained consumed offset extraDepth reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases failure with
  | opening failure =>
    rw [← same] at failure
    exact segmentRecord_not_open_failure _ _ _ _ _ _ valid failure
  | @map _ _ head payload body argument final2 consumed offset nestedDepth reason count enc major length depth fit failure =>
    cases valid with
    | nullish suffix hn =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | @map _ _ head2 payload2 body2 suffix2 argument2 final3 count2 enc2 major2 length2 depth2 valid =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head2 head payload2 payload
        (body2 ++ rest) body argument2 argument enc2 enc (by simpa only [List.append_assoc] using same)
      have counts : count2 = count := by rw [argumentEq] at length2; omega
      subst count2; subst body
      exact segmentPairs_not_rejected_of_array _ (next depth) _ _ _ _ _ _ _ _ _ _ _ valid failure

theorem segmentRecordRejected_unique_of_array (d : Nat) (next : d < 32 → SegmentArrayDeterministic (d + 1)) (consumedA offsetA consumedB offsetB : Nat)
    (old finalA finalB : SegmentValue) (wire : List w8) (depthA depthB : Nat) (a b : SegmentFailureReason)
    (first : SegmentRecordRejected d old wire finalA consumedA offsetA depthA a)
    (second : SegmentRecordRejected d old wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [same] at failure
      exact ⟨rfl, rfl, rfl, rfl,
        congrArg SegmentFailureReason.opening (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
    | map enc2 major2 length2 depth2 fit2 failure2 =>
      rw [same] at failure
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc2 major2 depth2 (by omega) failure)
  | @map _ _ head payload body argument final consumed offset nestedDepth reason count enc major length depth fit failure =>
    cases second with
    | opening failure2 =>
      rw [← same] at failure2
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc major depth (by omega) failure2)
    | @map _ _ head2 payload2 body2 argument2 final2 consumed2 offset2 nestedDepth2 reason2 count2 enc2 major2 length2 depth2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, bodiesEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      subst head2; subst payload2; subst argument2; subst body2
      have counts : count = count2 := by omega
      subst count2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := segmentPairsRejected_unique_of_array _ (next depth) _ _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩


/-- Record determinism follows from determinism of the next-depth child array. -/
theorem segmentRecord_deterministic_of_array (d : Nat)
    (next : d < 32 → SegmentArrayDeterministic (d + 1)) : SegmentRecordDeterministic d :=
  ⟨segmentRecord_prefix_unique_of_array d next,
   segmentRecord_not_rejected_of_array d next,
   segmentRecordRejected_unique_of_array d next⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_prefix_unique_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_not_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_not_rejected_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecordRejected_unique_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_deterministic_of_array
