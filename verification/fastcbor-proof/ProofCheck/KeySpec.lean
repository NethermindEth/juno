import ProofCheck.KeyFailureSpec
import ProofCheck.StringSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A successful key either supplies the exact valid UTF-8 text bytes or is an
    int64-range integer, reported as non-text. Every definite head width is allowed.
    Null/undefined are not valid keys; they have no field-preservation meaning here. -/
inductive KeyValid : List w8 → Option GoString → Prop where
  | text {head : w8} {payload body : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 96)
      (count : uint.Z argument = body.length) (valid : Utf8Valid body) :
      KeyValid ((head :: payload) ++ body) (some body)
  | integer {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
      (bound : uint.Z argument < 2^63) : KeyValid (head :: payload) none

/-- Full key rejection specification. Production proofs still cover only the
    narrower KeyFailure relation until general UTF-8 validation is discharged. -/
inductive KeyRejected : List w8 → KeyFailureReason → Prop where
  | head {bad : w8} (tail : List w8) {reason : HeadError} (fault : HeadFault bad reason) :
      KeyRejected (bad :: tail) (.head bad reason)
  | incomplete {head : w8} (tail : List w8) {size : w64}
      (info : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
      (sizeEq : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
      (short : tail.length < sint.Z size) : KeyRejected (head :: tail) .incomplete
  | eof : KeyRejected [] .eof
  | integerRange {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
      (bound : 2^63 ≤ uint.Z argument) : KeyRejected ((head :: payload) ++ tail) .integerRange
  | tag {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192) :
      KeyRejected ((head :: payload) ++ tail) .tag
  | typeMismatch {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (text : head &&& W8 224 ≠ W8 96)
      (uint : head &&& W8 224 ≠ W8 0) (negative : head &&& W8 224 ≠ W8 32)
      (tag : head &&& W8 224 ≠ W8 192) : KeyRejected ((head :: payload) ++ tail) (.typeMismatch head)
  | textTruncated {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 96)
      (short : tail.length < uint.Z argument) : KeyRejected ((head :: payload) ++ tail) .textTruncated
  | invalidUTF8 {head : w8} {payload body rem : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 96)
      (count : uint.Z argument = body.length) (invalid : ¬ Utf8Valid body) :
      KeyRejected ((head :: payload) ++ body ++ rem) .invalidUTF8

/-- Every input has a specified key outcome, including wider text heads,
    arbitrary Unicode payloads, integer overflow and every malformed input. -/
theorem key_classify (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ KeyValid item value) ∨
    (∃ reason, KeyRejected wire reason) := by
  classical
  cases wire with
  | nil => exact Or.inr ⟨.eof, .eof⟩
  | cons head tail =>
    cases head_classify head tail with
    | fault reason fault => exact Or.inr ⟨.head head reason, .head tail fault⟩
    | truncated size info sizeEq short => exact Or.inr ⟨.incomplete, .incomplete tail info sizeEq short⟩
    | encoded payload rest argument split enc =>
      subst tail
      by_cases text : head &&& W8 224 = W8 96
      · by_cases short : rest.length < uint.Z argument
        · exact Or.inr ⟨.textTruncated, .textTruncated enc text short⟩
        · let body := rest.take (uint.Z argument).toNat
          let rem := rest.drop (uint.Z argument).toNat
          have count : uint.Z argument = body.length := by
            have nonneg : 0 ≤ uint.Z argument := by word
            simp only [body, List.length_take]
            rw [Nat.min_eq_left (by omega)]
            omega
          have splitBody : body ++ rem = rest := List.take_append_drop _ _
          by_cases valid : Utf8Valid body
          · exact Or.inl ⟨(head :: payload) ++ body, rem, some body,
              by rw [List.append_assoc, splitBody]; rfl, .text enc text count valid⟩
          · refine Or.inr ⟨.invalidUTF8, ?_⟩
            rw [← splitBody]
            simpa only [List.append_assoc, List.cons_append] using
              (KeyRejected.invalidUTF8 (rem := rem) enc text count valid)
      · by_cases integer : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32
        · by_cases bound : uint.Z argument < 2^63
          · exact Or.inl ⟨head :: payload, rest, none, rfl, .integer enc integer bound⟩
          · exact Or.inr ⟨.integerRange, .integerRange enc integer (by omega)⟩
        · by_cases tag : head &&& W8 224 = W8 192
          · exact Or.inr ⟨.tag, .tag enc tag⟩
          · exact Or.inr ⟨.typeMismatch head, .typeMismatch enc text
              (fun h => integer (Or.inl h)) (fun h => integer (Or.inr h)) tag⟩

theorem keyValid_nonempty (wire : List w8) (value : Option GoString)
    (valid : KeyValid wire value) : 0 < wire.length := by
  cases valid <;> simp

/-- Both paths have exact consumption, even when text heads use non-preferred
    widths. This is independent of lookup caches and schema field matching. -/
theorem keyValid_prefix_unique (left right restLeft restRight : List w8)
    (leftValue rightValue : Option GoString)
    (first : KeyValid left leftValue) (second : KeyValid right rightValue)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ leftValue = rightValue ∧ restLeft = restRight := by
  cases first with
  | @text head payload body argument enc major count valid =>
    cases second with
    | @text other payload2 body2 argument2 enc2 major2 count2 valid2 =>
      obtain ⟨hi, hv, hr⟩ := stringValid_prefix_unique [] body body2 _ _ restLeft restRight
        (.text enc major count valid) (.text enc2 major2 count2 valid2) same
      exact ⟨hi, congrArg some hv, hr⟩
    | @integer other payload2 argument2 enc2 major2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases major2 with hm | hm <;> word
  | @integer head payload argument enc major bound =>
    cases second with
    | @text other payload2 body2 argument2 enc2 major2 count2 valid2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases major with hm | hm <;> word
    | @integer other payload2 argument2 enc2 major2 bound2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨rfl, _, rfl⟩ :=
        headArgument_prefix_unique head payload payload2 restLeft restRight argument argument2 enc enc2 same
      exact ⟨rfl, rfl, rfl⟩

/-- A successful key prefix cannot also satisfy any specified key rejection.
    This includes the full UTF-8 failure family, not only illegal first bytes. -/
theorem keyValid_not_rejected (item rest : List w8) (value : Option GoString)
    (reason : KeyFailureReason) (valid : KeyValid item value) :
    ¬ KeyRejected (item ++ rest) reason := by
  intro failure
  cases valid with
  | @text head payload body argument enc major count utf8 =>
    generalize hwire : (head :: payload) ++ body ++ rest = wire at failure
    cases failure with
    | head tail fault =>
      exact expectedFailure_excludes_encoded (W8 96) head payload (body ++ rest) _
        argument _ enc major (by simpa only [List.append_assoc] using hwire.symm) (.head tail fault)
    | incomplete tail info sizeEq short =>
      exact expectedFailure_excludes_encoded (W8 96) head payload (body ++ rest) _
        argument _ enc major (by simpa only [List.append_assoc] using hwire.symm)
        (.incomplete tail info sizeEq short)
    | eof => simp at hwire
    | integerRange enc2 major2 overflow =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, _⟩ := hwire
      rcases major2 with hm | hm <;> word
    | tag enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, _⟩ := hwire
      word
    | typeMismatch enc2 text uint negative tag =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, _⟩ := hwire
      exact text major
    | textTruncated enc2 major2 short =>
      apply stringValid_not_rejected [] body _ rest .truncated (.text enc major count utf8)
      rw [hwire]
      exact .truncated enc2 major2 short
    | invalidUTF8 enc2 major2 count2 invalid =>
      apply stringValid_not_rejected [] body _ rest .invalidUTF8 (.text enc major count utf8)
      rw [hwire]
      exact .invalidUTF8 enc2 major2 count2 invalid
  | @integer head payload argument enc major bound =>
    generalize hwire : (head :: payload) ++ rest = wire at failure
    cases failure with
    | head tail fault =>
      exact expectedFailure_excludes_encoded (head &&& W8 224) head payload rest _
        argument _ enc rfl hwire.symm (.head tail fault)
    | incomplete tail info sizeEq short =>
      exact expectedFailure_excludes_encoded (head &&& W8 224) head payload rest _
        argument _ enc rfl hwire.symm (.incomplete tail info sizeEq short)
    | eof => simp at hwire
    | @integerRange other payload2 tail argument2 enc2 major2 overflow =>
      simp only [List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, same⟩ := hwire
      obtain ⟨_, rfl, _⟩ :=
        headArgument_prefix_unique head payload payload2 rest tail argument argument2 enc enc2 same
      omega
    | tag enc2 major2 =>
      simp only [List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, _⟩ := hwire
      rcases major with hm | hm <;> word
    | typeMismatch enc2 text uint negative tag =>
      simp only [List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, _⟩ := hwire
      exact major.elim uint negative
    | textTruncated enc2 major2 short =>
      simp only [List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, _⟩ := hwire
      rcases major with hm | hm <;> word
    | invalidUTF8 enc2 major2 count2 invalid =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at hwire
      obtain ⟨rfl, _⟩ := hwire
      rcases major with hm | hm <;> word

/-- Previously proved key errors are included in the full contract; the length
    bound rules out wraparound in the older bad-leading-byte relation. -/
theorem keyFailure_rejected (wire : List w8) (reason : KeyFailureReason)
    (bound : wire.length < 2^64) (failure : KeyFailure wire reason) : KeyRejected wire reason := by
  cases failure with
  | head tail fault => exact .head tail fault
  | incomplete tail info sizeEq short => exact .incomplete tail info sizeEq short
  | eof => exact .eof
  | integerRange enc major overflow => exact .integerRange enc major overflow
  | tag enc major => exact .tag enc major
  | typeMismatch enc text uint negative tag => exact .typeMismatch enc text uint negative tag
  | textTruncated enc major short => exact .textTruncated enc major short
  | @badLead head b payload tail rem enc major bad =>
    apply KeyRejected.invalidUTF8 enc major
    · simp only [List.length_append, List.length_cons] at bound
      simp only [List.length_cons]
      word
    · exact utf8Valid_bad_lead b tail bad

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.key_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.keyValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.keyValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.keyFailure_rejected

#print axioms Perennial.github_com.NethermindEth.juno.core.keyValid_not_rejected
