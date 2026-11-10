import ProofCheck.Signatures

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

/-- Failure after any valid nullable-signature prefix keeps the old destination
    unchanged. The cursor retains outer entry plus the failed child's specified
    progress/depth, and the exact inner diagnostic remains owned. -/
theorem wp_readSignatures_prefix_failure (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (dq : DFrac) (d count consumed offset : Nat) (entered : Bool) (items : List (SignatureWire (d + 1)))
    (reason : FeltFailureReason) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (more : items.length < count)
    (fit : count ≤ (items.flatMap SignatureWire.bytes ++ wire).length)
    (failure : FeltPointersFailure (d + 1) wire consumed offset entered reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ items.flatMap SignatureWire.bytes ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length) + W64 consumed, depth' := if entered then v.depth' + W64 1 + W64 1 else v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ FeltFailureError err
         (v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length) + W64 offset) reason kind }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ items.flatMap SignatureWire.bytes) wire v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + (items.flatMap SignatureWire.bytes).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hbody := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
    List.length_append, List.length_cons] at hbody
  have hfit : (count : Int) < 2 ^ 63 := by
    simp only [sint.nat, sint.Z, List.length_append] at Hlen hp hbody fit ⊢
    word
  have hcount : 0 ≤ sint.Z (W64 count) := by word
  have hcountfit : (1 : Int) * uint.Z (W64 count) ≤
      sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    simp only [sint.nat, sint.Z, List.length_append] at Hlen hp hbody fit
    word
  have hi := input_suffix_head xs (payload ++ items.flatMap SignatureWire.bytes ++ wire)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by
    intro h
    have hc := hm
    rw [h] at hc
    exact (by decide : W8 246 &&& W8 224 ≠ W8 128) hc
  have hu : head ≠ W8 247 := by
    intro h
    have hc := hm
    rw [h] at hc
    exact (by decide : W8 247 &&& W8 224 ≠ W8 128) hc
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (items.flatMap SignatureWire.bytes ++ wire) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_ArrayLen_encoded c v xs payload (items.flatMap SignatureWire.bytes ++ wire) head (W64 count) dq
    enc hm hp.1 (by omega) (by omega) hcountfit hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_slice_make2_nonnil (V := GoSlice) (W64 count) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  have hnat : sint.nat (W64 count) = count := by word
  isimp only [hnat] at Hout
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  have houtlen : sint.Z out.len = count := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  ihave Hchildren := signatureChildren_nil (GF := GF)
  have Hhead := input_consumption xs (head :: payload) (items.flatMap SignatureWire.bytes ++ wire)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      items.flatMap SignatureWire.bytes ++ wire := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (SignatureWire (d + 1))) (slices : List GoSlice) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hout" ∷ out ↦* (slices ++ List.replicate (count - done.length) slice.nil) ∗
      "Hchildren" ∷ SignatureChildren slices (done.map SignatureWire.value) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap SignatureWire.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap SignatureWire.bytes ++ wire⌝ : IProp GF)
      $$ [Hrange Hi Hout Hchildren Hc]
  · iexists [], items, [], (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.map_nil, List.nil_append]
    iframe
    isplitl []
    · iapply signatureChildren_nil
    ipureintro
    exact ⟨trivial, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hdone : done.length ≤ items.length := by omega
  ihave %Hchildrenlen := signatureChildren_length slices (done.map SignatureWire.value) $$ Hchildren
  have hslices : slices.length = done.length := by simpa using Hchildrenlen
  have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  have hleft : 0 < count - done.length := by omega
  have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  have hidxnat : (sint.Z (W64 done.length)).toNat = done.length := by word
  have hlookup : (slices ++ List.replicate (count - done.length) slice.nil)[(sint.Z (W64 done.length)).toNat]? = some slice.nil := by
    rw [hidxnat, ← hslices]
    exact uninitialized_prefix_lookup _ _ _ (by simpa only [hslices] using hleft)
  wp_apply wp_load_slice_index (t := go.SliceType (go.PointerType felt.Felt.ty)) out (sint.Z (W64 done.length))
    (slices ++ List.replicate (count - done.length) slice.nil)
    (DFrac.own 1) slice.nil hindex.1 $$ [Hout] with Hout
  · iframe Hout
    ipureintro; exact hlookup
  cases todo with
  | nil =>
    have hdoneall : done = items := by simpa using Hsplit.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length) := by
      rw [hdoneall] at Hposition; word
    have hwire : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = wire := by simpa using Hremaining
    rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
    wp_auto
    wp_apply wp_readFeltPointers_slice_failure c out (slices ++ List.replicate (count - done.length) slice.nil)
      (sint.Z (W64 done.length)) slice.nil {v with pos' := pos, depth' := v.depth' + W64 1}
      xs wire dq (d + 1) consumed offset entered reason kind hindex.1 hlookup failure
      (by word) (by omega) Hpos hwire $$ [Hout Hc Hs] as %err ⟨Hout, Hc, Hs, Herr⟩
    · iframe Hkind Hout Hc Hs
    wp_for_post
    isimp only [hposfinal] at Hc Herr
    iapply HΦ
    iframe Hdst Hc Hs Herr
  | cons item todo =>
    have hstepcount : count - done.length = (count - (done ++ [item]).length) + 1 := by
      simp only [List.length_cons] at hlength
      simp only [List.length_append, List.length_singleton]
      omega
    have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item.bytes ++ (todo.flatMap SignatureWire.bytes ++ wire) := by
      simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
    rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
    wp_auto
    wp_apply wp_readFeltPointers_slice_valid c out (slices ++ List.replicate (count - done.length) slice.nil)
      (sint.Z (W64 done.length)) slice.nil item.value {v with pos' := pos, depth' := v.depth' + W64 1}
      xs item.bytes (todo.flatMap SignatureWire.bytes ++ wire) dq (d + 1) hindex.1 hlookup
      (by word) (by omega) item.valid Hpos hitem $$ [Hout Hc Hs] as %child ⟨Hout, Hchild, Hc, Hs, %Hadvance⟩
    · iframe Hout Hc Hs
    dsimp only at Hadvance
    have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
      simp only [List.length_append, List.length_singleton]; word
    have hnewlen : ((done ++ [item]).flatMap SignatureWire.bytes).length = (done.flatMap SignatureWire.bytes).length + item.bytes.length := by simp
    have houtstep : (slices ++ List.replicate (count - done.length) slice.nil).set done.length child =
        (slices ++ [child]) ++ List.replicate (count - (done ++ [item]).length) slice.nil := by
      simpa only [hstepcount, hslices] using initialized_prefix_step slices (count - (done ++ [item]).length) slice.nil child
    ihave Hchildren := signatureChildren_snoc slices (done.map SignatureWire.value) child item.value $$ [Hchildren Hchild]
    · iframe Hchildren Hchild
    wp_for_post
    isimp only [hinc] at Hrange
    isimp only [hidxnat, houtstep] at Hout
    iframe
    iexists (done ++ [item]), todo, (slices ++ [child]), (pos + W64 item.bytes.length), (W64 done.length)
    isimp only [List.map_append, List.map_singleton]
    iframe
    ipureintro
    refine ⟨?_, ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
    · simpa only [List.append_assoc, List.singleton_append] using Hsplit
    · rw [Hadvance.1]; omega
    · rw [Hadvance.1, hnewlen]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSignatures_prefix_failure
