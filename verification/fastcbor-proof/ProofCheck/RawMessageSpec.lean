import ProofCheck.BytesSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Opaque JSON storage: null/undefined clears the destination; a definite byte
    string supplies its exact bytes without validating JSON or UTF-8. -/
inductive RawMessageValid : List w8 → Option (List w8) → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : RawMessageValid [head] none
  | bytes {item body} (valid : BytesValid item body) : RawMessageValid item (some body)

def RawMessageRejected (wire : List w8) (reason : BytesFailureReason) : Prop :=
  wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) ∧ BytesRejected wire reason

theorem rawMessage_classify (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ RawMessageValid item value) ∨
    (∃ reason, RawMessageRejected wire reason) := by
  by_cases hn : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at hn
    | cons head rest =>
      simp only [List.head?_cons, Option.some.injEq] at hn
      exact Or.inl ⟨[head], rest, none, rfl, .nullish hn⟩
  · have hn1 : wire.head? ≠ some (W8 246) := fun h => hn (Or.inl h)
    have hn2 : wire.head? ≠ some (W8 247) := fun h => hn (Or.inr h)
    rcases bytes_classify wire with ⟨item, rest, body, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .bytes valid⟩
    · exact Or.inr ⟨reason, hn1, hn2, failure⟩

theorem rawMessageValid_nonempty (value : Option (List w8)) (item : List w8)
    (valid : RawMessageValid item value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | bytes valid => exact bytesValid_nonempty _ _ valid

theorem rawMessageValid_prefix_unique (a b : Option (List w8)) (left right restA restB : List w8)
    (first : RawMessageValid left a) (second : RawMessageValid right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | nullish hn =>
    cases second with
    | nullish ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | bytes valid =>
      cases valid with
      | bytes enc major count =>
        simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
        obtain ⟨rfl, _⟩ := same
        rcases hn with rfl | rfl <;> contradiction
  | bytes valid =>
    cases second with
    | nullish hn =>
      cases valid with
      | bytes enc major count =>
        simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
        obtain ⟨rfl, _⟩ := same
        rcases hn with rfl | rfl <;> contradiction
    | bytes valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := bytesValid_prefix_unique _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem rawMessageValid_not_rejected (value : Option (List w8)) (item rest : List w8)
    (reason : BytesFailureReason) (valid : RawMessageValid item value)
    (failure : RawMessageRejected (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    rcases hn with rfl | rfl
    · exact failure.1 rfl
    · exact failure.2.1 rfl
  | bytes valid => exact bytesValid_not_rejected _ _ _ _ valid failure.2.2

inductive RawMessageResult where
  | success (value : Option (List w8)) (rest : List w8)
  | failure (reason : BytesFailureReason)

def RawMessageSpec (wire : List w8) : RawMessageResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ RawMessageValid item value
  | .failure reason => RawMessageRejected wire reason

theorem rawMessageSpec_unique (wire : List w8) (a b : RawMessageResult)
    (first : RawMessageSpec wire a) (second : RawMessageSpec wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := rawMessageValid_prefix_unique value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure reason =>
      subst wire
      exact False.elim (rawMessageValid_not_rejected _ _ _ _ valid second)
  | failure reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (rawMessageValid_not_rejected _ _ _ _ valid first)
    | failure reason2 => exact congrArg RawMessageResult.failure (bytesRejected_unique _ _ _ first.2.2 second.2.2)

theorem rawMessageSpec_total_unique (wire : List w8) :
    ∃ outcome, RawMessageSpec wire outcome ∧ ∀ other, RawMessageSpec wire other → other = outcome := by
  have existsOutcome : ∃ outcome, RawMessageSpec wire outcome := by
    rcases rawMessage_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => rawMessageSpec_unique wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.rawMessage_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.rawMessageValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.rawMessageValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.rawMessageValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.rawMessageSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.rawMessageSpec_total_unique
