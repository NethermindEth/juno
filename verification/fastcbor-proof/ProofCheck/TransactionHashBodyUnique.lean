import ProofCheck.TransactionHashPairsUnique
import ProofCheck.TransactionHashRecord
import ProofCheck.StructOpenDisjoint
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem transactionHashBody_prefix_unique (d : Nat) 
    (old a b : TransactionHashValue) (left right restA restB : List w8)
    (first : TransactionHashBodyValid d old left a)
    (second : TransactionHashBodyValid d old right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | map enc major size depth valid =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @map head payload body argument final count enc major size depth valid =>
    cases second with
    | @nullish other ho =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @map other payload2 body2 argument2 final2 count2 enc2 major2 size2 depth2 valid2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head other payload payload2
        (body ++ restA) (body2 ++ restB) argument argument2 enc enc2 (by simpa only [List.append_assoc] using same)
      subst other; subst payload2; subst argument2
      have counts : count = count2 := by omega
      subst count2
      obtain ⟨bodyEq, valueEq, restEq⟩ := transactionHashPairs_prefix_unique (d + 1) count [] old _ _
        body body2 restA restB valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

theorem transactionHashBody_not_open_failure (d : Nat) 
    (old final : TransactionHashValue) (item rest : List w8) (reason : FeltFailureReason)
    (valid : TransactionHashBodyValid d old item final)
    (failure : StructOpenFailure d (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    have excluded := structOpenFailure_not_nullish d _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | map enc major length depth pairs =>
    have count := transactionHashPairs_count_bound _ _ _ _ _ _ pairs
    refine structOpenFailure_excludes_map d _ _ _ _ reason enc major depth ?_
      (by simpa only [List.append_assoc] using failure)
    simp only [List.length_append]
    omega

theorem transactionHashBody_not_rejected (d consumed offset : Nat) 
    (old final retained : TransactionHashValue) (item rest : List w8) (entered : Bool) (reason : TransactionHashRecordFailure)
    (valid : TransactionHashBodyValid d old item final)
    (failure : TransactionHashBodyRejected d old (item ++ rest) retained consumed offset entered reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases failure with
  | opening failure =>
    rw [← same] at failure
    exact transactionHashBody_not_open_failure _ _ _ _ _ _ valid failure
  | @map head payload body argument final2 consumed offset reason count enc major length depth fit failure =>
    cases valid with
    | nullish hn =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | @map head2 payload2 body2 argument2 final3 count2 enc2 major2 length2 depth2 valid =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head2 head payload2 payload
        (body2 ++ rest) body argument2 argument enc2 enc (by simpa only [List.append_assoc] using same)
      have counts : count2 = count := by rw [argumentEq] at length2; omega
      subst count2; subst body
      exact transactionHashPairs_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid failure

theorem transactionHashBodyRejected_unique (d consumedA offsetA consumedB offsetB : Nat) 
    (old finalA finalB : TransactionHashValue) (wire : List w8) (enteredA enteredB : Bool) (a b : TransactionHashRecordFailure)
    (first : TransactionHashBodyRejected d old wire finalA consumedA offsetA enteredA a)
    (second : TransactionHashBodyRejected d old wire finalB consumedB offsetB enteredB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [same] at failure
      exact ⟨rfl, rfl, rfl, rfl,
        congrArg TransactionHashRecordFailure.opening (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
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
      obtain ⟨rfl, rfl, rfl, rfl⟩ := transactionHashPairsRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashBody_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashBody_not_open_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashBody_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashBodyRejected_unique
