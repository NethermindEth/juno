import ProofCheck.SierraClassPairsUnique
import ProofCheck.SierraClassRecordSpec
import ProofCheck.StructOpenDisjoint
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem sierraClassRecord_prefix_unique (d : Nat)
    (old a b : SierraClassValue) (left right restA restB : List w8)
    (first : SierraClassRecordValid d old left restA a)
    (second : SierraClassRecordValid d old right restB b)
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
      obtain ⟨bodyEq, valueEq, restEq⟩ := sierraClassPairs_prefix_unique (d + 1) count [] old _ _
        body body2 restA restB valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

theorem sierraClassRecord_not_open_failure (d : Nat)
    (old final : SierraClassValue) (item rest : List w8) (reason : FeltFailureReason)
    (valid : SierraClassRecordValid d old item rest final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish suffix hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := sierraClassPairs_count_bound _ _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

theorem sierraClassRecord_not_rejected (d consumed offset : Nat)
    (old final retained : SierraClassValue) (item rest : List w8) (extraDepth : Nat) (reason : SierraClassFailureReason)
    (valid : SierraClassRecordValid d old item rest final)
    (failure : SierraClassRecordRejected d old (item ++ rest) retained consumed offset extraDepth reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases failure with
  | opening failure =>
    rw [← same] at failure
    exact sierraClassRecord_not_open_failure _ _ _ _ _ _ valid failure
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
      exact sierraClassPairs_not_rejected _ _ _ _ _ _ _ _ _ _ _ _ valid failure

theorem sierraClassRecordRejected_unique (d consumedA offsetA consumedB offsetB : Nat)
    (old finalA finalB : SierraClassValue) (wire : List w8) (depthA depthB : Nat) (a b : SierraClassFailureReason)
    (first : SierraClassRecordRejected d old wire finalA consumedA offsetA depthA a)
    (second : SierraClassRecordRejected d old wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [same] at failure
      exact ⟨rfl, rfl, rfl, rfl,
        congrArg SierraClassFailureReason.opening (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
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
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := sierraClassPairsRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

/-- One independent result contains either the complete value/suffix or the
    retained value, consumed position, diagnostic offset, depth and reason. -/
inductive SierraClassRecordResult where
  | success (value : SierraClassValue) (rest : List w8)
  | failure (retained : SierraClassValue) (consumed offset : Nat) (extraDepth : Nat) (reason : SierraClassFailureReason)

def SierraClassRecordSpec (d : Nat) (old : SierraClassValue) (wire : List w8) : SierraClassRecordResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ SierraClassRecordValid d old item rest value
  | .failure retained consumed offset extraDepth reason => SierraClassRecordRejected d old wire retained consumed offset extraDepth reason

theorem sierraClassRecordSpec_unique (d : Nat) (old : SierraClassValue) (wire : List w8) (a b : SierraClassRecordResult)
    (first : SierraClassRecordSpec d old wire a) (second : SierraClassRecordSpec d old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := sierraClassRecord_prefix_unique d old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset extraDepth reason =>
      rw [split] at second
      exact False.elim (sierraClassRecord_not_rejected d consumed offset old value retained item rest extraDepth reason valid second)
  | failure retained consumed offset extraDepth reason =>
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      rw [split2] at first
      exact False.elim (sierraClassRecord_not_rejected d consumed offset old value2 retained item2 rest2 extraDepth reason valid2 first)
    | failure retained2 consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := sierraClassRecordRejected_unique d consumed offset consumed2 offset2 old retained retained2 wire
        extraDepth extraDepth2 reason reason2 first second
      rfl

/-- Classification plus independent determinism gives exactly one result at
    every supported depth, without assuming any production helper contract. -/
theorem sierraClassRecordSpec_total_unique (d : Nat) (old : SierraClassValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, SierraClassRecordSpec d old wire outcome ∧
      ∀ other, SierraClassRecordSpec d old wire other → other = outcome := by
  rcases sierraClassRecord_classify d old wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
  · refine ⟨.success final rest, ⟨item, split, valid⟩, ?_⟩
    intro other spec
    exact sierraClassRecordSpec_unique d old wire other _ spec ⟨item, split, valid⟩
  · refine ⟨.failure final consumed offset extraDepth reason, failure, ?_⟩
    intro other spec
    exact sierraClassRecordSpec_unique d old wire other _ spec failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassRecord_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassRecord_not_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassRecord_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassRecordRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassRecordSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassRecordSpec_total_unique
