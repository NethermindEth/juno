import ProofCheck.StringsValid
import ProofCheck.InitializationErrors

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Failed construction owns the successful strings and all remaining empty
    slots. The caller's old slice is never overwritten on this path. -/
def StringsPartial (count : Nat) (done : List GoString) : IProp GF := iprop(
  ∃ out : GoSlice, ⌜out.ptr ≠ null⌝ ∗ out ↦* (done ++ List.replicate (count - done.length) (go!"")))

/-- The propagated string error retains its initialized category and exact
    diagnostic ownership, independently of the old destination. -/
def StringsElementError (err : Loc) (pos : w64) (reason : StringFailureReason) : IProp GF := iprop(
  ∃ kind : GoInterface, stringFailureAddr (GF := GF) reason ↦□ kind ∗ StringFailureError err pos reason kind)

/-- The actual typed slice loop propagates an element rejection after any
    successful prefix, preserving the destination and retaining exact private
    strings, consumed prefix, diagnostic position and entered array depth. -/
theorem wp_readStrings_array_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs payload body : List w8)
    (head : w8) (dq : DFrac) (d count offset : Nat)
    (doneValues : List GoString) (reason : StringFailureReason)
    (failure : StringsItemsRejected count body doneValues offset reason)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fit : count ≤ body.length)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStrings)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ StringsPartial count doneValues ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + offset), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StringsElementError err (v.pos' + W64 (1 + payload.length + offset)) reason ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + (body).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hfit : (count : Int) < 2 ^ 63 := by word
  have hcount : 0 ≤ sint.Z (W64 count) := by word
  have hi := input_suffix_head xs (payload ++ body)
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
      payload ++ body := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_ArrayLen_encoded c v xs payload body head (W64 count) dq
    enc hm hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_slice_make2_nonnil (V := GoString) (W64 count) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  have hnat : sint.nat (W64 count) = count := by word
  isimp only [hnat] at Hout
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  have houtlen : sint.Z out.len = count := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  have Hhead := input_consumption xs (head :: payload) body
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List GoString) (front tail : List w8)
      (leftOffset : Nat) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hout" ∷ out ↦* (done ++ List.replicate (count - done.length) (go!"")) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜doneValues = done ++ todo⌝ ∗
      "%Hwire" ∷ ⌜body = front ++ tail⌝ ∗
      "%Hfailure" ∷ ⌜StringsItemsRejected (count - done.length) tail todo leftOffset reason⌝ ∗
      "%Hoffset" ∷ ⌜offset = front.length + leftOffset⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + front.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = tail⌝ : IProp GF)
      $$ [Hrange Hi Hout Hc]
  · iexists [], doneValues, [], body, offset, (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.length_nil, List.nil_append, Nat.sub_zero]
    iframe
    ipureintro
    exact ⟨trivial, trivial, failure, by simp, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have bounds := stringsItemsRejected_bounds (count - done.length) leftOffset tail todo reason Hfailure
  have hdone : done.length < count := by omega
  have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  have hidx : sint.Z (W64 done.length) = done.length := by word
  simp only [hidx]
  have hlookup : (done ++ List.replicate (count - done.length) (go!""))[(done.length : Int).toNat]? = some (go!"") := by
    have hlen : count - done.length = (count - done.length - 1) + 1 := by omega
    rw [hlen]
    simp
  icases ownSlice_elem_acc (done.length : Int) (go!"") out (DFrac.own 1)
    (done ++ List.replicate (count - done.length) (go!"")) (by omega) hlookup $$ Hout with ⟨Hcell, Hrestore⟩
  wp_apply IntoValTyped.wp_load (t := go.string) _ _ _ $$ Hcell as Hcell
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  simp only [hidx]
  generalize hleft : count - done.length = left at Hfailure
  cases Hfailure with
  | @here n wire reason reject =>
    isimp only [← hleft] at Hrestore
    ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
    · solve_pkg_init
    icases Hglobals with ⟨%kinds, #Hglobals⟩
    ihave #Hkind := stringErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_readString_rejected validator c (sliceIndexRef GoString (done.length : Int) out) (go!"")
      {v with pos' := pos, depth' := v.depth' + W64 1} xs tail dq reason (stringFailureKind reason kinds)
      reject Hpos Hremaining $$ [Hcell Hc Hs] as %err ⟨Hcell, Hc, Hs, Herr⟩
    · iframe Hkind Hcell Hc Hs
    ihave Hout := Hrestore $$ %(go!"") Hcell
    have hsame : (done ++ List.replicate (count - done.length) (go!"")).set done.length (go!"") =
        done ++ List.replicate (count - done.length) (go!"") := by
      have hlen : count - done.length = (count - done.length - 1) + 1 := by omega
      rw [hlen, initialized_prefix_step]
      simp [List.replicate_succ, List.append_assoc]
    isimp only [Int.toNat_natCast, hsame] at Hout
    wp_for_post
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + offset) := by word
    have hdoneall : doneValues = done := by simpa only [List.append_nil] using Hsplit
    isimp only [hposfinal] at Hc Herr
    isimp only [hdoneall] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Hout]
    · unfold StringsPartial
      iexists out
      iframe Hout
      ipureintro; exact Houtnn
    isplitl [Herr]
    · unfold StringsElementError
      iexists stringFailureKind reason kinds
      iframe Hkind Herr
    ipureintro
    rw [← hposfinal]
    exact ⟨by omega, Hpos.2⟩
  | @next n item tail value todo leftOffset reason first rest =>
    isimp only [← hleft] at Hrestore
    have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ tail := Hremaining
    have hinc : W64 done.length + W64 1 = W64 (done ++ [value]).length := by
      simp only [List.length_append, List.length_singleton]; word
    have hrestcount : count - (done ++ [value]).length = n := by simp only [List.length_append, List.length_singleton]; omega
    wp_apply wp_readString_valid validator c (sliceIndexRef GoString (done.length : Int) out) (go!"")
      {v with pos' := pos, depth' := v.depth' + W64 1} xs item tail value dq
      first Hpos hitem $$ [Hcell Hc Hs] with ⟨Hcell, Hc, Hs, %Hstep⟩
    · iframe Hcell Hc Hs
    simp only at Hstep
    ihave Hout := Hrestore $$ %value Hcell
    have hcountNext : count - done.length = (count - (done.length + 1)) + 1 := by omega
    have houtstep : (done ++ List.replicate (count - done.length) (go!"")).set done.length value =
        (done ++ [value]) ++ List.replicate (count - (done.length + 1)) (go!"") := by
      rw [hcountNext]
      exact initialized_prefix_step done (count - (done.length + 1)) (go!"") value
    isimp only [Int.toNat_natCast, houtstep] at Hout
    wp_for_post
    isimp only [hinc, List.length_append, List.length_singleton] at Hrange
    iframe
    iexists (done ++ [value]), todo, (front ++ item), tail, leftOffset, (pos + W64 item.length), (W64 done.length)
    isimp only [List.length_append, List.length_singleton]
    iframe
    ipureintro
    refine ⟨?_, ?_, ?_, ?_, ⟨?_, Hstep.2.1⟩, ?_, Hstep.2.2⟩
    · simpa only [List.append_assoc, List.singleton_append] using Hsplit
    · simpa only [List.append_assoc] using Hwire
    · have hcountNext : count - (done.length + 1) = n := by omega
      simpa only [hcountNext] using rest
    · simp only [List.length_append] at *; omega
    · rw [Hstep.1]; omega
    · simp only [List.length_append, Int.natCast_add]
      rw [Hstep.1]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStrings_array_rejected
