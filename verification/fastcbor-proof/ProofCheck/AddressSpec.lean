import ProofCheck.BytesIntoSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Addresses retain their existing value on null/undefined. Otherwise the wire
    must contain a definite byte string of exactly twenty bytes. -/
inductive AddressValid (old : GoArray w8 20) : List w8 → GoArray w8 20 → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : AddressValid old [head] old
  | bytes {item body} (valid : BytesIntoValid 20 item body) : AddressValid old item (array.mk 20 body)

def AddressRejected (wire : List w8) (reason : BytesIntoFailureReason) : Prop :=
  wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) ∧ BytesIntoRejected 20 wire reason

theorem address_classify (old : GoArray w8 20) (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ AddressValid old item value) ∨
    (∃ reason, AddressRejected wire reason) := by
  by_cases hn : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at hn
    | cons head rest =>
      simp only [List.head?_cons, Option.some.injEq] at hn
      exact Or.inl ⟨[head], rest, old, rfl, .nullish hn⟩
  · have hn1 : wire.head? ≠ some (W8 246) := fun h => hn (Or.inl h)
    have hn2 : wire.head? ≠ some (W8 247) := fun h => hn (Or.inr h)
    rcases bytesInto_classify 20 wire with ⟨item, rest, body, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .bytes valid⟩
    · exact Or.inr ⟨reason, hn1, hn2, failure⟩

theorem addressValid_nonempty (old value : GoArray w8 20) (item : List w8)
    (valid : AddressValid old item value) : 0 < item.length := by
  cases valid with
  | nullish => simp
  | bytes valid => exact bytesIntoValid_nonempty _ _ _ valid

theorem addressValid_prefix_unique (old a b : GoArray w8 20) (left right restA restB : List w8)
    (first : AddressValid old left a) (second : AddressValid old right b)
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
      | bytes enc major count length =>
        simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
        obtain ⟨rfl, _⟩ := same
        rcases hn with rfl | rfl <;> contradiction
  | bytes valid =>
    cases second with
    | nullish hn =>
      cases valid with
      | bytes enc major count length =>
        simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
        obtain ⟨rfl, _⟩ := same
        rcases hn with rfl | rfl <;> contradiction
    | bytes valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := bytesIntoValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem addressValid_not_rejected (old value : GoArray w8 20) (item rest : List w8)
    (reason : BytesIntoFailureReason) (valid : AddressValid old item value)
    (failure : AddressRejected (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    rcases hn with rfl | rfl
    · exact failure.1 rfl
    · exact failure.2.1 rfl
  | bytes valid => exact bytesIntoValid_not_rejected _ _ _ _ _ valid failure.2.2

inductive AddressResult where
  | success (value : GoArray w8 20) (rest : List w8)
  | failure (reason : BytesIntoFailureReason)

def AddressSpec (old : GoArray w8 20) (wire : List w8) : AddressResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ AddressValid old item value
  | .failure reason => AddressRejected wire reason

theorem addressSpec_unique (old : GoArray w8 20) (wire : List w8) (a b : AddressResult)
    (first : AddressSpec old wire a) (second : AddressSpec old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := addressValid_prefix_unique old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure reason =>
      subst wire
      exact False.elim (addressValid_not_rejected _ _ _ _ _ valid second)
  | failure reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (addressValid_not_rejected _ _ _ _ _ valid first)
    | failure reason2 => exact congrArg AddressResult.failure (bytesIntoRejected_unique _ _ _ _ first.2.2 second.2.2)

theorem addressSpec_total_unique (old : GoArray w8 20) (wire : List w8) :
    ∃ outcome, AddressSpec old wire outcome ∧ ∀ other, AddressSpec old wire other → other = outcome := by
  have existsOutcome : ∃ outcome, AddressSpec old wire outcome := by
    rcases address_classify old wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => addressSpec_unique old wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.address_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.addressValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.addressValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.addressValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.addressSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.addressSpec_total_unique
