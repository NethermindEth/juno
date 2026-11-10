import ProofCheck.ASCII

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

/-- Continuation bytes in the independent RFC 3629 section 4 byte grammar. -/
def Utf8Tail (b : w8) : Prop := 128 ≤ uint.Z b ∧ uint.Z b ≤ 191

/-- UTF-8 syntax, independent of Go's classification tables and scan loops.
    The special E0/ED/F0/F4 cases exclude overlong encodings, surrogate scalars
    and values above U+10FFFF. This imposes no Unicode normalization or BOM policy.
    Source: https://www.rfc-editor.org/rfc/rfc3629.html#section-4 -/
inductive Utf8Valid : List w8 → Prop where
  | empty : Utf8Valid []
  | ascii {b : w8} {rest : List w8}
      (byte : uint.Z b < 128) (tail : Utf8Valid rest) : Utf8Valid (b :: rest)
  | two {b0 b1 : w8} {rest : List w8}
      (lead : 194 ≤ uint.Z b0 ∧ uint.Z b0 ≤ 223) (second : Utf8Tail b1)
      (tail : Utf8Valid rest) : Utf8Valid (b0 :: b1 :: rest)
  | threeLow {b1 b2 : w8} {rest : List w8}
      (second : 160 ≤ uint.Z b1 ∧ uint.Z b1 ≤ 191) (third : Utf8Tail b2)
      (tail : Utf8Valid rest) : Utf8Valid (W8 224 :: b1 :: b2 :: rest)
  | three {b0 b1 b2 : w8} {rest : List w8}
      (lead : (225 ≤ uint.Z b0 ∧ uint.Z b0 ≤ 236) ∨ (238 ≤ uint.Z b0 ∧ uint.Z b0 ≤ 239))
      (second : Utf8Tail b1) (third : Utf8Tail b2)
      (tail : Utf8Valid rest) : Utf8Valid (b0 :: b1 :: b2 :: rest)
  | threeSurrogate {b1 b2 : w8} {rest : List w8}
      (second : 128 ≤ uint.Z b1 ∧ uint.Z b1 ≤ 159) (third : Utf8Tail b2)
      (tail : Utf8Valid rest) : Utf8Valid (W8 237 :: b1 :: b2 :: rest)
  | fourLow {b1 b2 b3 : w8} {rest : List w8}
      (second : 144 ≤ uint.Z b1 ∧ uint.Z b1 ≤ 191) (third : Utf8Tail b2) (fourth : Utf8Tail b3)
      (tail : Utf8Valid rest) : Utf8Valid (W8 240 :: b1 :: b2 :: b3 :: rest)
  | four {b0 b1 b2 b3 : w8} {rest : List w8}
      (lead : 241 ≤ uint.Z b0 ∧ uint.Z b0 ≤ 243)
      (second : Utf8Tail b1) (third : Utf8Tail b2) (fourth : Utf8Tail b3)
      (tail : Utf8Valid rest) : Utf8Valid (b0 :: b1 :: b2 :: b3 :: rest)
  | fourHigh {b1 b2 b3 : w8} {rest : List w8}
      (second : 128 ≤ uint.Z b1 ∧ uint.Z b1 ≤ 143) (third : Utf8Tail b2) (fourth : Utf8Tail b3)
      (tail : Utf8Valid rest) : Utf8Valid (W8 244 :: b1 :: b2 :: b3 :: rest)

/-- These bytes can never start a UTF-8 character: continuation bytes, C0/C1,
    and F5 through FF. Rejection is independent of every following byte. -/
def Utf8BadLead (b : w8) : Prop :=
  (128 ≤ uint.Z b ∧ uint.Z b < 194) ∨ 245 ≤ uint.Z b

theorem utf8Valid_append {xs ys : List w8} (hx : Utf8Valid xs) (hy : Utf8Valid ys) :
    Utf8Valid (xs ++ ys) := by
  induction hx with
  | empty => exact hy
  | ascii h _ ih => exact .ascii h ih
  | two h0 h1 _ ih => exact .two h0 h1 ih
  | threeLow h1 h2 _ ih => exact .threeLow h1 h2 ih
  | three h0 h1 h2 _ ih => exact .three h0 h1 h2 ih
  | threeSurrogate h1 h2 _ ih => exact .threeSurrogate h1 h2 ih
  | fourLow h1 h2 h3 _ ih => exact .fourLow h1 h2 h3 ih
  | four h0 h1 h2 h3 _ ih => exact .four h0 h1 h2 h3 ih
  | fourHigh h1 h2 h3 _ ih => exact .fourHigh h1 h2 h3 ih

theorem utf8Valid_bad_lead (b : w8) (rest : List w8) (bad : Utf8BadLead b) :
    ¬ Utf8Valid (b :: rest) := by
  intro valid
  cases valid <;> simp_all [Utf8BadLead, uint.Z] <;> omega

theorem utf8Valid_ascii_cons (b : w8) (rest : List w8) (h : uint.Z b < 128) :
    Utf8Valid (b :: rest) ↔ Utf8Valid rest := by
  constructor
  · intro valid
    cases valid <;> simp_all [uint.Z] <;> omega
  · exact Utf8Valid.ascii h

/-- Arbitrary ASCII prefixes can be consumed in any chunking without changing
    whether the remaining text is valid UTF-8. This supports the word fast path. -/
theorem utf8Valid_ascii_prefix (pre rest : List w8) (h : ASCIIBytes pre) :
    Utf8Valid (pre ++ rest) ↔ Utf8Valid rest := by
  induction pre with
  | nil => rfl
  | cons b pre ih =>
    have hb : uint.Z b < 128 := h b (by simp)
    have ht : ASCIIBytes pre := fun x hx => h x (by simp [hx])
    exact (utf8Valid_ascii_cons b (pre ++ rest) hb).trans (ih ht)

theorem utf8Valid_ascii (xs : List w8) (h : ASCIIBytes xs) : Utf8Valid xs := by
  have hi := (utf8Valid_ascii_prefix xs [] h).2 Utf8Valid.empty
  simpa using hi

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Valid_append
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Valid_bad_lead
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Valid_ascii_cons
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Valid_ascii_prefix
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Valid_ascii
