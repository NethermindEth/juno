import ProofCheck.IndexesDisjoint
import ProofCheck.IntsFailureUnique
import ProofCheck.SkipFailureUnique
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Field failures uniquely determine cursor progress, diagnostic offset,
    retained depth and reason, for both selected arrays and ignored values. -/
theorem indexesItemRejected_unique (d consumedA offsetA depthA consumedB offsetB depthB : Nat)
    (choice : Option Bool) (wire : List w8) (a b : IndexesItemFailureReason)
    (bound : wire.length < 2^64)
    (first : IndexesItemRejected d choice wire consumedA offsetA depthA a)
    (second : IndexesItemRejected d choice wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | ignored failure =>
    cases second with
    | ignored failure2 =>
      have result := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, result.1, rfl, congrArg IndexesItemFailureReason.ignored result.2⟩
  | selected failure =>
    cases second with
    | selected failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := intsRejected_unique _ _ _ _ _ _ _ _ _ _ bound failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

/-- Full pair failures uniquely retain the partial Indexes, consumed bytes,
    diagnostic offset and reason, with key/value precedence. -/
theorem indexesPairsRejected_unique (d n consumedA offsetA depthA consumedB offsetB depthB : Nat) (fields : List Bool)
    (initial finalA finalB : IndexesValue) (wire : List w8) (a b : IndexesRecordFailureReason)
    (bound : wire.length < 2^64)
    (first : IndexesPairsRejected d n fields initial wire finalA consumedA offsetA depthA a)
    (second : IndexesPairsRejected d n fields initial wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  induction first generalizing wire2 finalB consumedB offsetB depthB b with
  | @key n consumed fields initial wire reason failure =>
    cases second with
    | @key n2 consumed2 fields2 initial2 wire2 reason2 failure2 =>
      rw [eqWire] at failure
      have result := integerKeyRejected_unique _ _ _ _ _ failure failure2
      exact ⟨rfl, result.1, rfl, rfl, congrArg IndexesRecordFailureReason.key result.2⟩
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 failure2 =>
      exact False.elim (integerKeyValid_not_rejected key2 tail2 choice2 _ _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 valueValid2 failure2 =>
      exact False.elim (integerKeyValid_not_rejected key2 (value2 ++ tail2) choice2 _ _ keyValid2
        (by rw [eqWire] at failure; simpa only [List.append_assoc] using failure))
  | @value n consumed offset extraDepth fields choice initial key tail reason keyValid failure =>
    cases second with
    | @key n2 consumed2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (integerKeyValid_not_rejected key tail choice _ _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := integerKeyValid_prefix_unique
        key key2 tail tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      have tailBound : tail.length < 2^64 := by simp only [List.length_append] at bound; omega
      have diagnostic := indexesItemRejected_unique d consumed offset extraDepth consumed2 offset2 depthB
        (indexesChoice fields choice) tail reason reason2 tailBound failure failure2
      exact ⟨rfl, by have := diagnostic.1; omega, by have := diagnostic.2.1; omega,
        diagnostic.2.2.1, congrArg IndexesRecordFailureReason.value diagnostic.2.2.2⟩
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := integerKeyValid_prefix_unique
        key key2 tail (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail
      have valueBound : (value2 ++ tail2).length < 2^64 := by simp only [List.length_append] at bound ⊢; omega
      exact False.elim (indexesItemValid_not_rejected d consumed offset extraDepth (indexesChoice fields choice) initial middle2 value2 tail2
        reason valueBound valueValid2 failure)
  | @next n consumed offset extraDepth fields choice initial middle final key value tail reason keyValid valueValid failure ih =>
    cases second with
    | @key n2 consumed2 fields2 initial2 wire2 reason2 failure2 =>
      exact False.elim (integerKeyValid_not_rejected key (value ++ tail) choice _ _ keyValid
        (by rw [eqWire.symm] at failure2; simpa only [List.append_assoc] using failure2))
    | @value n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 key2 tail2 reason2 keyValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := integerKeyValid_prefix_unique
        key key2 (value ++ tail) tail2 choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2; subst tail2
      have valueBound : (value ++ tail).length < 2^64 := by simp only [List.length_append] at bound ⊢; omega
      exact False.elim (indexesItemValid_not_rejected d consumed2 offset2 depthB (indexesChoice fields choice) initial middle value tail
        reason2 valueBound valueValid failure2)
    | @next n2 consumed2 offset2 extraDepth2 fields2 choice2 initial2 middle2 final2 key2 value2 tail2 reason2 keyValid2 valueValid2 failure2 =>
      obtain ⟨keyEq, choiceEq, tailsEq⟩ := integerKeyValid_prefix_unique
        key key2 (value ++ tail) (value2 ++ tail2) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using eqWire)
      subst key2
      subst choice2
      obtain ⟨valueEq, middleEq, restEq⟩ := indexesItemValid_prefix_unique d (indexesChoice fields choice) initial middle middle2
        value value2 tail tail2 valueValid valueValid2 tailsEq
      subst value2; subst middle2; subst tail2
      have tailBound : tail.length < 2^64 := by simp only [List.length_append] at bound; omega
      have diagnostic := ih _ _ _ _ _ tailBound _ rfl failure2
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega,
        by have := diagnostic.2.2.1; omega, diagnostic.2.2.2.1, diagnostic.2.2.2.2⟩

/-- Complete Indexes record failures also uniquely determine whether the map
    depth was entered and the exact partially populated record. -/
theorem indexesRecordRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (depthA depthB : Nat)
    (initial finalA finalB : IndexesValue) (wire : List w8) (a b : IndexesRecordFailureReason)
    (bound : wire.length < 2^64)
    (first : IndexesRecordRejected d initial wire finalA consumedA offsetA depthA a)
    (second : IndexesRecordRejected d initial wire finalB consumedB offsetB depthB b) :
    finalA = finalB ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | opening failure =>
    cases second with
    | opening failure2 =>
      rw [eqWire] at failure
      exact ⟨rfl, rfl, rfl, rfl, congrArg IndexesRecordFailureReason.opening (structOpenFailure_reason_unique _ _ _ _ failure failure2)⟩
    | map enc2 major2 length2 depth2 fit2 failure2 =>
      rw [eqWire] at failure
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc2 major2 depth2 (by omega) failure)
  | @map head payload body argument count consumed offset extraDepth final reason enc major length depth fit failure =>
    cases second with
    | opening failure2 =>
      rw [← eqWire] at failure2
      exact False.elim (structOpenFailure_excludes_map _ _ _ _ _ _ enc major depth (by omega) failure2)
    | @map head2 payload2 body2 argument2 count2 consumed2 offset2 extraDepth2 final2 reason2 enc2 major2 length2 depth2 fit2 failure2 =>
      obtain ⟨headEq, payloadEq, argumentEq, bodiesEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
      subst head2; subst payload2; subst argument2; subst body2
      have counts : count = count2 := by omega
      subst count2
      have bodyBound : body.length < 2^64 := by simp only [List.length_append, List.length_cons] at bound; omega
      have diagnostic := indexesPairsRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ bodyBound failure failure2
      exact ⟨diagnostic.1, by have := diagnostic.2.1; omega,
        by have := diagnostic.2.2.1; omega, by have := diagnostic.2.2.2.1; omega, diagnostic.2.2.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesItemRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesPairsRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesRecordRejected_unique
