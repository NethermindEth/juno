import ProofCheck.ExpectedDeterminism
import ProofCheck.ScalarComplete

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Deterministic item boundaries and values lift to any fixed-length sequence.
    The witness type may contain proofs; only bytes and application values are
    compared, so this does not require equality of encoding witnesses. -/
theorem wireSequence_prefix_unique {Item Value : Type}
    (bytes : Item → List w8) (value : Item → Value)
    (left right : List Item) (restLeft restRight : List w8)
    (length : left.length = right.length)
    (unique : ∀ a ∈ left, ∀ b ∈ right, ∀ tailA tailB,
      bytes a ++ tailA = bytes b ++ tailB →
      bytes a = bytes b ∧ value a = value b ∧ tailA = tailB)
    (same : left.flatMap bytes ++ restLeft = right.flatMap bytes ++ restRight) :
    left.flatMap bytes = right.flatMap bytes ∧ left.map value = right.map value ∧ restLeft = restRight := by
  induction left generalizing right with
  | nil =>
    have empty : right = [] := by simpa using length.symm
    subst right
    simpa using same
  | cons a left ih =>
    cases right with
    | nil => simp at length
    | cons b right =>
      have tailLength : left.length = right.length := by simpa using length
      have split : bytes a ++ (left.flatMap bytes ++ restLeft) =
          bytes b ++ (right.flatMap bytes ++ restRight) := by
        simpa only [List.flatMap_cons, List.append_assoc] using same
      obtain ⟨wire, val, tails⟩ := unique a (by simp) b (by simp) _ _ split
      obtain ⟨wires, vals, rest⟩ := ih right tailLength
        (fun x hx y hy => unique x (by simp [hx]) y (by simp [hy])) tails
      simp only [List.flatMap_cons, List.map_cons, wire, val, wires, vals, rest, and_self]

/-- Scalar values and consumption are unique, including null/undefined's
    preservation of the old field and every unsigned head width. -/
theorem scalarValid_prefix_unique (old a b : w64) (left right restLeft restRight : List w8)
    (first : ScalarValid old left a) (second : ScalarValid old right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | @unsigned other payload argument enc major =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @unsigned head payload argument enc major =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @unsigned other payload2 argument2 enc2 major2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨rfl, rfl, rfl⟩ := headArgument_prefix_unique head payload payload2 restLeft restRight a b enc enc2 same
      exact ⟨rfl, rfl, rfl⟩

/-- A successfully decoded scalar prefix cannot also have a scalar error,
    irrespective of bytes following that prefix. -/
theorem scalarValid_not_failure (old value : w64) (item rest : List w8)
    (reason : ExpectedFailureReason) (valid : ScalarValid old item value)
    (failure : ScalarFailure (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    rcases hn with rfl | rfl
    · exact failure.1 rfl
    · exact failure.2.1 rfl
  | unsigned enc major =>
    exact expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major rfl failure.2.2

/-- A strict prefix of deterministically delimited items leaves the next
    complete item of the enclosing sequence at the remaining input boundary. -/
theorem wireSequence_after_prefix {Item : Type} (bytes : Item → List w8)
    (done items : List Item) (tail rest : List w8) (more : done.length < items.length)
    (unique : ∀ a ∈ done, ∀ b ∈ items, ∀ restA restB,
      bytes a ++ restA = bytes b ++ restB → restA = restB)
    (same : done.flatMap bytes ++ tail = items.flatMap bytes ++ rest) :
    ∃ item suffix, item ∈ items ∧ tail = bytes item ++ suffix := by
  induction done generalizing items with
  | nil =>
    cases items with
    | nil => simp at more
    | cons item items =>
      exact ⟨item, items.flatMap bytes ++ rest, by simp,
        by simpa only [List.flatMap_nil, List.nil_append, List.flatMap_cons, List.append_assoc] using same⟩
  | cons item done ih =>
    cases items with
    | nil => simp at more
    | cons other items =>
      have tails := unique item (by simp) other (by simp) _ _
        (by simpa only [List.flatMap_cons, List.append_assoc] using same)
      obtain ⟨next, suffix, member, split⟩ := ih items (by simpa using more)
        (fun a ha b hb => unique a (by simp [ha]) b (by simp [hb])) tails
      exact ⟨next, suffix, by simp [member], split⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wireSequence_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.scalarValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.scalarValid_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wireSequence_after_prefix
