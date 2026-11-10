import ProofCheck.LimbsComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Borrow one four-word slot from a shared backing allocation, decode into it,
    and restore ownership of the whole slice with exactly that element updated.
    The other slots, including already-decoded elements, remain framed. -/
theorem wp_limbs_slice_valid (c : Loc) (data : GoSlice) (values : List (GoArray w64 4))
    (i : Int) (old words : GoArray w64 4) (v : cursor.Cursor)
    (xs wire rem : List w8) (dq : DFrac) (d : Nat)
    (hi : 0 ≤ i) (hlookup : values[i.toNat]? = some old)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : FeltValid d wire (some words))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       data ↦* values ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs"))
        (Val #(sliceIndexRef (GoArray w64 4) i data)))
    {{ RET #interface.nil; data ↦* values.set i.toNat words ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hvalues, Hc, Hs⟩
  icases ownSlice_elem_acc i old data (DFrac.own 1) values hi hlookup $$ Hvalues with ⟨Hslot, Hclose⟩
  wp_apply wp_limbs_valid c (sliceIndexRef (GoArray w64 4) i data) old words v xs wire rem dq d
    hd hdepth valid hp hbytes $$ [Hslot Hc Hs] with ⟨Hslot, Hc, Hs, %Hnext⟩
  · iframe Hslot Hc Hs
  ihave Hvalues := Hclose $$ %words Hslot
  wp_end

/-- A failed in-place decode leaves the entire shared backing allocation
    unchanged, including the target slot and all earlier successful elements. -/
theorem wp_limbs_slice_failure (c : Loc) (data : GoSlice) (values : List (GoArray w64 4))
    (i : Int) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (hi : 0 ≤ i) (hlookup : values[i.toNat]? = some old)
    (failure : LimbsFailure d wire offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ data ↦* values ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs"))
        (Val #(sliceIndexRef (GoArray w64 4) i data)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       data ↦* values ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hvalues, Hc, Hs⟩
  icases ownSlice_elem_acc i old data (DFrac.own 1) values hi hlookup $$ Hvalues with ⟨Hslot, Hclose⟩
  wp_apply wp_limbs_failure c (sliceIndexRef (GoArray w64 4) i data) old v xs wire dq d offset reason kind
    failure hd hdepth hp hbytes $$ [Hslot Hc Hs] as %err ⟨Hslot, Hc, Hs, Herr⟩
  · iframe Hkind Hslot Hc Hs
  ihave Hvalues := Hclose $$ %old Hslot
  have hidx := (List.getElem?_eq_some_iff.mp hlookup).choose
  have hval : values[i.toNat]'hidx = old :=
    Option.some.inj ((List.getElem?_eq_getElem hidx).symm.trans hlookup)
  have hset : values.set i.toNat old = values := by
    rw [← hval]
    exact List.set_getElem_self hidx
  isimp only [hset] at Hvalues
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_slice_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_slice_failure
