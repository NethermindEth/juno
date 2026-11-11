import ProofCheck.ReadLimbsComplete
import Perennial.Golang.Theory.Unsafe

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Owning four limbs supplies the positive address required by the plain-memory
    rules; no separate validity premise for the unsafe pointer is assumed. -/
theorem feltValues_array_pos (p : Loc) (value : GoArray w64 4) (dq : DFrac) :
    (p ↦{dq} value : IProp GF) ⊢ ⌜0 < p.addr⌝ := by
  iintro Hvalue
  ihave %Hlen := array_len p dq 4 value.arr $$ Hvalue
  iapply typedPointsto_array_pos p 4 value dq (by intro h; simp only [h, List.length_nil] at Hlen; omega) $$ Hvalue

local instance : PointstoPos (GF := GF) (GoArray w64 4) := ⟨feltValues_array_pos⟩

/-- Four owned limbs occupy exactly 32 bytes inside the modeled address space. -/
theorem feltValues_array_bounds (p : Loc) (value : GoArray w64 4) (dq : DFrac) :
    (p ↦{dq} value : IProp GF) ⊢ ⌜0 < p.addr ∧ p.addr + 32 ≤ maxAddr⌝ := by
  iintro Hvalue
  ihave %Hpos := feltValues_array_pos p value dq $$ Hvalue
  icases (typedPointsto_array_bytes go.uint64 (.scalar _ _ .isPlainScalar_uint64) p 4 value dq Hpos).1
    $$ Hvalue with ⟨%Hlen, Hbytes⟩
  have hbytes : (value.arr.flatMap encode).length = 32 := by rw [flatMap_encode_length]; omega
  have hnonempty : value.arr.flatMap encode ≠ [] := by intro h; rw [h] at hbytes; contradiction
  ihave Harray := (typedPointsto_byte_array p 32 (array.mk 32 (value.arr.flatMap encode)) dq hnonempty).2 $$ [Hbytes]
  · iframe Hbytes; ipureintro; simp only; omega
  ihave %Hbounds := typedPointsto_byte_array_bounds p 32 (array.mk 32 (value.arr.flatMap encode)) dq hnonempty $$ Harray
  ipureintro; exact Hbounds

/-- An owned nonempty felt slice proves the allocation range needed by
    unsafe.Slice, from the first and last element ownership. -/
theorem feltValues_slice_bounds (out : GoSlice) (values : List (GoArray w64 4)) (dq : DFrac)
    (nonempty : values ≠ []) :
    (out ↦*{dq} values : IProp GF) ⊢ ⌜0 < out.ptr.addr ∧ out.ptr.addr + 32 * values.length ≤ maxAddr⌝ := by
  iintro Hvalues
  ihave %Hpos := ownSlice_ptr_pos out values dq nonempty $$ Hvalues
  have hlen : 0 < values.length := List.length_pos_iff.mpr nonempty
  list_elem values (values.length - 1) as last
  icases ownSlice_elem_acc ((values.length - 1 : Nat) : Int) last out dq values
    (by omega) (by simpa using Hlast_lookup) $$ Hvalues with ⟨Hlast, Hclose⟩
  ihave %Hbound := feltValues_array_bounds (sliceIndexRef (GoArray w64 4) ((values.length - 1 : Nat) : Int) out) last dq $$ Hlast
  have layout : go.PlainLayout (go.ArrayType 4 go.uint64) 32 8 :=
    .array 4 _ 8 8 (.scalar _ _ .isPlainScalar_uint64) (by decide)
  have href := go.arrayIndexRef_plain (go.ArrayType 4 go.uint64) (GoArray w64 4) 32 8
    ((values.length - 1 : Nat) : Int) out.ptr layout Hpos (by omega)
  simp only [sliceIndexRef, href, Loc.add] at Hbound
  ipureintro
  exact ⟨Hpos, by omega⟩

/-- Rebuilding the same nonempty backing slice with capacity equal to length
    preserves ownership. The original capacity bound permits restoring its view. -/
theorem feltValues_recap (out : GoSlice) (values : List (GoArray w64 4)) (dq : DFrac)
    (nonempty : values ≠ []) (hcap : sint.Z out.len ≤ sint.Z out.cap) :
    (out ↦*{dq} values : IProp GF) ⊣⊢ slice.mk out.ptr out.len out.len ↦*{dq} values := by
  rw [ownSlice_unseal]
  unfold ownSliceDef
  constructor
  · iintro (%H | ⟨Harray, %_⟩)
    · exact False.elim (nonempty H.2)
    · iright; iframe Harray; ipureintro; exact Int.le_refl _
  · iintro (%H | ⟨Harray, %_⟩)
    · exact False.elim (nonempty H.2)
    · iright; iframe Harray; ipureintro; exact hcap

local instance : go.IsPlain (go.ArrayType 4 go.uint64) 32 :=
  ⟨8, .array 4 _ 8 8 (.scalar _ _ .isPlainScalar_uint64) (by decide)⟩

/-- The actual unsafe.Slice instantiation used by LimbSlice is safe for owned
    nonempty felt storage. Its pointer and length are unchanged; capacity is length.
    Pointer-range checks are proved from ownership rather than assumed. -/
theorem wp_feltValues_unsafeSlice (out : GoSlice) (values : List (GoArray w64 4)) (dq : DFrac)
    (nonempty : values ≠ []) :
    {{ (out ↦*{dq} values : IProp GF) }}
      (App (App (Val #(functions «unsafe».Slice [go.ArrayType 4 go.uint64, go.int])) (Val #out.ptr)) (Val #out.len))
    {{ RET #(slice.mk out.ptr out.len out.len);
       slice.mk out.ptr out.len out.len ↦*{dq} values }} := by
  wp_start as Hvalues
  ihave %Hlen := ownSlice_len _ _ _ $$ Hvalues
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hvalues
  ihave %Hbounds := feltValues_slice_bounds out values dq nonempty $$ Hvalues
  have hnonneg : ¬ sint.Z out.len < 0 := by omega
  have hnonnull : out.ptr ≠ null := by intro h; rw [h] at Hbounds; simp [null] at Hbounds
  have hover : ¬ 2^64 < out.ptr.addr + sint.Z out.len * 32 := by
    simp only [sint.nat, sint.Z] at Hlen Hbounds ⊢
    unfold maxAddr at Hbounds
    omega
  simp only [show ((32 : Nat) : Int) = 32 from rfl, hnonneg, hnonnull, hover, ite_false]
  wp_auto
  iapply HΦ
  iapply (feltValues_recap out values dq nonempty Hwf.2).1 $$ Hvalues

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValues_array_pos
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValues_array_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValues_slice_bounds

#print axioms Perennial.github_com.NethermindEth.juno.core.feltValues_recap
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_feltValues_unsafeSlice
