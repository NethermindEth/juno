import ProofCheck.StringSpec
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every new string slot starts empty. Null and undefined elements preserve
    that empty value; text elements must contain valid UTF-8. -/
inductive StringsItemsValid : Nat → List w8 → List GoString → Prop where
  | nil : StringsItemsValid 0 [] []
  | step {n item tail value values}
      (valid : StringValid [] item value) (rest : StringsItemsValid n tail values) :
      StringsItemsValid (n + 1) (item ++ tail) (value :: values)

/-- The failed string rolls back to its own starting position. Earlier strings
    stay consumed in the private array; no element enters container depth. -/
inductive StringsItemsRejected : Nat → List w8 → List GoString → Nat → StringFailureReason → Prop where
  | here {n wire reason} (failure : StringRejected wire reason) :
      StringsItemsRejected (n + 1) wire [] 0 reason
  | next {n item tail value done offset reason}
      (valid : StringValid [] item value)
      (failure : StringsItemsRejected n tail done offset reason) :
      StringsItemsRejected (n + 1) (item ++ tail) (value :: done) (item.length + offset) reason

theorem stringsItems_classify (n : Nat) (wire : List w8) :
    (∃ item rest values, wire = item ++ rest ∧ StringsItemsValid n item values) ∨
    (∃ done offset reason, StringsItemsRejected n wire done offset reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil⟩
  | succ n ih =>
    rcases string_classify [] wire with ⟨item, tail, value, split, valid⟩ | ⟨reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ | ⟨done, offset, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, item.length + offset, reason, .next valid failure⟩
    · exact Or.inr ⟨[], 0, reason, .here failure⟩

theorem stringsItems_count (n : Nat) (item : List w8) (values : List GoString)
    (valid : StringsItemsValid n item values) : values.length = n ∧ n ≤ item.length := by
  induction valid with
  | nil => simp
  | step valid rest ih =>
    have nonempty := stringValid_nonempty _ _ _ valid
    simp only [List.length_cons, List.length_append]
    omega

theorem stringsItemsRejected_bounds (n offset : Nat) (wire : List w8) (done : List GoString)
    (reason : StringFailureReason) (failure : StringsItemsRejected n wire done offset reason) :
    done.length < n ∧ offset ≤ wire.length := by
  induction failure with
  | here failure => simp
  | next valid failure ih => simp only [List.length_cons, List.length_append]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stringsItems_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stringsItems_count
#print axioms Perennial.github_com.NethermindEth.juno.core.stringsItemsRejected_bounds
