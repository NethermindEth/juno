import ProofCheck.FeltRecursiveReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The remaining input does not begin with a complete nine-byte unsigned value.
    This includes EOF and a truncated wide-uint head, not just a different major. -/
def SkipNonWideUint (wire : List w8) : Prop := wire[0]? = some (W8 27) → wire.length < 9

/-- The actual shortcut is unavailable either by depth, length or its byte guards. -/
def SkipWideRejected (d : Nat) (wire : List w8) : Prop :=
  d < 32 → 37 ≤ wire.length → ¬ SkipWideShape wire

/-- Supported enclosing containers/tags and valid siblings before a failing
    remaining input. The relation records encoded heads and boundary precedence,
    independently of parser control flow and the eventual error class. -/
inductive SkipFailureContext (tail : List w8) : Nat → List w8 → Nat → Prop where
  | nil (d : Nat) : SkipFailureContext tail d [] d
  | container {d leaf : Nat} {head : w8} {payload front : List w8} {count perCount : w64}
      (items : List (List w8)) (enc : HeadArgument head payload count)
      (major : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨
        (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
      (bound : d < 32) (more : (items.length : Int) < uint.Z count * uint.Z perCount)
      (fit : uint.Z count * uint.Z perCount ≤ (items.flatten ++ (front ++ tail)).length)
      (valid : ∀ item ∈ items, SkipValid (d + 1) item)
      (child : SkipFailureContext tail (d + 1) front leaf) :
      SkipFailureContext tail d ((head :: payload) ++ (items.flatten ++ front)) leaf
  | tag {d leaf : Nat} {head next : w8} {payload front rest : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192)
      (number : 3 < uint.Z argument) (nonempty : front ++ tail = next :: rest)
      (bound : next &&& W8 224 = W8 192 → d < 32)
      (child : SkipFailureContext tail (d + if next &&& W8 224 = W8 192 then 1 else 0) front leaf) :
      SkipFailureContext tail d ((head :: payload) ++ front) leaf

/-- Valid siblings cannot hide an incomplete wide unsigned child in a complete
    sequence of wide heads and their required argument bytes. -/
theorem skipPrefix_incomplete_wide (items : List (List w8)) (rem : List w8) (d n : Nat)
    (valid : ∀ item ∈ items, SkipValid d item) (short : SkipNonWideUint rem)
    (hcount : items.length < n) (hsize : 9 * n ≤ (items.flatten ++ rem).length)
    (hheads : ∀ i : Nat, i < n → (items.flatten ++ rem)[9 * i]? = some (W8 27)) : False := by
  induction items generalizing n with
  | nil =>
    have hh : rem[0]? = some (W8 27) := by simpa using hheads 0 (by simpa using hcount)
    have hs := short hh
    simp only [List.flatten_nil, List.nil_append] at hsize
    simp only [List.length_nil] at hcount
    omega
  | cons item items ih =>
    have hn : 0 < n := by simp only [List.length_cons] at hcount; omega
    have hv := valid item (by simp)
    have hnonempty := skipValid_nonempty d item hv
    have hraw : (item :: items).flatten ++ rem = item ++ (items.flatten ++ rem) := by
      simp only [List.flatten_cons, List.append_assoc]
    have hh : item[0]? = some (W8 27) := by
      have hh := hheads 0 hn
      simpa only [hraw, Nat.mul_zero, List.getElem?_append, hnonempty, ite_true] using hh
    have hwidth := skipValid_wide_uint d item hv hh
    have hsize' : 9 * (n - 1) ≤ (items.flatten ++ rem).length := by
      rw [hraw, List.length_append, hwidth] at hsize
      omega
    apply ih (n - 1) (fun x hx => valid x (by simp only [List.mem_cons]; exact Or.inr hx))
      (by simp only [List.length_cons] at hcount; omega) hsize'
    intro i hi
    have hg := hheads (i + 1) (by omega)
    rw [hraw] at hg
    simpa only [List.getElem?_append, hwidth, show ¬ 9 * (i + 1) < 9 by omega,
      ite_false, show 9 * (i + 1) - 9 = 9 * i by omega] using hg

/-- An enclosing tag or container cannot turn a failing leaf into a wide uint. -/
theorem skipFailureContext_nonWideUint (tail front : List w8) (d leaf : Nat)
    (context : SkipFailureContext tail d front leaf) (short : SkipNonWideUint tail) :
    SkipNonWideUint (front ++ tail) := by
  cases context with
  | nil => exact short
  | container items enc hm bound more fit valid child =>
    intro heq
    simp only [List.append_assoc, List.cons_append, List.getElem?_cons_zero, Option.some.injEq] at heq
    rcases hm with ⟨hm, _⟩ | ⟨hm, _⟩ <;> rw [heq] at hm <;> contradiction
  | tag enc hm number nonempty bound child =>
    intro heq
    simp only [List.append_assoc, List.cons_append, List.getElem?_cons_zero, Option.some.injEq] at heq
    rw [heq] at hm; contradiction

/-- Contexts preserve the actual shortcut exclusion, including short inputs
    matching all five head bytes and depth-limited wide-shaped arrays. -/
theorem skipFailureContext_wideRejected (tail front : List w8) (d leaf : Nat)
    (context : SkipFailureContext tail d front leaf)
    (short : SkipNonWideUint tail) (rejected : SkipWideRejected leaf tail) :
    SkipWideRejected d (front ++ tail) := by
  intro hd hsize shape
  cases context with
  | nil => exact rejected hd hsize shape
  | @container d leaf head payload front count perCount items enc hm bound more fit valid child =>
    have heq : head = W8 132 := by simpa using shape.1
    subst head
    obtain ⟨rfl, hc⟩ := headArgument_inline_inv (W8 132) payload count enc (by decide)
    rcases hm with ⟨_, rfl⟩ | ⟨hm, _⟩
    · have hcount : items.length < 4 := by rw [hc] at more; change (items.length : Int) < 4 * 1 at more; omega
      apply skipPrefix_incomplete_wide items (front ++ tail) (d + 1) 4 valid
        (skipFailureContext_nonWideUint tail front (d + 1) leaf child short) hcount
      · simp only [List.length_append, List.length_cons, List.length_nil] at hsize ⊢; omega
      · intro i hi
        have hc : i = 0 ∨ i = 1 ∨ i = 2 ∨ i = 3 := by omega
        rcases hc with rfl | rfl | rfl | rfl
        · simpa [List.append_assoc] using shape.2.1
        · simpa [List.append_assoc] using shape.2.2.1
        · simpa [List.append_assoc] using shape.2.2.2.1
        · simpa [List.append_assoc] using shape.2.2.2.2
    · contradiction
  | tag enc hm number nonempty bound child =>
    have heq := shape.1
    simp only [List.append_assoc, List.cons_append, List.getElem?_cons_zero, Option.some.injEq] at heq
    rw [heq] at hm; contradiction

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipPrefix_incomplete_wide
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureContext_nonWideUint
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureContext_wideRejected
