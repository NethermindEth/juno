import ProofCheck.DeployPairsUnique
import ProofCheck.DeployRecordSpec
import ProofCheck.StructOpenDisjoint
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem deployRecord_prefix_unique (d : Nat)
    (old a b : DeployValue) (left right restA restB : List w8)
    (first : DeployRecordValid d old left restA a)
    (second : DeployRecordValid d old right restB b)
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
      obtain ⟨bodyEq, valueEq, restEq⟩ := deployPairs_prefix_unique (d + 1) count [] old _ _
        body body2 restA restB valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

theorem deployRecord_not_open_failure (d : Nat)
    (old final : DeployValue) (item rest : List w8) (reason : FeltFailureReason)
    (valid : DeployRecordValid d old item rest final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish suffix hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := deployPairs_count_bound _ _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

theorem deployRecord_not_rejected (d consumed offset : Nat)
    (old final retained : DeployValue) (item rest : List w8) (entered : Bool) (reason : DeployFailureReason)
    (valid : DeployRecordValid d old item rest final)
    (failure : DeployRecordRejected d old (item ++ rest) retained consumed offset entered reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases failure with
  | opening failure =>
    rw [← same] at failure
    exact deployRecord_not_open_failure _ _ _ _ _ _ valid failure
  | @map head payload body argument final2 consumed offset reason count enc major length depth fit failure =>
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
      exact deployPairs_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid failure

theorem deployRecordRejected_unique (d consumedA offsetA consumedB offsetB : Nat)
    (old finalA finalB : DeployValue) (wire : List w8) (enteredA enteredB : Bool) (a b : DeployFailureReason)
    (first : DeployRecordRejected d old wire finalA consumedA offsetA enteredA a)
    (second : DeployRecordRejected d old wire finalB consumedB offsetB enteredB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [same] at failure
      exact ⟨rfl, rfl, rfl, rfl,
        congrArg DeployFailureReason.opening (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
    | map enc2 major2 length2 depth2 fit2 failure2 =>
      rw [same] at failure
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc2 major2 depth2 (by omega) failure)
  | @map head payload body argument final consumed offset reason count enc major length depth fit failure =>
    cases second with
    | opening failure2 =>
      rw [← same] at failure2
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc major depth (by omega) failure2)
    | @map head2 payload2 body2 argument2 final2 consumed2 offset2 reason2 count2 enc2 major2 length2 depth2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, bodiesEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
      subst head2; subst payload2; subst argument2; subst body2
      have counts : count = count2 := by omega
      subst count2
      obtain ⟨rfl, rfl, rfl, rfl⟩ := deployPairsRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

/-- One independent result contains either the complete value/suffix or the
    retained value, consumed position, diagnostic offset, depth and reason. -/
inductive DeployRecordResult where
  | success (value : DeployValue) (rest : List w8)
  | failure (retained : DeployValue) (consumed offset : Nat) (entered : Bool) (reason : DeployFailureReason)

def DeployRecordSpec (d : Nat) (old : DeployValue) (wire : List w8) : DeployRecordResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ DeployRecordValid d old item rest value
  | .failure retained consumed offset entered reason => DeployRecordRejected d old wire retained consumed offset entered reason

theorem deployRecordSpec_unique (d : Nat) (old : DeployValue) (wire : List w8) (a b : DeployRecordResult)
    (first : DeployRecordSpec d old wire a) (second : DeployRecordSpec d old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := deployRecord_prefix_unique d old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset entered reason =>
      rw [split] at second
      exact False.elim (deployRecord_not_rejected d consumed offset old value retained item rest entered reason valid second)
  | failure retained consumed offset entered reason =>
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      rw [split2] at first
      exact False.elim (deployRecord_not_rejected d consumed offset old value2 retained item2 rest2 entered reason valid2 first)
    | failure retained2 consumed2 offset2 entered2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := deployRecordRejected_unique d consumed offset consumed2 offset2 old retained retained2 wire
        entered entered2 reason reason2 first second
      rfl

/-- Classification plus independent determinism gives exactly one result at
    every supported depth, without assuming any production helper salt. -/
theorem deployRecordSpec_total_unique (d : Nat) (old : DeployValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, DeployRecordSpec d old wire outcome ∧
      ∀ other, DeployRecordSpec d old wire other → other = outcome := by
  rcases deployRecord_classify d old wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · refine ⟨.success final rest, ⟨item, split, valid⟩, ?_⟩
    intro other spec
    exact deployRecordSpec_unique d old wire other _ spec ⟨item, split, valid⟩
  · refine ⟨.failure final consumed offset entered reason, failure, ?_⟩
    intro other spec
    exact deployRecordSpec_unique d old wire other _ spec failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployRecord_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployRecord_not_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.deployRecord_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.deployRecordRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployRecordSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployRecordSpec_total_unique
