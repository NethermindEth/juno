import ProofCheck.ResourceMapSpec
import ProofCheck.ResourceMapEntriesUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The complete map relation has a unique value and suffix or diagnostic.
    The contextual count guard is resolved before outer depth entry; successful
    and rejected entry sequences cannot overlap. -/
theorem resourceMapSpec_unique (d : Nat) (wire : List w8) (a b : ResourceMapOutcome)
    (first : ResourceMapSpec d wire a) (second : ResourceMapSpec d wire b) : a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | nullish head tail hn =>
    cases second with
    | nullish head2 tail2 hn2 =>
      simp only [List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      rfl
    | @outer wire2 reason2 hn2 hu2 failure2 =>
      rw [← same] at hn2 hu2
      rcases hn with rfl | rfl
      · exact False.elim (hn2 rfl)
      · exact False.elim (hu2 rfl)
    | @count head2 payload2 tail2 argument2 count2 enc2 major2 size2 short2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | @depth head2 payload2 tail2 argument2 count2 enc2 major2 size2 fit2 deep2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | @map head2 payload2 item2 rest2 argument2 values2 count2 enc2 major2 size2 fit2 depth2 valid2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | @element head2 payload2 tail2 argument2 done2 retained2 consumed2 offset2 entered2 reason2 count2 enc2 major2 size2 fit2 depth2 failure2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @outer wire reason hn hu failure =>
    cases second with
    | nullish head2 tail2 hn2 =>
      rw [same] at hn hu
      rcases hn2 with rfl | rfl
      · exact False.elim (hn rfl)
      · exact False.elim (hu rfl)
    | @outer wire2 reason2 hn2 hu2 failure2 =>
      rw [same] at failure
      have eq := expectedFailure_reason_unique (W8 160) _ _ _ failure failure2
      subst reason2
      rfl
    | @count head2 payload2 tail2 argument2 count2 enc2 major2 size2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 160) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using same) failure)
    | @depth head2 payload2 tail2 argument2 count2 enc2 major2 size2 fit2 deep2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 160) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using same) failure)
    | @map head2 payload2 item2 rest2 argument2 values2 count2 enc2 major2 size2 fit2 depth2 valid2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 160) head2 payload2 (item2 ++ rest2)
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using same) failure)
    | @element head2 payload2 tail2 argument2 done2 retained2 consumed2 offset2 entered2 reason2 count2 enc2 major2 size2 fit2 depth2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 160) head2 payload2 tail2
        _ argument2 _ enc2 major2 (by simpa only [List.append_assoc] using same) failure)
  | @count head payload tail argument count enc major size short =>
    cases second with
    | nullish head2 tail2 hn2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn2 with rfl | rfl <;> contradiction
    | @outer wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 160) head payload tail
        _ argument _ enc major (by simpa only [List.append_assoc] using same.symm) failure2)
    | @count head2 payload2 tail2 argument2 count2 enc2 major2 size2 short2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      rfl
    | @depth head2 payload2 tail2 argument2 count2 enc2 major2 size2 fit2 deep2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      omega
    | @map head2 payload2 item2 rest2 argument2 values2 count2 enc2 major2 size2 fit2 depth2 valid2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (item2 ++ rest2) argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      have lengthsEq := congrArg List.length tailsEq
      omega
    | @element head2 payload2 tail2 argument2 done2 retained2 consumed2 offset2 entered2 reason2 count2 enc2 major2 size2 fit2 depth2 failure2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      omega
  | @depth head payload tail argument count enc major size fit deep =>
    cases second with
    | nullish head2 tail2 hn2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn2 with rfl | rfl <;> contradiction
    | @outer wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 160) head payload tail
        _ argument _ enc major (by simpa only [List.append_assoc] using same.symm) failure2)
    | @count head2 payload2 tail2 argument2 count2 enc2 major2 size2 short2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      omega
    | @depth head2 payload2 tail2 argument2 count2 enc2 major2 size2 fit2 deep2 =>
      rfl
    | @map head2 payload2 item2 rest2 argument2 values2 count2 enc2 major2 size2 fit2 depth2 valid2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (item2 ++ rest2) argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      have lengthsEq := congrArg List.length tailsEq
      omega
    | @element head2 payload2 tail2 argument2 done2 retained2 consumed2 offset2 entered2 reason2 count2 enc2 major2 size2 fit2 depth2 failure2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      omega
  | @map head payload item rest argument values count enc major size fit depth valid =>
    cases second with
    | nullish head2 tail2 hn2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn2 with rfl | rfl <;> contradiction
    | @outer wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 160) head payload (item ++ rest)
        _ argument _ enc major (by simpa only [List.append_assoc] using same.symm) failure2)
    | @count head2 payload2 tail2 argument2 count2 enc2 major2 size2 short2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (item ++ rest) tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      have lengthsEq := congrArg List.length tailsEq
      omega
    | @depth head2 payload2 tail2 argument2 count2 enc2 major2 size2 fit2 deep2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (item ++ rest) tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      have lengthsEq := congrArg List.length tailsEq
      omega
    | @map head2 payload2 item2 rest2 argument2 values2 count2 enc2 major2 size2 fit2 depth2 valid2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (item ++ rest) (item2 ++ rest2) argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      obtain ⟨_, rfl, rfl⟩ := resourceMapEntries_prefix_unique (d + 1) count item item2 rest rest2 values values2 valid valid2 tailsEq
      rfl
    | @element head2 payload2 tail2 argument2 done2 retained2 consumed2 offset2 entered2 reason2 count2 enc2 major2 size2 fit2 depth2 failure2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (item ++ rest) tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      exact False.elim (resourceMapEntries_not_rejected (d + 1) count consumed2 offset2 item rest values done2 retained2 entered2 reason2 valid failure2)
  | @element head payload tail argument done retained consumed offset entered reason count enc major size fit depth failure =>
    cases second with
    | nullish head2 tail2 hn2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn2 with rfl | rfl <;> contradiction
    | @outer wire2 reason2 hn2 hu2 failure2 =>
      exact False.elim (expectedFailure_excludes_encoded (W8 160) head payload tail
        _ argument _ enc major (by simpa only [List.append_assoc] using same.symm) failure2)
    | @count head2 payload2 tail2 argument2 count2 enc2 major2 size2 short2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      omega
    | @depth head2 payload2 tail2 argument2 count2 enc2 major2 size2 fit2 deep2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      omega
    | @map head2 payload2 item2 rest2 argument2 values2 count2 enc2 major2 size2 fit2 depth2 valid2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (item2 ++ rest2) argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail
      exact False.elim (resourceMapEntries_not_rejected (d + 1) count consumed offset item2 rest2 values2 done retained entered reason valid2 failure)
    | @element head2 payload2 tail2 argument2 done2 retained2 consumed2 offset2 entered2 reason2 count2 enc2 major2 size2 fit2 depth2 failure2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have countsEq : count = count2 := by omega
      subst count2
      subst tail2
      obtain ⟨_, _, rfl, rfl, rfl, rfl⟩ := resourceMapEntriesRejected_unique (d + 1) count consumed offset consumed2 offset2 tail done done2 retained retained2 entered entered2 reason reason2 failure failure2
      rfl

/-- Every supported-depth input has exactly one independent outcome. -/
theorem resourceMapSpec_total_unique (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, ResourceMapSpec d wire outcome ∧
      ∀ other, ResourceMapSpec d wire other → other = outcome := by
  obtain ⟨outcome, spec⟩ := resourceMap_classify d wire depth
  exact ⟨outcome, spec, fun other otherSpec => resourceMapSpec_unique d wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapSpec_total_unique
