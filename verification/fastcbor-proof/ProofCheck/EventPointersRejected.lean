import ProofCheck.EventPointersValid

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

/-- On an element error the temporary table, successful records and failed
    partial Event stay private; none is published through the destination. -/
def EventPointersPartial (count : Nat) (done : List (Option EventValue)) (retained : EventValue) : IProp GF := iprop(
  ∃ (out : GoSlice) (ptrs : List Loc) (failed : Loc), ⌜out.ptr ≠ null⌝ ∗
    out ↦* (ptrs ++ List.replicate (count - done.length) null) ∗
    EventPointerChildren ptrs done ∗ EventOwned failed retained)

/-- The actual generated slice loop propagates an element rejection after any
    successful prefix, preserving the destination and retaining exact private
    records, consumption, diagnostic position and entered nesting levels. -/
theorem wp_readEventPointers_array_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs payload body : List w8)
    (head : w8) (dq : DFrac) (d count consumed offset : Nat)
    (doneValues : List (Option EventValue)) (retained : EventValue) (entered : Bool) (reason : EventFailureReason)
    (failure : EventPointerItemsRejected (d + 1) count body doneValues retained consumed offset entered reason)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fit : count ≤ body.length)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadEventPointers)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ EventPointersPartial count doneValues retained ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := if entered then v.depth' + W64 1 + W64 1 else v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ EventFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) reason ∗
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
  wp_apply wp_slice_make2_nonnil (V := Loc) (W64 count) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  wp_method_call
  wp_call
  wp_auto
  let shareWord := if sint.Z (W64 count) < sint.Z (BitVec.sdiv (v.buf'.len - (v.pos' + W64 (1 + payload.length))) (W64 32)) then
      W64 count else BitVec.sdiv (v.buf'.len - (v.pos' + W64 (1 + payload.length))) (W64 32)
  wp_apply wp_eventSharedMin (W64 count) (BitVec.sdiv (v.buf'.len - (v.pos' + W64 (1 + payload.length))) (W64 32))
  have hshare : 0 ≤ sint.Z shareWord ∧ sint.Z shareWord ≤ count := by
    dsimp only [shareWord]
    split <;> word
  wp_apply wp_slice_make2_nonnil (V := Event) shareWord as %data ⟨Hdata, -, %Hdatann⟩
  · ipureintro; exact hshare.1
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  irename used => Hused
  let shareCount := sint.nat shareWord
  have hsharenat : (shareCount : Int) = sint.Z shareWord := by dsimp only [shareCount]; word
  have hsharebound : shareCount ≤ count := by omega
  have hnat : sint.nat (W64 count) = count := by word
  isimp only [hnat] at Hout
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  ihave %Hdatasize := ownSlice_len _ _ _ $$ Hdata
  have houtlen : sint.Z out.len = count := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  have hdatalen : sint.Z data.len = shareCount := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Hdatasize ⊢
    dsimp only [shareCount, sint.nat, sint.Z]; omega
  ihave Hstorage := eventStorage_init data shareCount $$ Hdata
  have Hhead := input_consumption xs (head :: payload) body
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave Hchildren : (EventPointerChildren [] [] : IProp GF) $$ []
  · iapply eventPointerChildren_nil
  ihave HI : (∃ (done todo : List (Option EventValue)) (ptrs : List Loc)
      (front tail : List w8) (used leftConsumed leftOffset : Nat) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hused" ∷ used_ptr ↦ W64 used ∗
      "Hout" ∷ out ↦* (ptrs ++ List.replicate (count - done.length) null) ∗
      "Hchildren" ∷ EventPointerChildren ptrs done ∗
      "Hstorage" ∷ EventStorage data used shareCount ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜doneValues = done ++ todo⌝ ∗
      "%Hwire" ∷ ⌜body = front ++ tail⌝ ∗
      "%Husedbound" ∷ ⌜used ≤ shareCount⌝ ∗
      "%Hfailure" ∷ ⌜EventPointerItemsRejected (d + 1) (count - done.length) tail todo retained leftConsumed leftOffset entered reason⌝ ∗
      "%Hconsumed" ∷ ⌜consumed = front.length + leftConsumed⌝ ∗
      "%Hoffset" ∷ ⌜offset = front.length + leftOffset⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + front.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = tail⌝ : IProp GF)
      $$ [Hrange Hi Hused Hout Hchildren Hstorage Hc]
  · iexists [], doneValues, [], [], body, 0, consumed, offset, (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.length_nil, List.nil_append, Nat.sub_zero]
    iframe
    ipureintro
    exact ⟨trivial, trivial, Nat.zero_le _, failure, by simp, by simp, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have bounds := eventPointerItemsRejected_bounds (d + 1) (count - done.length) leftConsumed leftOffset tail todo retained entered reason Hfailure
  have hdone : done.length < count := by omega
  ihave %Hptrlen := eventPointerChildren_length ptrs done $$ Hchildren
  have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  have hidxnat : (sint.Z (W64 done.length)).toNat = ptrs.length := by word
  wp_apply wp_load_slice_index (t := go.PointerType Event.ty) out (sint.Z (W64 done.length))
    (ptrs ++ List.replicate (count - done.length) null) (DFrac.own 1) null hindex.1
    $$ [Hout] with Hout
  · iframe Hout
    ipureintro
    rw [hidxnat]
    exact uninitialized_prefix_lookup ptrs (count - done.length) null (by omega)
  generalize hleft : count - done.length = left at Hfailure
  cases Hfailure with
  | @here n wire retained leftConsumed leftOffset entered reason fail =>
    isimp only [← hleft] at Hout
    cases fail with
    | record hn hu reject =>
      wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1}
        xs tail dq hn hu Hpos Hremaining $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      by_cases hshared : used < shareCount
      · have hcond : sint.Z (W64 used) < sint.Z shareWord := by word
        dsimp only [shareWord] at hcond
        simp only [hcond, _root_.decide_true]
        wp_auto
        have hslot : 0 ≤ sint.Z (W64 used) ∧ sint.Z (W64 used) < sint.Z data.len := by word
        rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
        wp_auto
        icases eventStorage_take data used shareCount hshared $$ Hstorage with ⟨Howned, Hstorage⟩
        have husedint : sint.Z (W64 used) = used := by word
        simp only [husedint]
        isimp only [EventPointerOwned] at Howned
        wp_apply wp_readEvent_record_rejected validator c (sliceIndexRef Event (used : Int) data) eventZero retained
          {v with pos' := pos, depth' := v.depth' + W64 1} xs tail dq (d + 1) leftConsumed leftOffset entered reason
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
        isplitl [Hout Hchildren Howned]
        · unfold EventPointersPartial
          iexists out, ptrs, (sliceIndexRef Event (used : Int) data)
          iframe Hout Hchildren Howned
          ipureintro; exact Houtnn
        · ipureintro
          refine ⟨?_, ?_, Hstep.2.2.1, Hstep.2.2.2⟩ <;> omega
      · have hcond : ¬ sint.Z (W64 used) < sint.Z shareWord := by word
        dsimp only [shareWord] at hcond
        simp only [hcond, _root_.decide_false]
        wp_auto
        ihave Howned := eventPointerOwned_zero «$r0_ptr» $$ «$r0»
        isimp only [EventPointerOwned] at Howned
        wp_apply wp_readEvent_record_rejected validator c «$r0_ptr» eventZero retained
          {v with pos' := pos, depth' := v.depth' + W64 1} xs tail dq (d + 1) leftConsumed leftOffset entered reason
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
        isplitl [Hout Hchildren Howned]
        · unfold EventPointersPartial
          iexists out, ptrs, «$r0_ptr»
          iframe Hout Hchildren Howned
          ipureintro; exact Houtnn
        · ipureintro
          refine ⟨?_, ?_, Hstep.2.2.1, Hstep.2.2.2⟩ <;> omega

  | @next n item tail value todo retained leftConsumed leftOffset entered reason first rest =>
    isimp only [← hleft] at Hout
    have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ tail := Hremaining
    have Hadvance := input_consumption xs item tail pos v.buf'.len Hpos Hlen.1 hitem
    have hinc : W64 done.length + W64 1 = W64 (done ++ [value]).length := by
      simp only [List.length_append, List.length_singleton]; word
    have hstepcount : count - done.length = (count - (done ++ [value]).length) + 1 := by simp only [List.length_append, List.length_singleton]; omega
    have hrestcount : count - (done ++ [value]).length = n := by simp only [List.length_append, List.length_singleton]; omega
    cases first with
    | @nullish b suffix hn =>
      have hi := input_suffix_head xs tail (sint.nat pos) (sint.nat v.buf'.len) b hitem
      wp_apply wp_null_at c {v with pos' := pos, depth' := v.depth' + W64 1} xs b dq Hpos.1 hi hn
        $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      have houtstep : ptrs ++ List.replicate (count - done.length) null =
          (ptrs ++ [null]) ++ List.replicate (count - (done ++ [none]).length) null := by
        rw [hstepcount]
        simp [List.replicate_succ, List.append_assoc]
      ihave Hnil : (EventPointerOwned null none : IProp GF) $$ []
      · unfold EventPointerOwned; ipureintro; rfl
      ihave Hchildren := eventPointerChildren_snoc ptrs done null none $$ [Hchildren Hnil]
      · iframe Hchildren Hnil
      wp_for_post
      isimp only [hinc] at Hrange
      isimp only [houtstep] at Hout
      iframe
      iexists (done ++ [none]), todo, (ptrs ++ [null]), (front ++ [b]), tail, used, leftConsumed, leftOffset, (pos + W64 1), (W64 done.length)
      iframe
      ipureintro
      refine ⟨?_, ?_, Husedbound, ?_, ?_, ?_, ?_, ?_, ?_⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hsplit
      · simpa only [List.append_assoc, List.singleton_append] using Hwire
      · simpa only [hrestcount] using rest
      · simp only [List.length_append, List.length_singleton] at *; omega
      · simp only [List.length_append, List.length_singleton] at *; omega
      · simp only [List.length_singleton, Int.natCast_one] at Hadvance
        exact ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩
      · simp only [List.length_singleton, Int.natCast_one] at Hadvance
        simp only [List.length_append, List.length_singleton, Int.natCast_add, Int.natCast_one]
        omega
      · simpa only [List.length_singleton, Int.natCast_one] using Hadvance.2.2
    | @record item suffix value hn hu record =>
      wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1}
        xs (item ++ tail) dq hn hu Hpos hitem $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      by_cases hshared : used < shareCount
      · have hcond : sint.Z (W64 used) < sint.Z shareWord := by word
        dsimp only [shareWord] at hcond
        simp only [hcond, _root_.decide_true]
        wp_auto
        have hslot : 0 ≤ sint.Z (W64 used) ∧ sint.Z (W64 used) < sint.Z data.len := by word
        rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
        wp_auto
        icases eventStorage_take data used shareCount hshared $$ Hstorage with ⟨Howned, Hstorage⟩
        have husedint : sint.Z (W64 used) = used := by word
        simp only [husedint]
        isimp only [EventPointerOwned] at Howned
        wp_apply wp_readEvent_record_valid validator c (sliceIndexRef Event (used : Int) data) eventZero value
          {v with pos' := pos, depth' := v.depth' + W64 1} xs item tail dq (d + 1)
          record (by word) Hpos hitem $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
        · iframe Howned Hc Hs
        simp only at Hstep
        rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
        wp_apply wp_store_slice_index (t := go.PointerType Event.ty) out (sint.Z (W64 done.length))
          (ptrs ++ List.replicate (count - done.length) null) (sliceIndexRef Event (used : Int) data) $$ [Hout] with Hout
        · iframe Hout
          ipureintro
          simp only [List.length_append, List.length_replicate]
          constructor <;> word
        ihave Hchildren := eventPointerChildren_snoc ptrs done (sliceIndexRef Event (used : Int) data) (some value) $$ [Hchildren Howned]
        · unfold EventPointerOwned
          iframe Hchildren Howned
        have houtstep := initialized_prefix_step ptrs (count - (done ++ [some value]).length) null (sliceIndexRef Event (used : Int) data)
        wp_for_post
        isimp only [hinc] at Hrange
        isimp only [hidxnat, hstepcount, houtstep] at Hout
        have husednext : W64 used + W64 1 = W64 (used + 1) := by word
        isimp only [husednext] at Hused
        iframe
        iexists (done ++ [some value]), todo, (ptrs ++ [(sliceIndexRef Event (used : Int) data)]), (front ++ item), tail, (used + 1), leftConsumed, leftOffset,
          (pos + W64 item.length), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ?_, ?_, ?_, ?_, ?_, ⟨?_, Hstep.2.1⟩, ?_, Hstep.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.append_assoc] using Hwire
        · omega
        · simpa only [hrestcount] using rest
        · simp only [List.length_append] at *; omega
        · simp only [List.length_append] at *; omega
        · rw [Hstep.1]; omega
        · simp only [List.length_append, Int.natCast_add]
          rw [Hstep.1]; omega
      · have hcond : ¬ sint.Z (W64 used) < sint.Z shareWord := by word
        dsimp only [shareWord] at hcond
        simp only [hcond, _root_.decide_false]
        wp_auto
        ihave Howned := eventPointerOwned_zero «$r0_ptr» $$ «$r0»
        isimp only [EventPointerOwned] at Howned
        wp_apply wp_readEvent_record_valid validator c «$r0_ptr» eventZero value
          {v with pos' := pos, depth' := v.depth' + W64 1} xs item tail dq (d + 1)
          record (by word) Hpos hitem $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
        · iframe Howned Hc Hs
        simp only at Hstep
        rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
        wp_apply wp_store_slice_index (t := go.PointerType Event.ty) out (sint.Z (W64 done.length))
          (ptrs ++ List.replicate (count - done.length) null) «$r0_ptr» $$ [Hout] with Hout
        · iframe Hout
          ipureintro
          simp only [List.length_append, List.length_replicate]
          constructor <;> word
        ihave Hchildren := eventPointerChildren_snoc ptrs done «$r0_ptr» (some value) $$ [Hchildren Howned]
        · unfold EventPointerOwned
          iframe Hchildren Howned
        have houtstep := initialized_prefix_step ptrs (count - (done ++ [some value]).length) null «$r0_ptr»
        wp_for_post
        isimp only [hinc] at Hrange
        isimp only [hidxnat, hstepcount, houtstep] at Hout
        iframe
        iexists (done ++ [some value]), todo, (ptrs ++ [«$r0_ptr»]), (front ++ item), tail, used, leftConsumed, leftOffset,
          (pos + W64 item.length), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ?_, ?_, ?_, ?_, ?_, ⟨?_, Hstep.2.1⟩, ?_, Hstep.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.append_assoc] using Hwire
        · omega
        · simpa only [hrestcount] using rest
        · simp only [List.length_append] at *; omega
        · simp only [List.length_append] at *; omega
        · rw [Hstep.1]; omega
        · simp only [List.length_append, Int.natCast_add]
          rw [Hstep.1]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readEventPointers_array_rejected
