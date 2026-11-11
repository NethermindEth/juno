import ProofCheck.StateUpdatePairsUnique
import ProofCheck.StateUpdateRecordSpec
import ProofCheck.StructOpenDisjoint
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem stateUpdateRecord_prefix_unique (d : Nat)
    (old a b : StateUpdateValue) (left right restA restB : List w8)
    (first : StateUpdateRecordValid d old left restA a)
    (second : StateUpdateRecordValid d old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @nullish head suffix hn =>
    cases second with
    | @nullish other suffix2 ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | map enc major size depth valid =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @map head payload body suffix argument final count enc major size depth valid =>
    cases second with
    | @nullish other suffix2 ho =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @map other payload2 body2 suffix2 argument2 final2 count2 enc2 major2 size2 depth2 valid2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head other payload payload2
        (body ++ restA) (body2 ++ restB) argument argument2 enc enc2 (by simpa only [List.append_assoc] using same)
      subst other; subst payload2; subst argument2
      have counts : count = count2 := by omega
      subst count2
      obtain ⟨bodyEq, valueEq, restEq⟩ := stateUpdatePairs_prefix_unique (d + 1) count [] old _ _
        body body2 restA restB valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

theorem stateUpdateRecord_not_open_failure (d : Nat)
    (old final : StateUpdateValue) (item rest : List w8) (reason : FeltFailureReason)
    (valid : StateUpdateRecordValid d old item rest final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish suffix hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := stateUpdatePairs_count_bound _ _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

theorem stateUpdateRecord_not_rejected (d consumed offset : Nat)
    (old final retained : StateUpdateValue) (item rest : List w8) (extraDepth : Nat) (reason : StateUpdateFailureReason)
    (valid : StateUpdateRecordValid d old item rest final)
    (failure : StateUpdateRecordRejected d old (item ++ rest) retained consumed offset extraDepth reason) (bound : (item ++ rest).length < 2^64) : False := by
  generalize same : item ++ rest = wire at failure
  cases failure with
  | opening failure =>
    rw [← same] at failure
    exact stateUpdateRecord_not_open_failure _ _ _ _ _ _ valid failure
  | @map head payload body argument final2 consumed offset nestedDepth reason count enc major length depth fit failure =>
    cases valid with
    | nullish suffix hn =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | @map head2 payload2 body2 suffix2 argument2 final3 count2 enc2 major2 length2 depth2 valid =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head2 head payload2 payload
        (body2 ++ rest) body argument2 argument enc2 enc (by simpa only [List.append_assoc] using same)
      have counts : count2 = count := by rw [argumentEq] at length2; omega
      subst count2; subst body
      exact stateUpdatePairs_not_rejected _ _ _ _ _ _ _ _ _ _ _ _ valid failure (by
        have lengths := congrArg List.length same
        simp only [List.length_append, List.length_cons] at bound lengths ⊢
        omega)

theorem stateUpdateRecordRejected_unique (d consumedA offsetA consumedB offsetB : Nat)
    (old finalA finalB : StateUpdateValue) (wire : List w8) (depthA depthB : Nat) (a b : StateUpdateFailureReason)
    (first : StateUpdateRecordRejected d old wire finalA consumedA offsetA depthA a)
    (second : StateUpdateRecordRejected d old wire finalB consumedB offsetB depthB b) (bound : wire.length < 2^64) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [same] at failure
      exact ⟨rfl, rfl, rfl, rfl,
        congrArg StateUpdateFailureReason.opening (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
    | map enc2 major2 length2 depth2 fit2 failure2 =>
      rw [same] at failure
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc2 major2 depth2 (by omega) failure)
  | @map head payload body argument final consumed offset nestedDepth reason count enc major length depth fit failure =>
    cases second with
    | opening failure2 =>
      rw [← same] at failure2
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc major depth (by omega) failure2)
    | @map head2 payload2 body2 argument2 final2 consumed2 offset2 nestedDepth2 reason2 count2 enc2 major2 length2 depth2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, bodiesEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      subst head2; subst payload2; subst argument2; subst body2
      have counts : count = count2 := by omega
      subst count2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := stateUpdatePairsRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2 (by simp only [List.length_append, List.length_cons] at bound ⊢; omega)
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

/-- One independent result contains either the complete value/suffix or the
    retained value, consumed position, diagnostic offset, depth and reason. -/
inductive StateUpdateRecordResult where
  | success (value : StateUpdateValue) (rest : List w8)
  | failure (retained : StateUpdateValue) (consumed offset : Nat) (extraDepth : Nat) (reason : StateUpdateFailureReason)

def StateUpdateRecordSpec (d : Nat) (old : StateUpdateValue) (wire : List w8) : StateUpdateRecordResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ StateUpdateRecordValid d old item rest value
  | .failure retained consumed offset extraDepth reason => StateUpdateRecordRejected d old wire retained consumed offset extraDepth reason

theorem stateUpdateRecordSpec_unique (d : Nat) (old : StateUpdateValue) (wire : List w8) (a b : StateUpdateRecordResult)
    (first : StateUpdateRecordSpec d old wire a) (second : StateUpdateRecordSpec d old wire b) (bound : wire.length < 2^64) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := stateUpdateRecord_prefix_unique d old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset extraDepth reason =>
      rw [split] at second
      exact False.elim (stateUpdateRecord_not_rejected d consumed offset old value retained item rest extraDepth reason valid second (by simpa only [split] using bound))
  | failure retained consumed offset extraDepth reason =>
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      rw [split2] at first
      exact False.elim (stateUpdateRecord_not_rejected d consumed offset old value2 retained item2 rest2 extraDepth reason valid2 first (by simpa only [split2] using bound))
    | failure retained2 consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := stateUpdateRecordRejected_unique d consumed offset consumed2 offset2 old retained retained2 wire
        extraDepth extraDepth2 reason reason2 first second bound
      rfl

/-- Classification plus independent determinism gives exactly one result at
    every supported depth, without assuming any production helper contract. -/
theorem stateUpdateRecordSpec_total_unique (d : Nat) (old : StateUpdateValue) (wire : List w8) (depth : d ≤ 32) (bound : wire.length < 2^64) :
    ∃ outcome, StateUpdateRecordSpec d old wire outcome ∧
      ∀ other, StateUpdateRecordSpec d old wire other → other = outcome := by
  rcases stateUpdateRecord_classify d old wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
  · refine ⟨.success final rest, ⟨item, split, valid⟩, ?_⟩
    intro other spec
    exact stateUpdateRecordSpec_unique d old wire other _ spec ⟨item, split, valid⟩ bound
  · refine ⟨.failure final consumed offset extraDepth reason, failure, ?_⟩
    intro other spec
    exact stateUpdateRecordSpec_unique d old wire other _ spec failure bound

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateRecord_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateRecord_not_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateRecord_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateRecordRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateRecordSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateRecordSpec_total_unique
