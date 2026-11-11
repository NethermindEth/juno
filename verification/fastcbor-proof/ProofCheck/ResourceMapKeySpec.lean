import ProofCheck.DAModeSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Resource map keys accept every unsigned 32-bit number, including unknown enum
    values and wider CBOR encodings. Null and undefined are rejected keys. -/
inductive ResourceMapKeyValid : List w8 → w32 → Prop where
  | unsigned {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
      (fits : uint.Z argument ≤ 4294967295) :
      ResourceMapKeyValid (head :: payload) (W32 (uint.Z argument))

inductive ResourceMapKeyFailureReason where
  | nullish
  | numeric (reason : DAModeFailureReason)

/-- Nullish rejection consumes one byte; width overflow consumes the unsigned
    item. Unsigned parse errors restore the cursor. The first two errors are
    bare sentinels; their consumed positions are not allocated diagnostics. -/
inductive ResourceMapKeyRejected : List w8 → Nat → ResourceMapKeyFailureReason → Prop where
  | nullish {head : w8} (rest : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      ResourceMapKeyRejected (head :: rest) 1 .nullish
  | numeric {wire consumed reason} (failure : DAModeRejected wire consumed reason) :
      ResourceMapKeyRejected wire consumed (.numeric reason)

theorem resourceMapKeyValid_scalar (item : List w8) (key : w32) (valid : ResourceMapKeyValid item key) :
    DAModeValid (W32 0) item key := by
  cases valid with
  | unsigned enc major fits => exact .unsigned enc major fits

theorem resourceMapKeyValid_not_nullish (item rest : List w8) (key : w32)
    (valid : ResourceMapKeyValid item key) :
    (item ++ rest).head? ≠ some (W8 246) ∧ (item ++ rest).head? ≠ some (W8 247) := by
  cases valid with
  | unsigned enc major fits =>
    constructor <;> intro eq <;> simp only [List.cons_append, List.head?_cons, Option.some.injEq] at eq
    all_goals subst eq; contradiction

theorem resourceMapKey_classify (wire : List w8) :
    (∃ item rest key, wire = item ++ rest ∧ ResourceMapKeyValid item key) ∨
    (∃ consumed reason, ResourceMapKeyRejected wire consumed reason) := by
  rcases daMode_classify (W32 0) wire with ⟨item, rest, key, split, valid⟩ | ⟨consumed, reason, failure⟩
  · cases valid with
    | nullish hn => subst wire; exact Or.inr ⟨1, .nullish, .nullish rest hn⟩
    | unsigned enc major fits => exact Or.inl ⟨_, rest, _, split, .unsigned enc major fits⟩
  · exact Or.inr ⟨consumed, .numeric reason, .numeric failure⟩

theorem resourceMapKeyValid_nonempty (item : List w8) (key : w32)
    (valid : ResourceMapKeyValid item key) : 0 < item.length := by cases valid; simp

theorem resourceMapKeyValid_prefix_unique (left right restA restB : List w8) (a b : w32)
    (first : ResourceMapKeyValid left a) (second : ResourceMapKeyValid right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB :=
  daModeValid_prefix_unique _ _ _ _ _ _ _ (resourceMapKeyValid_scalar _ _ first)
    (resourceMapKeyValid_scalar _ _ second) same

theorem resourceMapKeyValid_not_rejected (item rest : List w8) (key : w32) (consumed : Nat)
    (reason : ResourceMapKeyFailureReason) (valid : ResourceMapKeyValid item key)
    (failure : ResourceMapKeyRejected (item ++ rest) consumed reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases failure with
  | nullish tail hn =>
    have excluded := resourceMapKeyValid_not_nullish item rest key valid
    rw [same] at excluded
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | numeric failure =>
    rw [← same] at failure
    exact daModeValid_not_rejected _ _ _ _ _ _ (resourceMapKeyValid_scalar _ _ valid) failure

theorem resourceMapKeyRejected_unique (wire : List w8) (consumedA consumedB : Nat)
    (a b : ResourceMapKeyFailureReason)
    (first : ResourceMapKeyRejected wire consumedA a) (second : ResourceMapKeyRejected wire consumedB b) :
    consumedA = consumedB ∧ a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | nullish tail hn =>
    cases second with
    | nullish tail2 hn2 => exact ⟨rfl, rfl⟩
    | numeric failure =>
      rw [← same] at failure
      exact False.elim (daModeValid_not_rejected (W32 0) (W32 0) _ _ _ _ (.nullish hn) failure)
  | numeric failure =>
    cases second with
    | nullish tail2 hn2 =>
      rw [same] at failure
      exact False.elim (daModeValid_not_rejected (W32 0) (W32 0) _ _ _ _ (.nullish hn2) failure)
    | numeric failure2 =>
      rw [← same] at failure2
      obtain ⟨rfl, rfl⟩ := daModeRejected_unique _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩

theorem resourceMapKeyRejected_bound (wire : List w8) (consumed : Nat) (reason : ResourceMapKeyFailureReason)
    (failure : ResourceMapKeyRejected wire consumed reason) : consumed ≤ wire.length := by
  cases failure with
  | nullish => simp
  | numeric failure => exact daModeRejected_bound _ _ _ failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapKeyValid_scalar
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapKeyValid_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapKey_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapKeyValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapKeyValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapKeyValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapKeyRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapKeyRejected_bound
