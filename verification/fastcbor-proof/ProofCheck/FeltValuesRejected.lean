import ProofCheck.FeltValuesNonempty

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

local instance : go.IsPlain (go.ArrayType 4 go.uint64) 32 :=
  ⟨8, .array 4 _ 8 8 (.scalar _ _ .isPlainScalar_uint64) (by decide)⟩

/-- Any rejected element after an arbitrary decoded prefix causes the actual
    slice wrapper to return nil and restore the entire original cursor. The
    error retains the exact inner offset rather than the rolled-back cursor. -/
theorem wp_limbSlice_nonempty_rejected (c : Loc) (v : cursor.Cursor)
    (xs payload wire : List w8) (done : List (GoArray w64 4))
    (head : w8) (dq : DFrac) (d count consumed offset : Nat)
    (reason : FeltFailureReason) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 128)
    (nonempty : 0 < count) (fit : 5 * count ≤ wire.length)
    (failure : LimbItemsRejected (d + 1) count wire done consumed offset reason)
    (hd : sint.Z v.depth' = d) (hdepth : d + 1 < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       FeltFailureError err (v.pos' + W64 (1 + payload.length + offset)) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hhead := input_consumption xs (head :: payload) wire v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_assoc] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + wire.length = sint.Z v.buf'.len := by
    have hlength := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlength
    simp only [sint.nat, sint.Z] at Hlen hlength hp ⊢
    omega
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hfit : (count : Int) < 2^63 := by word
  have hcount : 0 ≤ sint.Z (W64 count) := by word
  have hnat : sint.nat (W64 count) = count := by word
  have hn := array_head_not_nullish head major
  have hi := input_suffix_head xs (payload ++ wire) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ wire := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = wire := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have hexpect := wp_expect_encoded (GF := GF) c v xs payload wire head (W64 count) dq enc hp.1 hbound hi hafter
  rw [major] at hexpect
  wp_apply hexpect $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hok : ¬ uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / W64 5) < uint.Z (W64 count) := by
    word
  simp only [hok, _root_.decide_false]
  wp_auto
  wp_apply wp_enter c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' (by simp only; omega)
    $$ [Hc] with Hc
  · iframe Hc
  have hpositive : uint.Z (W64 0) < uint.Z (W64 count) := by word
  have hshallow : ¬ sint.Z (W64 32) ≤ sint.Z (v.depth' + W64 1) := by word
  simp only [hpositive, _root_.decide_true]
  wp_auto
  simp only [hshallow, _root_.decide_false]
  wp_auto
  wp_apply wp_slice_make2_nonnil (V := GoArray w64 4) (W64 count) as %out ⟨Hout, Hcap, %Hnn⟩
  · ipureintro; exact hcount
  isimp only [hnat] at Hout
  ihave %Houtlen := ownSlice_len _ _ _ $$ Hout
  ihave %Houtwf := ownSlice_wf _ _ _ $$ Hout
  have houtlen : sint.Z out.len = count := by simp only [List.length_replicate, sint.nat, sint.Z] at Houtlen ⊢; omega
  have houtnz : out.len ≠ W64 0 := by word
  simp only [houtnz, _root_.decide_false]
  wp_auto
  have hindex : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) < sint.Z out.len := by word
  simp only [hindex, _root_.and_self, ite_true]
  simp only [sliceIndexRef, show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0]
  wp_auto
  have hstorage : List.replicate count (zero_val (GoArray w64 4)) ≠ [] := by
    intro h; have := congrArg List.length h; simp only [List.length_replicate, List.length_nil] at this; omega
  wp_apply wp_feltValues_unsafeSlice out (List.replicate count (zero_val (GoArray w64 4)))
    (DFrac.own 1) hstorage $$ [Hout] with Hview
  · iframe Hout
  wp_apply wp_readLimbs_rejected c (slice.mk out.ptr out.len out.len)
    (List.replicate count (zero_val (GoArray w64 4))) done
    {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1}
    xs wire dq (d + 1) count consumed offset reason kind (by simp) failure
    (by simp only; word) hdepth (by simp only; constructor <;> word) hremaining
    $$ [Hview Hc Hs] as %err ⟨Hview, Hc, Hs, Herr⟩
  · iframe Hkind Hview Hc Hs
  have hrestore : v.depth' + W64 1 - W64 1 = v.depth' := by word
  have hoffset : v.pos' + W64 (1 + payload.length) + W64 offset =
      v.pos' + W64 (1 + payload.length + offset) := by word
  isimp only [hrestore] at Hc
  isimp only [hoffset] at Herr
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_nonempty_rejected
