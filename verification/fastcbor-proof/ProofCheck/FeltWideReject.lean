import ProofCheck.FeltShortcutReject

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

/-- An unsigned failure after a valid prefix cannot also contain a longer
    sequence of complete nine-byte unsigned items. This is a wire property. -/
theorem unsignedReject_not_wide_prefix (items : List UnsignedWire) (rem : List w8)
    (reject : UnsignedReject rem) (n : Nat) (hcount : items.length < n)
    (hsize : 9 * n ≤ (unsignedBytes items ++ rem).length)
    (hheads : ∀ i : Nat, i < n → (unsignedBytes items ++ rem)[9 * i]? = some (W8 27)) : False := by
  induction items generalizing n with
  | nil =>
    cases rem with
    | nil =>
      simp only [unsignedBytes, List.flatMap_nil, List.nil_append, List.length_nil] at hsize
      simp only [List.length_nil] at hcount
      omega
    | cons head rest =>
      have hh : head = W8 27 := by simpa [unsignedBytes] using hheads 0 (by simpa using hcount)
      have hs := reject.2.2.2.2 hh
      simp only [unsignedBytes, List.flatMap_nil, List.nil_append, List.length_cons] at hsize
      simp only [List.length_nil] at hcount
      omega
  | cons item items ih =>
    have hn : 0 < n := by simp only [List.length_cons] at hcount; omega
    have hh : item.head = W8 27 := by simpa [unsignedBytes, UnsignedWire.bytes] using hheads 0 hn
    have hwidth : item.bytes.length = 9 := by simp only [UnsignedWire.bytes, List.length_cons, (unsignedWire_wide item hh).1]
    have hraw : unsignedBytes (item :: items) ++ rem = item.bytes ++ (unsignedBytes items ++ rem) := by
      simp only [unsignedBytes, List.flatMap_cons, List.append_assoc]
    have hsize' : 9 * (n - 1) ≤ (unsignedBytes items ++ rem).length := by
      rw [hraw] at hsize
      rw [List.length_append, hwidth] at hsize
      omega
    apply ih (n - 1) (by simp only [List.length_cons] at hcount; omega) hsize'
    intro i hi
    have hg := hheads (i + 1) (by omega)
    rw [hraw] at hg
    simpa only [List.getElem?_append, hwidth, show ¬ 9 * (i + 1) < 9 by omega,
      ite_false, show 9 * (i + 1) - 9 = 9 * i by omega] using hg

/-- The four hard-coded wide guards cannot hold on a rejected limb prefix when
    the full 37-byte fast window is available. -/
theorem rejected_felt_not_wide (items : List UnsignedWire) (rem : List w8)
    (hcount : items.length < 4) (reject : UnsignedReject rem)
    (hsize : 37 ≤ (W8 132 :: (unsignedBytes items ++ rem)).length) :
    ¬ ((W8 132 :: (unsignedBytes items ++ rem))[1]? = some (W8 27) ∧
      (W8 132 :: (unsignedBytes items ++ rem))[10]? = some (W8 27) ∧
      (W8 132 :: (unsignedBytes items ++ rem))[19]? = some (W8 27) ∧
      (W8 132 :: (unsignedBytes items ++ rem))[28]? = some (W8 27)) := by
  intro ⟨h1, h10, h19, h28⟩
  apply unsignedReject_not_wide_prefix items rem reject 4 hcount
    (by simp only [List.length_cons] at hsize; omega)
  intro i hi
  have hc : i = 0 ∨ i = 1 ∨ i = 2 ∨ i = 3 := by omega
  rcases hc with rfl | rfl | rfl | rfl
  · simpa using h1
  · simpa using h10
  · simpa using h19
  · simpa using h28

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedReject_not_wide_prefix
#print axioms Perennial.github_com.NethermindEth.juno.core.rejected_felt_not_wide
