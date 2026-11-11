import ProofCheck.BigIntSpec

set_option maxHeartbeats 6400000
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Signed int64 wire values, independent of the Go reader and FX. All supported
    head widths are allowed; the CBOR argument must fit 63 unsigned bits for
    either sign. Thus the negative endpoint is -2^63, not -MaxInt64. -/
inductive Int64Valid : List w8 → Int → Prop where
  | unsigned {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
      (bound : uint.Z argument < 2 ^ 63) :
      Int64Valid (head :: payload) (uint.Z argument)
  | negative {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 32)
      (bound : uint.Z argument < 2 ^ 63) :
      Int64Valid (head :: payload) (-1 - uint.Z argument)

inductive Int64FailureReason where
  | head (bad : w8) (reason : HeadError)
  | incomplete
  | eof
  | typeMismatch (bad : w8)
  | tag
  | overflow

/-- A head fault or incomplete argument is reported before checking the major
    type. Complete tags are unsupported; other non-integers are type errors.
    All failures leave the cursor at the original position. -/
inductive Int64Rejected : List w8 → Int64FailureReason → Prop where
  | head {head : w8} {reason : HeadError} (tail : List w8) (fault : HeadFault head reason) :
      Int64Rejected (head :: tail) (.head head reason)
  | incomplete {head : w8} (tail : List w8) {size : w64}
      (info : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
      (sizeEq : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
      (short : tail.length < sint.Z size) : Int64Rejected (head :: tail) .incomplete
  | eof : Int64Rejected [] .eof
  | typeMismatch {head : w8} {payload : List w8} {argument : w64} (tail : List w8)
      (enc : HeadArgument head payload argument)
      (notUnsigned : head &&& W8 224 ≠ W8 0) (notNegative : head &&& W8 224 ≠ W8 32)
      (notTag : head &&& W8 224 ≠ W8 192) :
      Int64Rejected ((head :: payload) ++ tail) (.typeMismatch head)
  | tag {head : w8} {payload : List w8} {argument : w64} (tail : List w8)
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192) :
      Int64Rejected ((head :: payload) ++ tail) .tag
  | overflow {head : w8} {payload : List w8} {argument : w64} (tail : List w8)
      (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
      (bound : 2 ^ 63 ≤ uint.Z argument) :
      Int64Rejected ((head :: payload) ++ tail) .overflow

theorem int64_classify (wire : List w8) :
    (∃ item rest number, wire = item ++ rest ∧ Int64Valid item number) ∨
    (∃ reason, Int64Rejected wire reason) := by
  cases wire with
  | nil => exact Or.inr ⟨.eof, .eof⟩
  | cons head tail =>
    cases head_classify head tail with
    | fault reason fault => exact Or.inr ⟨.head head reason, .head tail fault⟩
    | truncated size info sizeEq short => exact Or.inr ⟨.incomplete, .incomplete tail info sizeEq short⟩
    | encoded payload rest argument split enc =>
      subst tail
      by_cases major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32
      · by_cases bound : uint.Z argument < 2 ^ 63
        · rcases major with unsigned | negative
          · exact Or.inl ⟨head :: payload, rest, _, rfl, .unsigned enc unsigned bound⟩
          · exact Or.inl ⟨head :: payload, rest, _, rfl, .negative enc negative bound⟩
        · exact Or.inr ⟨.overflow, .overflow rest enc major (by omega)⟩
      · by_cases tag : head &&& W8 224 = W8 192
        · exact Or.inr ⟨.tag, .tag rest enc tag⟩
        · exact Or.inr ⟨.typeMismatch head, .typeMismatch rest enc
            (fun h => major (Or.inl h)) (fun h => major (Or.inr h)) tag⟩

theorem int64Valid_bigInt (wire : List w8) (number : Int) (valid : Int64Valid wire number) :
    BigIntValid wire number := by
  cases valid with
  | unsigned enc major _ => exact .unsigned enc major
  | negative enc major _ => exact .negative enc major

theorem int64Valid_bounds (wire : List w8) (number : Int) (valid : Int64Valid wire number) :
    -(2 ^ 63) ≤ number ∧ number < 2 ^ 63 := by
  cases valid with
  | @unsigned head payload argument enc major bound =>
    have h : 0 ≤ uint.Z argument := by word
    omega
  | @negative head payload argument enc major bound =>
    have h : 0 ≤ uint.Z argument := by word
    omega

theorem int64Valid_prefix_unique (left right restLeft restRight : List w8)
    (leftValue rightValue : Int) (first : Int64Valid left leftValue)
    (second : Int64Valid right rightValue) (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ leftValue = rightValue ∧ restLeft = restRight := by
  exact bigIntValid_prefix_unique left right restLeft restRight leftValue rightValue
    (int64Valid_bigInt left leftValue first) (int64Valid_bigInt right rightValue second) same

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.int64_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.int64Valid_bigInt
#print axioms Perennial.github_com.NethermindEth.juno.core.int64Valid_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.int64Valid_prefix_unique
