import ProofCheck.WantedDeterminism
import ProofCheck.KeyFailureUnique
import ProofCheck.SkipFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Rejected projection scans determine their exact cursor consumption,
    diagnostic position, retained pair count and reason. -/
theorem wantedRejected_unique (d n consumedA offsetA leftA consumedB offsetB leftB : Nat)
    (names : List GoString) (seen : List Nat) (wire : List w8) (a b : WantedFailureReason)
    (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (first : WantedRejected d names seen n wire consumedA offsetA leftA a)
    (second : WantedRejected d names seen n wire consumedB offsetB leftB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ leftA = leftB ∧ a = b := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 consumedB offsetB leftB b with
  | @key n wire reason failure =>
    cases second with
    | key failure2 =>
      rw [← same] at failure2
      exact ⟨rfl, rfl, rfl, congrArg WantedFailureReason.key (keyRejected_reason_unique _ _ _ failure failure2)⟩
    | @skipped n2 offset2 key2 tail2 choice2 reason2 valid2 ignored2 failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning names) key2 tail2 choice2 _ valid2
        (by rw [same] at failure; exact failure))
    | @next n2 consumed2 offset2 left2 key2 value2 tail2 choice2 reason2 valid2 ignored2 valueValid2 failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning names) key2 (value2 ++ tail2) choice2 _ valid2
        (by rw [same] at failure; simpa only [List.append_assoc] using failure))
  | @skipped n offset key tail choice reason valid ignored failure =>
    cases second with
    | key failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning names) key tail choice _ valid
        (by rw [← same] at failure2; exact failure2))
    | @skipped n2 offset2 key2 tail2 choice2 reason2 valid2 ignored2 failure2 =>
      obtain ⟨rfl, _, rfl⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key key2 tail tail2 choice choice2 valid valid2 same
      obtain ⟨rfl, rfl⟩ := skipFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
    | @next n2 consumed2 offset2 left2 key2 value2 tail2 choice2 reason2 valid2 ignored2 valueValid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key key2 tail (value2 ++ tail2)
        choice choice2 valid valid2 (by simpa only [List.append_assoc] using same)
      subst tail
      exact False.elim (skipValid_not_failure d offset value2 tail2 reason valueValid2 failure)
  | @next n consumed offset left key value tail choice reason valid ignored valueValid failure ih =>
    cases second with
    | key failure2 =>
      exact False.elim (schemaKey_not_rejected (WantedKeyMeaning names) key (value ++ tail) choice _ valid
        (by rw [← same] at failure2; simpa only [List.append_assoc] using failure2))
    | @skipped n2 offset2 key2 tail2 choice2 reason2 valid2 ignored2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key key2 (value ++ tail) tail2
        choice choice2 valid valid2 (by simpa only [List.append_assoc] using same)
      subst tail2
      exact False.elim (skipValid_not_failure d offset2 value tail reason2 valueValid failure2)
    | @next n2 consumed2 offset2 left2 key2 value2 tail2 choice2 reason2 valid2 ignored2 valueValid2 failure2 =>
      obtain ⟨keyEq, _, valuesEq⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key key2 (value ++ tail) (value2 ++ tail2)
        choice choice2 valid valid2 (by simpa only [List.append_assoc] using same)
      obtain ⟨valueEq, tailEq⟩ := skipValid_prefix_unique d value value2 tail tail2 valueValid valueValid2 valuesEq
      subst key2; subst value2; subst tail2
      obtain ⟨rfl, rfl, rfl, rfl⟩ := ih _ _ _ _ _ rfl failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

/-- Complete abstract scan outcomes retain the unread suffix on success and
    the independent consumption/diagnostic positions on failure. -/
inductive WantedOutcome where
  | success (result : WantedResult) (rest : List w8)
  | failure (consumed offset left : Nat) (reason : WantedFailureReason)

def WantedSpec (d n : Nat) (names : List GoString) (seen : List Nat)
    (wire : List w8) (outcome : WantedOutcome) : Prop :=
  match outcome with
  | .success result rest => ∃ item, wire = item ++ rest ∧ WantedValid d names seen n item result
  | .failure consumed offset left reason => WantedRejected d names seen n wire consumed offset left reason

theorem wantedSpec_unique (d n : Nat) (names : List GoString) (seen : List Nat) (wire : List w8)
    (a b : WantedOutcome) (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (first : WantedSpec d n names seen wire a) (second : WantedSpec d n names seen wire b) : a = b := by
  cases a with
  | success result rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success result2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := wantedValid_prefix_unique d n names seen item item2 rest rest2 result result2 distinct valid valid2
        (split.symm.trans split2)
      rfl
    | failure consumed offset left reason =>
      subst wire
      exact False.elim (wantedValid_not_rejected d n consumed offset left names seen item rest result reason distinct valid second)
  | failure consumed offset left reason =>
    cases b with
    | success result2 rest2 =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (wantedValid_not_rejected d n consumed offset left names seen item rest2 result2 reason distinct valid first)
    | failure consumed2 offset2 left2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := wantedRejected_unique d n consumed offset left consumed2 offset2 left2 names seen wire
        reason reason2 distinct first second
      rfl

/-- Every input has exactly one specified scan result under supported depth
    and distinct folded schema names. This does not prove Go termination. -/
theorem wantedSpec_total_unique (d n : Nat) (names : List GoString) (seen : List Nat) (wire : List w8)
    (depth : d ≤ 32) (distinct : (names.map (List.map fieldFoldByte)).Nodup) :
    ∃ result, WantedSpec d n names seen wire result ∧
      ∀ other, WantedSpec d n names seen wire other → other = result := by
  have existsResult : ∃ result, WantedSpec d n names seen wire result := by
    rcases wanted_classify d n names seen wire depth with ⟨item, rest, result, split, valid⟩ | ⟨consumed, offset, left, reason, failure⟩
    · exact ⟨.success result rest, item, split, valid⟩
    · exact ⟨.failure consumed offset left reason, failure⟩
  obtain ⟨result, spec⟩ := existsResult
  exact ⟨result, spec, fun other otherSpec => wantedSpec_unique d n names seen wire other result distinct otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedSpec_total_unique
