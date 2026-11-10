import ProofCheck.SkipContainerPrefix

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

/-- A malformed head reached through supported tags and containers, after valid
    nested siblings. Available-byte and depth guards record when that inner
    diagnostic is reached instead of an earlier count or depth error. -/
inductive SkipHeadFailure (bad : w8) (reason : HeadError) : Nat → List w8 → Nat → Prop where
  | fault {d : Nat} (tail : List w8) (fault : HeadFault bad reason) :
      SkipHeadFailure bad reason d (bad :: tail) 0
  | container {d offset : Nat} {head : w8} {payload tail : List w8} {count perCount : w64}
      (items : List (List w8)) (enc : HeadArgument head payload count)
      (major : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨
        (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
      (bound : d < 32) (more : (items.length : Int) < uint.Z count * uint.Z perCount)
      (fit : uint.Z count * uint.Z perCount ≤ (items.flatten ++ tail).length)
      (valid : ∀ item ∈ items, SkipValid (d + 1) item)
      (child : SkipHeadFailure bad reason (d + 1) tail offset) :
      SkipHeadFailure bad reason d ((head :: payload) ++ (items.flatten ++ tail))
        (1 + payload.length + items.flatten.length + offset)
  | tag {d offset : Nat} {head next : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192)
      (number : 3 < uint.Z argument) (bound : next &&& W8 224 = W8 192 → d < 32)
      (child : SkipHeadFailure bad reason (d + if next &&& W8 224 = W8 192 then 1 else 0) (next :: tail) offset) :
      SkipHeadFailure bad reason d ((head :: payload) ++ (next :: tail)) (1 + payload.length + offset)

/-- Every recursively failing input has an initial byte other than a wide uint
    head, independently of how deep its eventual malformed head lies. -/
theorem skipHeadFailure_first (bad : w8) (reason : HeadError) (d offset : Nat) (wire : List w8)
    (failure : SkipHeadFailure bad reason d wire offset) :
    ∃ head tail, wire = head :: tail ∧ head ≠ W8 27 := by
  cases failure with
  | fault tail fault =>
    have hi := headFault_info bad reason fault
    refine ⟨bad, tail, rfl, ?_⟩
    intro h; subst bad; contradiction
  | container items enc hm bound more fit valid child =>
    refine ⟨_, _, rfl, ?_⟩
    intro h
    rcases hm with ⟨hm, _⟩ | ⟨hm, _⟩ <;> rw [h] at hm <;> contradiction
  | tag enc hm number bound child =>
    refine ⟨_, _, rfl, ?_⟩
    intro h; rw [h] at hm; contradiction

/-- The recursively specified failure never has the complete wide-felt shape,
    including an inline-four container with valid children before its failure. -/
theorem skipHeadFailure_not_wide (bad : w8) (reason : HeadError) (d offset : Nat) (wire : List w8)
    (failure : SkipHeadFailure bad reason d wire offset) : ¬ SkipWideShape wire := by
  intro shape
  cases failure with
  | fault tail fault =>
    have hi := headFault_info bad reason fault
    have heq : bad = W8 132 := by simpa using shape.1
    subst bad; contradiction
  | @container d offset head payload tail count perCount items enc hm bound more fit valid child =>
    have heq : head = W8 132 := by simpa using shape.1
    subst head
    obtain ⟨rfl, hc⟩ := headArgument_inline_inv (W8 132) payload count enc (by decide)
    rcases hm with ⟨_, rfl⟩ | ⟨hm, _⟩
    · have hcount : items.length < 4 := by rw [hc] at more; change (items.length : Int) < 4 * 1 at more; omega
      obtain ⟨first, rest, rfl, hne⟩ := skipHeadFailure_first bad reason (d + 1) offset tail child
      apply skipPrefix_not_wide_heads items first rest (d + 1) 4 valid hne hcount
      intro i hi
      have hc : i = 0 ∨ i = 1 ∨ i = 2 ∨ i = 3 := by omega
      rcases hc with rfl | rfl | rfl | rfl
      · simpa using shape.2.1
      · simpa using shape.2.2.1
      · simpa using shape.2.2.2.1
      · simpa using shape.2.2.2.2
    · contradiction
  | tag enc hm number bound child =>
    have heq := shape.1
    simp only [List.cons_append, List.getElem?_cons_zero, Option.some.injEq] at heq
    rw [heq] at hm
    contradiction

/-- The diagnostic points at a byte actually present in the full remaining input. -/
theorem skipHeadFailure_offset (bad : w8) (reason : HeadError) (d offset : Nat) (wire : List w8)
    (failure : SkipHeadFailure bad reason d wire offset) : offset < wire.length := by
  induction failure with
  | fault tail fault => simp
  | container items enc hm bound more fit valid child ih =>
    simp only [List.length_append, List.length_cons] at ih ⊢; omega
  | tag enc hm number bound child ih =>
    simp only [List.length_append, List.length_cons] at ih ⊢; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipHeadFailure_first
#print axioms Perennial.github_com.NethermindEth.juno.core.skipHeadFailure_not_wide
#print axioms Perennial.github_com.NethermindEth.juno.core.skipHeadFailure_offset
