import ProofCheck.WantedSpec
import ProofCheck.SkipDeterminism
import ProofCheck.SkipDisjoint

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- With distinct folded schema names, a successful projection scan fixes its
    selected field/remaining count (or Done), consumed prefix and unread suffix. -/
theorem wantedValid_prefix_unique (d n : Nat) (names : List GoString) (seen : List Nat)
    (left right restA restB : List w8) (a b : WantedResult)
    (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (first : WantedValid d names seen n left a) (second : WantedValid d names seen n right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right b restA restB with
  | done =>
    cases second
    simpa using same
  | @selected n index key choice valid fresh =>
    cases second with
    | @selected n2 index2 key2 choice2 valid2 fresh2 =>
      obtain ⟨rfl, rfl, rfl⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key _ restA restB choice choice2 valid valid2 same
      have indices : index = index2 := Option.some.inj (fresh.symm.trans fresh2)
      subst index2
      exact ⟨rfl, rfl, rfl⟩
    | @skipped n2 key2 value2 tail2 choice2 result2 valid2 ignored2 valueValid2 rest2 =>
      obtain ⟨_, choiceEq, _⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key _ restA
        (value2 ++ tail2 ++ restB) choice choice2 valid valid2 (by simpa only [List.append_assoc] using same)
      subst choice2
      rw [ignored2] at fresh
      cases fresh
  | @skipped n key value tail choice result keyValid ignored valueValid rest ih =>
    cases second with
    | @selected n2 index2 key2 choice2 valid2 fresh2 =>
      obtain ⟨_, choiceEq, _⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key _
        (value ++ tail ++ restA) restB choice choice2 keyValid valid2 (by simpa only [List.append_assoc] using same)
      subst choice2
      rw [ignored] at fresh2
      cases fresh2
    | @skipped n2 key2 value2 tail2 choice2 result2 keyValid2 ignored2 valueValid2 rest2 =>
      obtain ⟨keyEq, _, valuesEq⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key _
        (value ++ tail ++ restA) (value2 ++ tail2 ++ restB) choice choice2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      obtain ⟨valueEq, tailsEq⟩ := skipValid_prefix_unique d value value2 (tail ++ restA) (tail2 ++ restB)
        valueValid valueValid2 (by simpa only [List.append_assoc] using valuesEq)
      obtain ⟨tailEq, resultEq, suffixEq⟩ := ih _ _ _ _ rest2 tailsEq
      exact ⟨by rw [keyEq, valueEq, tailEq], resultEq, suffixEq⟩

/-- A successful scan cannot also fail a key or ignored value, including
    failures after any number of already-skipped pairs. -/
theorem wantedValid_not_rejected (d n consumed offset left : Nat) (names : List GoString) (seen : List Nat)
    (item rest : List w8) (result : WantedResult) (reason : WantedFailureReason)
    (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (valid : WantedValid d names seen n item result)
    (failure : WantedRejected d names seen n (item ++ rest) consumed offset left reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing wire rest consumed offset left reason with
  | done => cases failure
  | @selected n index key choice valid fresh =>
    cases failure with
    | key rejected =>
      rw [← same] at rejected
      exact schemaKey_not_rejected (WantedKeyMeaning names) key rest choice _ valid rejected
    | @skipped n2 offset2 key2 tail2 choice2 reason2 valid2 ignored2 rejected =>
      obtain ⟨_, choiceEq, _⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key _ rest tail2 choice choice2 valid valid2 same
      subst choice2
      rw [ignored2] at fresh
      cases fresh
    | @next n2 consumed2 offset2 left2 key2 value2 tail2 choice2 reason2 valid2 ignored2 valueValid2 rejected =>
      obtain ⟨_, choiceEq, _⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key _ rest (value2 ++ tail2)
        choice choice2 valid valid2 (by simpa only [List.append_assoc] using same)
      subst choice2
      rw [ignored2] at fresh
      cases fresh
  | @skipped n key value tail choice result keyValid ignored valueValid restValid ih =>
    cases failure with
    | key rejected =>
      apply schemaKey_not_rejected (WantedKeyMeaning names) key (value ++ tail ++ rest) choice _ keyValid
      rw [← same] at rejected
      simpa only [List.append_assoc] using rejected
    | @skipped n2 offset2 key2 tail2 choice2 reason2 valid2 ignored2 rejected =>
      obtain ⟨_, _, valuesEq⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key _ (value ++ tail ++ rest) tail2
        choice choice2 keyValid valid2 (by simpa only [List.append_assoc] using same)
      subst tail2
      exact skipValid_not_failure d offset2 value (tail ++ rest) reason2 valueValid
        (by simpa only [List.append_assoc] using rejected)
    | @next n2 consumed2 offset2 left2 key2 value2 tail2 choice2 reason2 valid2 ignored2 valueValid2 rejected =>
      obtain ⟨_, _, valuesEq⟩ := schemaKey_prefix_unique (WantedKeyMeaning names)
        (fun key a b => wantedKeyMeaning_unique names key a b distinct) key _ (value ++ tail ++ rest) (value2 ++ tail2)
        choice choice2 keyValid valid2 (by simpa only [List.append_assoc] using same)
      obtain ⟨_, tailsEq⟩ := skipValid_prefix_unique d value value2 (tail ++ rest) tail2
        valueValid valueValid2 (by simpa only [List.append_assoc] using valuesEq)
      exact ih _ _ _ _ _ _ tailsEq rejected

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedValid_not_rejected
