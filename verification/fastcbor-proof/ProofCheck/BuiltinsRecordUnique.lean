import ProofCheck.BuiltinsPairsUnique
import ProofCheck.BuiltinsRecordSpec
import ProofCheck.StructOpenDisjoint
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem builtinsRecord_prefix_unique (d : Nat)
    (old a b : BuiltinsValue) (left right restA restB : List w8)
    (first : BuiltinsRecordValid d old left restA a)
    (second : BuiltinsRecordValid d old right restB b)
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
      obtain ⟨bodyEq, valueEq, restEq⟩ := builtinsPairs_prefix_unique (d + 1) count [] old _ _
        body body2 restA restB valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

theorem builtinsRecord_not_open_failure (d : Nat)
    (old final : BuiltinsValue) (item rest : List w8) (reason : FeltFailureReason)
    (valid : BuiltinsRecordValid d old item rest final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish suffix hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := builtinsPairs_count_bound _ _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

theorem builtinsRecord_not_rejected (d consumed offset : Nat)
    (old final retained : BuiltinsValue) (item rest : List w8) (entered : Bool) (reason : BuiltinsFailureReason)
    (valid : BuiltinsRecordValid d old item rest final)
    (failure : BuiltinsRecordRejected d old (item ++ rest) retained consumed offset entered reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases failure with
  | opening failure =>
    rw [← same] at failure
    exact builtinsRecord_not_open_failure _ _ _ _ _ _ valid failure
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
      exact builtinsPairs_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid failure

theorem builtinsRecordRejected_unique (d consumedA offsetA consumedB offsetB : Nat)
    (old finalA finalB : BuiltinsValue) (wire : List w8) (enteredA enteredB : Bool) (a b : BuiltinsFailureReason)
    (first : BuiltinsRecordRejected d old wire finalA consumedA offsetA enteredA a)
    (second : BuiltinsRecordRejected d old wire finalB consumedB offsetB enteredB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [same] at failure
      exact ⟨rfl, rfl, rfl, rfl,
        congrArg BuiltinsFailureReason.opening (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
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
      obtain ⟨rfl, rfl, rfl, rfl⟩ := builtinsPairsRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

/-- One independent result contains either the complete value/suffix or the
    retained value, consumed position, diagnostic offset, depth and reason. -/
inductive BuiltinsRecordResult where
  | success (value : BuiltinsValue) (rest : List w8)
  | failure (retained : BuiltinsValue) (consumed offset : Nat) (entered : Bool) (reason : BuiltinsFailureReason)

def BuiltinsRecordSpec (d : Nat) (old : BuiltinsValue) (wire : List w8) : BuiltinsRecordResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ BuiltinsRecordValid d old item rest value
  | .failure retained consumed offset entered reason => BuiltinsRecordRejected d old wire retained consumed offset entered reason

theorem builtinsRecordSpec_unique (d : Nat) (old : BuiltinsValue) (wire : List w8) (a b : BuiltinsRecordResult)
    (first : BuiltinsRecordSpec d old wire a) (second : BuiltinsRecordSpec d old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := builtinsRecord_prefix_unique d old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset entered reason =>
      rw [split] at second
      exact False.elim (builtinsRecord_not_rejected d consumed offset old value retained item rest entered reason valid second)
  | failure retained consumed offset entered reason =>
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      rw [split2] at first
      exact False.elim (builtinsRecord_not_rejected d consumed offset old value2 retained item2 rest2 entered reason valid2 first)
    | failure retained2 consumed2 offset2 entered2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := builtinsRecordRejected_unique d consumed offset consumed2 offset2 old retained retained2 wire
        entered entered2 reason reason2 first second
      rfl

/-- Classification plus independent determinism gives exactly one result at
    every supported depth, without assuming any production helper contract. -/
theorem builtinsRecordSpec_total_unique (d : Nat) (old : BuiltinsValue) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, BuiltinsRecordSpec d old wire outcome ∧
      ∀ other, BuiltinsRecordSpec d old wire other → other = outcome := by
  rcases builtinsRecord_classify d old wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨final, consumed, offset, entered, reason, failure⟩
  · refine ⟨.success final rest, ⟨item, split, valid⟩, ?_⟩
    intro other spec
    exact builtinsRecordSpec_unique d old wire other _ spec ⟨item, split, valid⟩
  · refine ⟨.failure final consumed offset entered reason, failure, ?_⟩
    intro other spec
    exact builtinsRecordSpec_unique d old wire other _ spec failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecord_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecord_not_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecord_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecordRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecordSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsRecordSpec_total_unique
