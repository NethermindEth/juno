import ProofCheck.L2MessagePointersBoundary
import ProofCheck.L2MessageRecordComplete
import ProofCheck.L2MessagePointerItemsSpec
import ProofCheck.SliceAllocation

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

/-- The shared-allocation heuristic uses the signed Go int minimum. -/
theorem wp_l2MessageSharedMin (a b : w64) :
    {{ (True : IProp GF) }}
      (App (App (Val #(functions go.min [go.int, go.int])) (Val #a)) (Val #b))
    {{ RET #(if sint.Z a < sint.Z b then a else b); True }} := by
  wp_start
  by_cases h : sint.Z a < sint.Z b
  · simp only [h, _root_.decide_true, ite_true]
    wp_end
  · simp only [h, _root_.decide_false, ite_false]
    wp_end

/-- The actual pointer-slice reader allocates and fills its shared backing array
    for arbitrary nullable L2ToL1Message elements, retaining the exact unread suffix. -/
theorem wp_readL2MessagePointers_array (validator : Utf8ValidatorContract (GF := GF)) (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac) (d : Nat) (count : Nat) (body : List w8) (values : List (Option L2MessageValue))
    (valid : L2MessagePointerItemsValid (d + 1) count body rem values)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadL2ToL1MessagePointers)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ L2MessagePointersOwned out values ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (body).length)} ∗
       v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (body).length)) =
           sint.Z v.pos' + 1 + payload.length + (body).length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (body).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + (body).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hitems := l2MessagePointerItems_count (d + 1) count body rem values valid
  have hfit : (count : Int) < 2 ^ 63 := by word
  have hcount : 0 ≤ sint.Z (W64 count) := by word
  have hi := input_suffix_head xs (payload ++ body ++ rem)
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
      payload ++ (body ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_ArrayLen_encoded c v xs payload (body ++ rem) head (W64 count) dq
    enc hm hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_slice_make2_nonnil (V := Loc) (W64 count) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  wp_method_call
  wp_call
  wp_auto
  let shareWord := if sint.Z (W64 count) < sint.Z (BitVec.sdiv (v.buf'.len - (v.pos' + W64 (1 + payload.length))) (W64 32)) then
      W64 count else BitVec.sdiv (v.buf'.len - (v.pos' + W64 (1 + payload.length))) (W64 32)
  wp_apply wp_l2MessageSharedMin (W64 count) (BitVec.sdiv (v.buf'.len - (v.pos' + W64 (1 + payload.length))) (W64 32))
  have hshare : 0 ≤ sint.Z shareWord ∧ sint.Z shareWord ≤ count := by
    dsimp only [shareWord]
    split <;> word
  wp_apply wp_slice_make2_nonnil (V := L2ToL1Message) shareWord as %data ⟨Hdata, -, %Hdatann⟩
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
  ihave Hstorage := l2MessageStorage_init data shareCount $$ Hdata
  have Hhead := input_consumption xs (head :: payload) (body ++ rem)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body ++ rem := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave Hchildren : (L2MessagePointerChildren [] [] : IProp GF) $$ []
  · iapply l2MessagePointerChildren_nil
  ihave HI : (∃ (done todo : List (Option L2MessageValue)) (ptrs : List Loc)
      (front tail : List w8) (used : Nat) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hused" ∷ used_ptr ↦ W64 used ∗
      "Hout" ∷ out ↦* (ptrs ++ List.replicate todo.length null) ∗
      "Hchildren" ∷ L2MessagePointerChildren ptrs done ∗
      "Hstorage" ∷ L2MessageStorage data used shareCount ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜values = done ++ todo⌝ ∗
      "%Hwire" ∷ ⌜body = front ++ tail⌝ ∗
      "%Husedbound" ∷ ⌜used ≤ shareCount⌝ ∗
      "%Hvalid" ∷ ⌜L2MessagePointerItemsValid (d + 1) todo.length tail rem todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + front.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = tail ++ rem⌝ : IProp GF)
      $$ [Hrange Hi Hused Hout Hchildren Hstorage Hc]
  · iexists [], values, [], [], body, 0, (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.length_nil, List.nil_append, hitems.1]
    iframe
    ipureintro
    exact ⟨trivial, trivial, Nat.zero_le _, valid, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have hlength : count = done.length + todo.length := by rw [← hitems.1, Hsplit, List.length_append]
  ihave %Hptrlen := l2MessagePointerChildren_length ptrs done $$ Hchildren
  by_cases more : done.length < count
  · have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    generalize htodoCount : todo.length = todoCount at Hvalid
    cases Hvalid with
    | nil => simp only [List.length_nil] at hlength; omega
    | @step n item tail suffix value todo first rest =>
      isimp only [← htodoCount] at Hout
      have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
      rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
      have hidxnat : (sint.Z (W64 done.length)).toNat = ptrs.length := by word
      wp_apply wp_load_slice_index (t := go.PointerType L2ToL1Message.ty) out (sint.Z (W64 done.length))
        (ptrs ++ List.replicate (value :: todo).length null) (DFrac.own 1) null hindex.1
        $$ [Hout] with Hout
      · iframe Hout
        ipureintro
        rw [hidxnat]
        simp
      have hrestcount := (l2MessagePointerItems_count (d + 1) n tail rem todo rest).1
      have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ (tail ++ rem) := by
        simpa only [List.append_assoc] using Hremaining
      have Hadvance := input_consumption xs item (tail ++ rem) pos v.buf'.len Hpos Hlen.1 hitem
      have hinc : W64 done.length + W64 1 = W64 (done ++ [value]).length := by
        simp only [List.length_append, List.length_singleton]; word
      cases first with
      | @nullish b suffix hn =>
        have hi := input_suffix_head xs (tail ++ rem) (sint.nat pos) (sint.nat v.buf'.len) b hitem
        wp_apply wp_null_at c {v with pos' := pos, depth' := v.depth' + W64 1} xs b dq Hpos.1 hi hn
          $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        have houtstep : ptrs ++ List.replicate (none :: todo).length null =
            (ptrs ++ [null]) ++ List.replicate todo.length null := by
          simp [List.replicate_succ, List.append_assoc]
        ihave Hnil : (L2MessagePointerOwned null none : IProp GF) $$ []
        · unfold L2MessagePointerOwned; ipureintro; rfl
        ihave Hchildren := l2MessagePointerChildren_snoc ptrs done null none $$ [Hchildren Hnil]
        · iframe Hchildren Hnil
        wp_for_post
        isimp only [hinc] at Hrange
        isimp only [houtstep] at Hout
        iframe
        iexists (done ++ [none]), todo, (ptrs ++ [null]), (front ++ [b]), tail, used, (pos + W64 1), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ?_, Husedbound, ?_, ?_, ?_, ?_⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.append_assoc, List.singleton_append] using Hwire
        · simpa only [hrestcount] using rest
        · simp only [List.length_singleton, Int.natCast_one] at Hadvance
          exact ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩
        · simp only [List.length_singleton, Int.natCast_one] at Hadvance
          simp only [List.length_append, List.length_singleton, Int.natCast_add, Int.natCast_one]
          omega
        · simpa only [List.length_singleton, Int.natCast_one] using Hadvance.2.2
      | @record item suffix value hn hu record =>
        wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1}
          xs (item ++ (tail ++ rem)) dq hn hu Hpos hitem $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        by_cases hshared : used < shareCount
        · have hcond : sint.Z (W64 used) < sint.Z shareWord := by word
          dsimp only [shareWord] at hcond
          simp only [hcond, _root_.decide_true]
          wp_auto
          have hslot : 0 ≤ sint.Z (W64 used) ∧ sint.Z (W64 used) < sint.Z data.len := by word
          rw [ite_eq_left_of_eq_true _ _ (eq_true hslot)]
          wp_auto
          icases l2MessageStorage_take data used shareCount hshared $$ Hstorage with ⟨Howned, Hstorage⟩
          have husedint : sint.Z (W64 used) = used := by word
          simp only [husedint]
          isimp only [L2MessagePointerOwned] at Howned
          wp_apply wp_readL2Message_record_valid validator c (sliceIndexRef L2ToL1Message (used : Int) data) l2MessageZero value
            {v with pos' := pos, depth' := v.depth' + W64 1} xs item (tail ++ rem) dq (d + 1)
            record (by word) Hpos hitem $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
          · iframe Howned Hc Hs
          simp only at Hstep
          rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
          wp_apply wp_store_slice_index (t := go.PointerType L2ToL1Message.ty) out (sint.Z (W64 done.length))
            (ptrs ++ List.replicate (some value :: todo).length null) (sliceIndexRef L2ToL1Message (used : Int) data) $$ [Hout] with Hout
          · iframe Hout
            ipureintro
            simp only [List.length_append, List.length_cons, List.length_replicate]
            constructor <;> word
          ihave Hchildren := l2MessagePointerChildren_snoc ptrs done (sliceIndexRef L2ToL1Message (used : Int) data) (some value) $$ [Hchildren Howned]
          · unfold L2MessagePointerOwned
            iframe Hchildren Howned
          have houtstep := initialized_prefix_step ptrs todo.length null (sliceIndexRef L2ToL1Message (used : Int) data)
          wp_for_post
          isimp only [hinc] at Hrange
          isimp only [hidxnat, List.length_cons, houtstep] at Hout
          have husednext : W64 used + W64 1 = W64 (used + 1) := by word
          isimp only [husednext] at Hused
          iframe
          iexists (done ++ [some value]), todo, (ptrs ++ [(sliceIndexRef L2ToL1Message (used : Int) data)]), (front ++ item), tail, (used + 1),
            (pos + W64 item.length), (W64 done.length)
          iframe
          ipureintro
          refine ⟨?_, ?_, ?_, ?_, ⟨?_, Hstep.2.1⟩, ?_, Hstep.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa only [List.append_assoc] using Hwire
          · omega
          · simpa only [hrestcount] using rest
          · rw [Hstep.1]; omega
          · simp only [List.length_append, Int.natCast_add]
            rw [Hstep.1]; omega
        · have hcond : ¬ sint.Z (W64 used) < sint.Z shareWord := by word
          dsimp only [shareWord] at hcond
          simp only [hcond, _root_.decide_false]
          wp_auto
          ihave Howned := l2MessagePointerOwned_zero «$r0_ptr» $$ «$r0»
          isimp only [L2MessagePointerOwned] at Howned
          wp_apply wp_readL2Message_record_valid validator c «$r0_ptr» l2MessageZero value
            {v with pos' := pos, depth' := v.depth' + W64 1} xs item (tail ++ rem) dq (d + 1)
            record (by word) Hpos hitem $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
          · iframe Howned Hc Hs
          simp only at Hstep
          rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
          wp_apply wp_store_slice_index (t := go.PointerType L2ToL1Message.ty) out (sint.Z (W64 done.length))
            (ptrs ++ List.replicate (some value :: todo).length null) «$r0_ptr» $$ [Hout] with Hout
          · iframe Hout
            ipureintro
            simp only [List.length_append, List.length_cons, List.length_replicate]
            constructor <;> word
          ihave Hchildren := l2MessagePointerChildren_snoc ptrs done «$r0_ptr» (some value) $$ [Hchildren Howned]
          · unfold L2MessagePointerOwned
            iframe Hchildren Howned
          have houtstep := initialized_prefix_step ptrs todo.length null «$r0_ptr»
          wp_for_post
          isimp only [hinc] at Hrange
          isimp only [hidxnat, List.length_cons, houtstep] at Hout
          iframe
          iexists (done ++ [some value]), todo, (ptrs ++ [«$r0_ptr»]), (front ++ item), tail, used,
            (pos + W64 item.length), (W64 done.length)
          iframe
          ipureintro
          refine ⟨?_, ?_, ?_, ?_, ⟨?_, Hstep.2.1⟩, ?_, Hstep.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa only [List.append_assoc] using Hwire
          · omega
          · simpa only [hrestcount] using rest
          · rw [Hstep.1]; omega
          · simp only [List.length_append, Int.natCast_add]
            rw [Hstep.1]; omega
  · have hcond : ¬ sint.Z (W64 done.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    have hdoneall : done = values := by simpa [htodo] using Hsplit.symm
    have htail : tail = [] := by
      rw [htodo] at Hvalid
      cases Hvalid
      rfl
    have hfront : front = body := by simpa [htail] using Hwire.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + body.length) := by
      rw [hfront] at Hposition; word
    wp_apply wp_leave c {v with pos' := pos} $$ [Hc] with Hc
    · iframe Hc
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hposfinal, hrestoredepth] at Hc
    isimp only [htodo, List.length_nil, List.replicate_zero, List.append_nil] at Hout
    isimp only [hdoneall] at Hchildren
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Hout Hchildren]
    · unfold L2MessagePointersOwned
      iexists ptrs
      iframe Hout Hchildren
      ipureintro; exact Houtnn
    · ipureintro
      simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1, Int.add_assoc] using And.intro Hnext.1 Hnext.2.2

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_l2MessageSharedMin
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readL2MessagePointers_array
