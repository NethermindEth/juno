import ProofCheck.BigIntMagnitude
import ProofCheck.FeltClassify
import ProofCheck.HeadDeterminism

set_option maxHeartbeats 6400000
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Supported integer encodings, with mathematical values independent of math/big
    and fxamacker. All definite head widths are accepted, including wider-than-needed
    heads and bignum bodies with leading zero bytes. -/
inductive BigIntValid : List w8 → Int → Prop where
  | unsigned {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0) :
      BigIntValid (head :: payload) (uint.Z argument)
  | negative {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 32) :
      BigIntValid (head :: payload) (-1 - uint.Z argument)
  | tagged {tagHead bodyHead : w8} {tagBytes lengthBytes body : List w8} {tag count : w64}
      (tagEnc : HeadArgument tagHead tagBytes tag) (tagMajor : tagHead &&& W8 224 = W8 192)
      (allowed : tag = W64 2 ∨ tag = W64 3)
      (bodyEnc : HeadArgument bodyHead lengthBytes count) (bodyMajor : bodyHead &&& W8 224 = W8 64)
      (length : uint.Z count = body.length) :
      BigIntValid ((tagHead :: tagBytes) ++ (bodyHead :: lengthBytes) ++ body)
        (if tag = W64 3 then -1 - bigEndianMagnitude body else bigEndianMagnitude body)

/-- The outer cursor always rolls back on failure. Inner payload diagnostics
    retain their own offset, except a payload type mismatch is reclassified as
    syntax at the outer tag's start. -/
inductive BigIntFailureReason where
  | head (bad : w8) (reason : HeadError)
  | incomplete
  | eof
  | typeMismatch (bad : w8)
  | tag (argument : w64)
  | payloadExpected (tagWidth : Nat) (reason : ExpectedFailureReason)
  | payloadTruncated (tagWidth : Nat)

inductive BigIntRejected : List w8 → BigIntFailureReason → Prop where
  | head {head : w8} {reason : HeadError} (tail : List w8) (fault : HeadFault head reason) :
      BigIntRejected (head :: tail) (.head head reason)
  | incomplete {head : w8} (tail : List w8) {size : w64}
      (info : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
      (sizeEq : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
      (short : tail.length < sint.Z size) : BigIntRejected (head :: tail) .incomplete
  | eof : BigIntRejected [] .eof
  | typeMismatch {head : w8} {payload : List w8} {argument : w64} (tail : List w8)
      (enc : HeadArgument head payload argument)
      (notUnsigned : head &&& W8 224 ≠ W8 0) (notNegative : head &&& W8 224 ≠ W8 32)
      (notTag : head &&& W8 224 ≠ W8 192) :
      BigIntRejected ((head :: payload) ++ tail) (.typeMismatch head)
  | tag {head : w8} {payload : List w8} {argument : w64} (tail : List w8)
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192)
      (notPositive : argument ≠ W64 2) (notNegative : argument ≠ W64 3) :
      BigIntRejected ((head :: payload) ++ tail) (.tag argument)
  | payloadExpected {tagHead : w8} {tagBytes tail : List w8} {tag : w64} {reason : ExpectedFailureReason}
      (tagEnc : HeadArgument tagHead tagBytes tag) (tagMajor : tagHead &&& W8 224 = W8 192)
      (allowed : tag = W64 2 ∨ tag = W64 3) (failure : ExpectedFailure (W8 64) tail reason) :
      BigIntRejected ((tagHead :: tagBytes) ++ tail) (.payloadExpected (1 + tagBytes.length) reason)
  | payloadTruncated {tagHead bodyHead : w8} {tagBytes lengthBytes tail : List w8} {tag count : w64}
      (tagEnc : HeadArgument tagHead tagBytes tag) (tagMajor : tagHead &&& W8 224 = W8 192)
      (allowed : tag = W64 2 ∨ tag = W64 3)
      (bodyEnc : HeadArgument bodyHead lengthBytes count) (bodyMajor : bodyHead &&& W8 224 = W8 64)
      (short : tail.length < uint.Z count) :
      BigIntRejected ((tagHead :: tagBytes) ++ (bodyHead :: lengthBytes) ++ tail)
        (.payloadTruncated (1 + tagBytes.length))

/-- Every byte input has a supported consumed prefix or a specified rejection.
    This is a specification result; it does not assume implementation correctness. -/
theorem bigInt_classify (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ BigIntValid item value) ∨
    (∃ reason, BigIntRejected wire reason) := by
  classical
  cases wire with
  | nil => exact Or.inr ⟨.eof, .eof⟩
  | cons head tail =>
    cases head_classify head tail with
    | fault reason fault => exact Or.inr ⟨.head head reason, .head tail fault⟩
    | truncated size info sizeEq short => exact Or.inr ⟨.incomplete, .incomplete tail info sizeEq short⟩
    | encoded payload rest argument split enc =>
      subst tail
      by_cases unsigned : head &&& W8 224 = W8 0
      · exact Or.inl ⟨head :: payload, rest, _, rfl, .unsigned enc unsigned⟩
      by_cases negative : head &&& W8 224 = W8 32
      · exact Or.inl ⟨head :: payload, rest, _, rfl, .negative enc negative⟩
      by_cases tag : head &&& W8 224 = W8 192
      · by_cases allowed : argument = W64 2 ∨ argument = W64 3
        · rcases expected_classify (W8 64) rest with
            ⟨bodyHead, lengthBytes, bodyTail, count, split, bodyEnc, bodyMajor⟩ | ⟨reason, failure⟩
          · subst rest
            by_cases short : bodyTail.length < uint.Z count
            · exact Or.inr ⟨.payloadTruncated (1 + payload.length),
                by simpa only [List.append_assoc, List.cons_append] using
                  BigIntRejected.payloadTruncated enc tag allowed bodyEnc bodyMajor short⟩
            · let body := bodyTail.take (uint.Z count).toNat
              let rest := bodyTail.drop (uint.Z count).toNat
              have hcount : uint.Z count = body.length := by
                have nonneg : 0 ≤ uint.Z count := by word
                simp only [body, List.length_take]
                rw [Nat.min_eq_left (by omega)]
                omega
              have hsplit : body ++ rest = bodyTail := List.take_append_drop _ _
              refine Or.inl ⟨(head :: payload) ++ (bodyHead :: lengthBytes) ++ body, rest, _, ?_,
                .tagged enc tag allowed bodyEnc bodyMajor hcount⟩
              simp only [List.append_assoc, List.cons_append, hsplit]
          · exact Or.inr ⟨.payloadExpected (1 + payload.length) reason,
              .payloadExpected enc tag allowed failure⟩
        · exact Or.inr ⟨.tag argument, .tag rest enc tag
            (fun h => allowed (Or.inl h)) (fun h => allowed (Or.inr h))⟩
      · exact Or.inr ⟨.typeMismatch head, .typeMismatch rest enc unsigned negative tag⟩

theorem bigIntValid_nonempty (wire : List w8) (value : Int) (valid : BigIntValid wire value) :
    0 < wire.length := by
  cases valid <;> simp

/-- Supported decodings of one byte input agree on the mathematical value,
    consumed prefix and exact suffix, including tagged non-preferred encodings. -/
theorem bigIntValid_prefix_unique (left right restLeft restRight : List w8)
    (leftValue rightValue : Int) (first : BigIntValid left leftValue)
    (second : BigIntValid right rightValue) (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ leftValue = rightValue ∧ restLeft = restRight := by
  cases first with
  | @unsigned head payload argument enc major =>
    cases second with
    | @unsigned other payload2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨rfl, rfl, rfl⟩ := headArgument_prefix_unique head payload payload2 restLeft restRight
        argument argument2 enc enc2 same
      exact ⟨rfl, rfl, rfl⟩
    | @negative other payload2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
    | @tagged other bodyHead2 payload2 lengthBytes2 body2 argument2 count2 enc2 major2 allowed2 bodyEnc2 bodyMajor2 countEq2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
  | @negative head payload argument enc major =>
    cases second with
    | @unsigned other payload2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
    | @negative other payload2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨rfl, rfl, rfl⟩ := headArgument_prefix_unique head payload payload2 restLeft restRight
        argument argument2 enc enc2 same
      exact ⟨rfl, rfl, rfl⟩
    | @tagged other bodyHead2 payload2 lengthBytes2 body2 argument2 count2 enc2 major2 allowed2 bodyEnc2 bodyMajor2 countEq2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
  | @tagged head bodyHead payload lengthBytes body argument count enc major allowed bodyEnc bodyMajor countEq =>
    cases second with
    | @unsigned other payload2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
    | @negative other payload2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
    | @tagged other bodyHead2 payload2 lengthBytes2 body2 argument2 count2 enc2 major2 allowed2 bodyEnc2 bodyMajor2 countEq2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨rfl, rfl, afterTag⟩ := headArgument_prefix_unique head payload payload2
        ((bodyHead :: lengthBytes) ++ body ++ restLeft) ((bodyHead2 :: lengthBytes2) ++ body2 ++ restRight)
        argument argument2 enc enc2 (by simpa only [List.cons_append, List.append_assoc] using same)
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at afterTag
      obtain ⟨rfl, afterBodyHead⟩ := afterTag
      obtain ⟨rfl, rfl, afterLength⟩ := headArgument_prefix_unique bodyHead lengthBytes lengthBytes2
        (body ++ restLeft) (body2 ++ restRight) count count2 bodyEnc bodyEnc2 afterBodyHead
      have hlength : body.length = body2.length := by omega
      have hbodies := congrArg (List.take body.length) afterLength
      simp only [List.take_left] at hbodies
      rw [hlength, List.take_left] at hbodies
      subst body2
      have ht := List.append_cancel_left afterLength
      subst restRight
      exact ⟨rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.bigInt_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntValid_nonempty

#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntValid_prefix_unique
