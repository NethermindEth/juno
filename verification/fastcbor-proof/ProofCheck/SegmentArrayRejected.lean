import ProofCheck.SegmentRejectedContracts

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def SegmentArrayPartial (count : Nat) (done : List SegmentValue) (retained : SegmentValue) : IProp GF := iprop(
  ∃ out : GoSlice, ⌜out.ptr ≠ null ∧ sint.Z out.len = count⌝ ∗
    SegmentSlots out done ∗
    SegmentOwned (sliceIndexRef SegmentLengths (done.length : Int) out) retained ∗
    SegmentStorage out (done.length + 1) count)

/-- The actual generated slice loop propagates an element rejection after any
    successful prefix, preserving the destination and retaining exact private
    records, consumption, diagnostic position and entered nesting levels. -/
theorem wp_readSegmentArray_array_rejected_of_record (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs payload body : List w8)
    (head : w8) (dq : DFrac) (d : Nat) (records : SegmentRecordRejectedContract (GF := GF) (d + 1)) (count consumed offset : Nat)
    (doneValues : List SegmentValue) (retained : SegmentValue) (childDepth : Nat) (reason : SegmentFailureReason)
    (failure : SegmentItemsRejected (d + 1) count body doneValues retained consumed offset childDepth reason)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fit : count ≤ body.length)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengthsValues)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ SegmentArrayPartial count doneValues retained ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1 + W64 childDepth} ∗
       v.buf' ↦*{dq} xs ∗ SegmentDiagnosticOwned err (v.pos' + W64 (1 + payload.length + offset)) reason ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
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
  wp_apply wp_slice_make2_nonnil (V := SegmentLengths) (W64 count) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  have hnat : sint.nat (W64 count) = count := by word
  isimp only [hnat] at Hout
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  have houtlen : sint.Z out.len = count := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  ihave Hstorage := segmentStorage_init out count $$ Hout
  have Hhead := input_consumption xs (head :: payload) body
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave Hslots : (SegmentSlots out [] : IProp GF) $$ []
  · iapply segmentSlots_nil
  ihave HI : (∃ (done todo : List SegmentValue) (front tail : List w8)
      (leftConsumed leftOffset : Nat) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hslots" ∷ SegmentSlots out done ∗
      "Hstorage" ∷ SegmentStorage out done.length count ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜doneValues = done ++ todo⌝ ∗
      "%Hwire" ∷ ⌜body = front ++ tail⌝ ∗
      "%Hfailure" ∷ ⌜SegmentItemsRejected (d + 1) (count - done.length) tail todo retained leftConsumed leftOffset childDepth reason⌝ ∗
      "%Hconsumed" ∷ ⌜consumed = front.length + leftConsumed⌝ ∗
      "%Hoffset" ∷ ⌜offset = front.length + leftOffset⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + front.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = tail⌝ : IProp GF)
      $$ [Hrange Hi Hslots Hstorage Hc]
  · iexists [], doneValues, [], body, consumed, offset, (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.length_nil, List.nil_append, Nat.sub_zero]
    iframe
    ipureintro
    exact ⟨trivial, trivial, failure, by simp, by simp, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have bounds := segmentItemsRejected_bounds_of_record (d + 1)
    (segment_failure_bounds_depth (32 - (d + 1)) (d + 1) (by omega) rfl).1
    (count - done.length) tail todo retained leftConsumed leftOffset childDepth reason Hfailure
  have hdone : done.length < count := by omega
  have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  have hidx : sint.Z (W64 done.length) = done.length := by word
  simp only [hidx]
  icases segmentStorage_take_cell out done.length count hdone $$ Hstorage with ⟨Hcell, Hstorage⟩
  wp_apply IntoValTyped.wp_load (t := SegmentLengths.ty) _ _ _ $$ Hcell as Hcell
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  simp only [hidx]
  ihave Howned := segmentOwned_zero (sliceIndexRef SegmentLengths (done.length : Int) out) $$ Hcell
  generalize hleft : count - done.length = left at Hfailure
  cases Hfailure with
  | @here _ n wire retained leftConsumed leftOffset childDepth reason reject =>
    wp_apply records c (sliceIndexRef SegmentLengths (done.length : Int) out) segmentZero retained
      {v with pos' := pos, depth' := v.depth' + W64 1} xs tail dq leftConsumed leftOffset childDepth reason
      reject (by word) Hpos Hremaining $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hstep⟩
    · iframe Howned Hc Hs
    simp only at Hstep
    wp_for_post
    have hposfinal : pos + W64 leftConsumed = v.pos' + W64 (1 + payload.length + consumed) := by word
    have hoffsetfinal : pos + W64 leftOffset = v.pos' + W64 (1 + payload.length + offset) := by word
    have hdoneall : doneValues = done := by simpa only [List.append_nil] using Hsplit
    isimp only [hposfinal] at Hc
    isimp only [hoffsetfinal] at Herr
    isimp only [hdoneall] at HΦ
    simp only [hposfinal, hoffsetfinal] at Hstep
    iapply HΦ
    iframe Hdst Hc Hs Herr
    isplitl [Hslots Howned Hstorage]
    · unfold SegmentArrayPartial
      iexists out
      iframe Hslots Howned Hstorage
      ipureintro; exact ⟨Houtnn, houtlen⟩
    · ipureintro
      refine ⟨?_, ?_, Hstep.2.2.1, Hstep.2.2.2⟩ <;> omega
  | @next _ n item tail value todo retained leftConsumed leftOffset childDepth reason first rest =>
    have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ tail := Hremaining
    have hinc : W64 done.length + W64 1 = W64 (done ++ [value]).length := by
      simp only [List.length_append, List.length_singleton]; word
    have hrestcount : count - (done ++ [value]).length = n := by simp only [List.length_append, List.length_singleton]; omega
    wp_apply wp_readSegment_record_valid validator c (sliceIndexRef SegmentLengths (done.length : Int) out) segmentZero value
      {v with pos' := pos, depth' := v.depth' + W64 1} xs item tail dq (d + 1) (by omega)
      first (by word) Hpos hitem $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
    · iframe Howned Hc Hs
    simp only at Hstep
    ihave Hslots := segmentSlots_snoc out done value $$ [Hslots Howned]
    · iframe Hslots Howned
    wp_for_post
    isimp only [hinc, List.length_append, List.length_singleton] at Hrange
    iframe
    iexists (done ++ [value]), todo, (front ++ item), tail, leftConsumed, leftOffset, (pos + W64 item.length), (W64 done.length)
    isimp only [List.length_append, List.length_singleton]
    iframe
    ipureintro
    refine ⟨?_, ?_, ?_, ?_, ?_, ⟨?_, Hstep.2.1⟩, ?_, Hstep.2.2⟩
    · simpa only [List.append_assoc, List.singleton_append] using Hsplit
    · simpa only [List.append_assoc] using Hwire
    · have hcountNext : count - (done.length + 1) = n := by omega
      simpa only [hcountNext] using rest
    · simp only [List.length_append] at *; omega
    · simp only [List.length_append] at *; omega
    · rw [Hstep.1]; omega
    · simp only [List.length_append, Int.natCast_add]
      rw [Hstep.1]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegmentArray_array_rejected_of_record
