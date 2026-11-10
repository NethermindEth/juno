import ProofCheck.StringFailure
import ProofCheck.Utf8Classify
import ProofCheck.FeltClassify
import ProofCheck.HeadDeterminism
import ProofCheck.ExpectedDeterminism

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The string field policy preserves the old bytes on null/undefined and copies
    exactly the declared UTF-8 text payload otherwise. Length equality is in
    mathematical integers, so a wrapped machine length is not an encoding. -/
inductive StringValid (old : GoString) : List w8 → GoString → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) :
      StringValid old [head] old
  | text {head : w8} {payload body : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 96)
      (count : uint.Z argument = body.length) (valid : Utf8Valid body) :
      StringValid old ((head :: payload) ++ body) body

/-- The full independent rejection specification. `StringFailure` currently
    has production proofs for only a subset of these cases; this relation does
    not claim that the missing validator/string implementation proof is closed. -/
inductive StringRejected : List w8 → StringFailureReason → Prop where
  | expected {wire : List w8} {reason : ExpectedFailureReason}
      (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 96) wire reason) : StringRejected wire (.expected reason)
  | truncated {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 96)
      (short : tail.length < uint.Z argument) : StringRejected ((head :: payload) ++ tail) .truncated
  | invalidUTF8 {head : w8} {payload body rem : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 96)
      (count : uint.Z argument = body.length) (invalid : ¬ Utf8Valid body) :
      StringRejected ((head :: payload) ++ body ++ rem) .invalidUTF8

/-- Every byte list has a specified string-field outcome, with no restrictions
    on head widths, text contents or suffixes. Nullish values take precedence;
    incomplete payloads are rejected before UTF-8 is inspected. -/
theorem string_classify (old : GoString) (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ StringValid old item value) ∨
    (∃ reason, StringRejected wire reason) := by
  classical
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, old, rfl, .nullish (by simpa using nullish)⟩
  · have hn : wire.head? ≠ some (W8 246) := fun h => nullish (Or.inl h)
    have hu : wire.head? ≠ some (W8 247) := fun h => nullish (Or.inr h)
    rcases expected_classify (W8 96) wire with
      ⟨head, payload, tail, argument, split, enc, major⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases short : tail.length < uint.Z argument
      · exact Or.inr ⟨.truncated, .truncated enc major short⟩
      · let body := tail.take (uint.Z argument).toNat
        let rest := tail.drop (uint.Z argument).toNat
        have count : uint.Z argument = body.length := by
          have nonneg : 0 ≤ uint.Z argument := by word
          simp only [body, List.length_take]
          rw [Nat.min_eq_left (by omega)]
          omega
        have splitBody : body ++ rest = tail := List.take_append_drop _ _
        by_cases valid : Utf8Valid body
        · exact Or.inl ⟨(head :: payload) ++ body, rest, body,
            by rw [List.append_assoc, splitBody], .text enc major count valid⟩
        · apply Or.inr
          refine ⟨.invalidUTF8, ?_⟩
          rw [← splitBody, ← List.append_assoc]
          exact .invalidUTF8 enc major count valid
    · exact Or.inr ⟨.expected reason, .expected hn hu failure⟩

/-- Even an empty text payload consumes its CBOR head; nullish values consume
    one byte. This is the progress fact needed by enclosing record relations. -/
theorem stringValid_nonempty (old value : GoString) (wire : List w8)
    (valid : StringValid old wire value) : 0 < wire.length := by
  cases valid <;> simp

/-- A successful non-null text result is valid UTF-8. A nullish field preserves
    the old string, even when the caller's previous bytes are not valid UTF-8. -/
theorem stringValid_value (old value : GoString) (wire : List w8)
    (valid : StringValid old wire value) : value = old ∨ Utf8Valid value := by
  cases valid with
  | nullish _ => exact Or.inl rfl
  | text _ _ _ utf8 => exact Or.inr utf8

/-- A successful string-field decoding of a given input has exactly one
    consumed prefix, output value and suffix, even across alternative head widths. -/
theorem stringValid_prefix_unique (old leftValue rightValue : GoString)
    (left right restLeft restRight : List w8)
    (first : StringValid old left leftValue) (second : StringValid old right rightValue)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ leftValue = rightValue ∧ restLeft = restRight := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | @text other payload body argument enc major count valid =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @text head payload body argument enc major count valid =>
    cases second with
    | @nullish other ho =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @text other payload2 body2 argument2 enc2 major2 count2 valid2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨rfl, rfl, same⟩ :=
        headArgument_prefix_unique head payload payload2 (leftValue ++ restLeft) (rightValue ++ restRight)
          argument argument2 enc enc2 same
      have lengths : leftValue.length = rightValue.length := by omega
      have bodies := congrArg (List.take leftValue.length) same
      simp only [List.take_left] at bodies
      rw [lengths, List.take_left] at bodies
      subst rightValue
      have tails := List.append_cancel_left same
      subst restRight
      exact ⟨rfl, rfl, rfl⟩

/-- Rejection never overrides the typed reader's initial null/undefined check. -/
theorem stringRejected_not_nullish (wire : List w8) (reason : StringFailureReason)
    (failure : StringRejected wire reason) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  cases failure with
  | expected hn hu _ => exact ⟨hn, hu⟩
  | truncated enc major short =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction
  | invalidUTF8 enc major count invalid =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction

/-- The independent success and failure specifications are disjoint on the
    same input, including malformed text after otherwise valid character prefixes. -/
theorem stringValid_not_rejected (old value : GoString) (item rest : List w8)
    (reason : StringFailureReason) (valid : StringValid old item value) :
    ¬ StringRejected (item ++ rest) reason := by
  intro failure
  cases valid with
  | @nullish head hn =>
    have notNullish := stringRejected_not_nullish _ _ failure
    rcases hn with rfl | rfl
    · exact notNullish.1 rfl
    · exact notNullish.2 rfl
  | @text head payload body argument enc major count utf8 =>
    generalize hwire : (head :: payload) ++ value ++ rest = wire at failure
    cases failure with
    | expected hn hu failure =>
      exact expectedFailure_excludes_encoded (W8 96) head payload (value ++ rest) wire
        argument _ enc major (by simpa only [List.append_assoc] using hwire.symm) failure
    | @truncated other payload2 tail argument2 enc2 major2 short =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, same⟩ := hwire
      obtain ⟨rfl, rfl, rfl⟩ :=
        headArgument_prefix_unique head payload payload2 (value ++ rest) tail
          argument argument2 enc enc2 same
      simp only [List.length_append] at short
      omega
    | @invalidUTF8 other payload2 body2 rem argument2 enc2 major2 count2 invalid =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, same⟩ := hwire
      obtain ⟨rfl, rfl, same⟩ :=
        headArgument_prefix_unique head payload payload2 (value ++ rest) (body2 ++ rem)
          argument argument2 enc enc2 same
      have lengths : value.length = body2.length := by omega
      have bodies := congrArg (List.take value.length) same
      simp only [List.take_left] at bodies
      rw [lengths, List.take_left] at bodies
      subst body2
      exact invalid utf8

/-- The previous production-proved failures embed in the full specification on
    representable input lengths. The bound prevents a mathematical list length
    from wrapping in the legacy bad-leading-byte relation's W64 constructor. -/
theorem stringFailure_rejected (wire : List w8) (reason : StringFailureReason)
    (bound : wire.length < 2^64) (failure : StringFailure wire reason) :
    StringRejected wire reason := by
  cases failure with
  | expected hn hu failure => exact .expected hn hu failure
  | truncated enc major short => exact .truncated enc major short
  | @badLead head b payload tail rem enc major bad =>
    apply StringRejected.invalidUTF8 enc major
    · simp only [List.length_append, List.length_cons] at bound
      simp only [List.length_cons]
      word
    · exact utf8Valid_bad_lead b tail bad

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.string_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stringValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stringValid_value
#print axioms Perennial.github_com.NethermindEth.juno.core.stringFailure_rejected

#print axioms Perennial.github_com.NethermindEth.juno.core.stringValid_prefix_unique

#print axioms Perennial.github_com.NethermindEth.juno.core.stringRejected_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.stringValid_not_rejected
