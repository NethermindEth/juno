import ProofCheck.FeltValuesView
import ProofCheck.FeltValuesEmpty

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

/-- The actual nonempty slice decoder allocates typed storage, safely casts its
    view and fills it with arbitrary valid felt encodings. Its contextual budget
    includes the suffix, and its caller's depth is restored on success. -/
theorem wp_limbSlice_nonempty_valid (c : Loc) (v : cursor.Cursor)
    (xs payload item rest : List w8) (values : List (GoArray w64 4))
    (head : w8) (dq : DFrac) (d count : Nat)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 128)
    (nonempty : 0 < count) (fit : 5 * count ≤ (item ++ rest).length)
    (valid : LimbItemsValid (d + 1) count item values)
    (hd : sint.Z v.depth' = d) (hdepth : d + 1 < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (out : GoSlice), RET (PairV #out #interface.nil);
       out ↦* values ∗ ownSliceCap (GoArray w64 4) out (DFrac.own 1) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + item.length)} ∗
       v.buf' ↦*{dq} xs ∗ ⌜out.ptr ≠ null ∧
         sint.Z (v.pos' + W64 (1 + payload.length + item.length)) =
           sint.Z v.pos' + 1 + payload.length + item.length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + item.length)))
           (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hhead := input_consumption xs (head :: payload) (item ++ rest) v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_assoc] using hbytes)
  have Hnext := input_consumption xs ((head :: payload) ++ item) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + item.length + rest.length = sint.Z v.buf'.len := by
    have hlength := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlength
    simp only [sint.nat, sint.Z] at Hlen hlength hp ⊢
    omega
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hfit : (count : Int) < 2^63 := by simp only [List.length_append] at fit; word
  have hcount : 0 ≤ sint.Z (W64 count) := by word
  have hnat : sint.nat (W64 count) = count := by word
  have hn := array_head_not_nullish head major
  have hi := input_suffix_head xs (payload ++ item ++ rest) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ (item ++ rest) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = item ++ rest := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have hexpect := wp_expect_encoded (GF := GF) c v xs payload (item ++ rest) head (W64 count) dq enc hp.1 hbound hi hafter
  rw [major] at hexpect
  wp_apply hexpect $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hok : ¬ uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / W64 5) < uint.Z (W64 count) := by
    simp only [List.length_append] at fit
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
  simp only [hindex, and_self, ite_true]
  simp only [sliceIndexRef, show sint.Z (W64 0) = 0 from rfl, go.arrayIndexRef_0]
  wp_auto
  have hstorage : List.replicate count (zero_val (GoArray w64 4)) ≠ [] := by
    intro h; have := congrArg List.length h; simp only [List.length_replicate, List.length_nil] at this; omega
  wp_apply wp_feltValues_unsafeSlice out (List.replicate count (zero_val (GoArray w64 4)))
    (DFrac.own 1) hstorage $$ [Hout] with Hview
  · iframe Hout
  wp_apply wp_readLimbs_valid c (slice.mk out.ptr out.len out.len)
    (List.replicate count (zero_val (GoArray w64 4))) values
    {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1}
    xs item rest dq (d + 1) count (by simp) valid (by simp only; word) hdepth
    (by simp only; constructor <;> word) hremaining $$ [Hview Hc Hs] with ⟨Hview, Hc, Hs, %Hadvance⟩
  · iframe Hview Hc Hs
  have hvalues : values ≠ [] := by
    have := limbItems_valid_length (d + 1) count item values valid
    intro h; rw [h] at this; simp only [List.length_nil] at this; omega
  ihave Hout := (feltValues_recap out values (DFrac.own 1) hvalues Houtwf.2).2 $$ Hview
  have hrestore : v.depth' + W64 1 - W64 1 = v.depth' := by word
  have hpos : v.pos' + W64 (1 + payload.length) + W64 item.length =
      v.pos' + W64 (1 + payload.length + item.length) := by word
  isimp only [hrestore, hpos] at Hc
  iapply HΦ
  iframe Hout Hcap Hc Hs
  ipureintro
  refine ⟨Hnn, ?_, ?_⟩
  · simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    word
  · simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_nonempty_valid
