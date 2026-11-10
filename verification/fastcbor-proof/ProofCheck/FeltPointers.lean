import ProofCheck.FeltPointersSpec

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

/-- The actual pointer-slice reader allocates and fills its shared backing array
    for arbitrary nullable felt elements, retaining the exact unread suffix. -/
theorem wp_readFeltPointers_array (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac) (d : Nat) (items : List (FeltWire (d + 1)))
    (enc : HeadArgument head payload (W64 items.length)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ items.flatMap FeltWire.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ FeltPointersOwned out (items.map FeltWire.value) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length)} ∗
       v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length)) =
           sint.Z v.pos' + 1 + payload.length + (items.flatMap FeltWire.bytes).length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ items.flatMap FeltWire.bytes) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + (items.flatMap FeltWire.bytes).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hitems := feltWire_count_bound (d + 1) items
  have hfit : (items.length : Int) < 2 ^ 63 := by word
  have hcount : 0 ≤ sint.Z (W64 items.length) := by word
  have hi := input_suffix_head xs (payload ++ items.flatMap FeltWire.bytes ++ rem)
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
      payload ++ (items.flatMap FeltWire.bytes ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_ArrayLen_encoded c v xs payload (items.flatMap FeltWire.bytes ++ rem) head (W64 items.length) dq
    enc hm hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_slice_make2_nonnil (V := Loc) (W64 items.length) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  wp_apply wp_slice_make2_nonnil (V := GoArray w64 4) (W64 items.length) as %data ⟨Hvalues, -, %Hdatann⟩
  · ipureintro; exact hcount
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  have hnat : sint.nat (W64 items.length) = items.length := by word
  isimp only [hnat] at Hout Hvalues
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  ihave %Hdatasize := ownSlice_len _ _ _ $$ Hvalues
  have houtlen : sint.Z out.len = items.length := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  have hdatalen : sint.Z data.len = items.length := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Hdatasize ⊢; omega
  have Hhead := input_consumption xs (head :: payload) (items.flatMap FeltWire.bytes ++ rem)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      items.flatMap FeltWire.bytes ++ rem := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (FeltWire (d + 1))) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hout" ∷ out ↦* (feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate todo.length null) ∗
      "Hvalues" ∷ data ↦* (feltStorage (done.map FeltWire.value) ++ List.replicate todo.length (zero_val (GoArray w64 4))) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap FeltWire.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap FeltWire.bytes ++ rem⌝ : IProp GF)
      $$ [Hrange Hi Hout Hvalues Hc]
  · iexists [], items, (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.map_nil, feltPointers, List.mapIdx_nil, feltStorage, List.nil_append]
    iframe
    ipureintro
    exact ⟨trivial, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hdone : done.length ≤ items.length := by omega
  by_cases more : done.length < items.length
  · have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    cases todo with
    | nil => simp only [List.length_nil] at hlength; omega
    | cons item todo =>
      have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
      rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
      have hpl : (feltPointers (GF := GF) data (done.map FeltWire.value)).length = done.length := by simp [feltPointers]
      have hidxnat : (sint.Z (W64 done.length)).toNat = done.length := by word
      wp_apply wp_load_slice_index (t := go.PointerType felt.Felt.ty) out (sint.Z (W64 done.length))
        (feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (item :: todo).length null)
        (DFrac.own 1) null hindex.1 $$ [Hout] with Hout
      · iframe Hout
        ipureintro
        rw [hidxnat, ← hpl]
        simp
      have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item.bytes ++ (todo.flatMap FeltWire.bytes ++ rem) := by
        simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
      have Hadvance := input_consumption xs item.bytes (todo.flatMap FeltWire.bytes ++ rem) pos v.buf'.len Hpos Hlen.1 hitem
      have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
        simp only [List.length_append, List.length_singleton]; word
      have hnewlen : ((done ++ [item]).flatMap FeltWire.bytes).length = (done.flatMap FeltWire.bytes).length + item.bytes.length := by simp
      cases hv : item.value with
      | none =>
        obtain ⟨b, hb, hnull⟩ := feltValid_none (d + 1) item.bytes (by rw [← hv]; exact item.valid)
        have hbi : xs[(sint.Z pos).toNat]? = some b :=
          input_suffix_head xs (todo.flatMap FeltWire.bytes ++ rem) (sint.nat pos) (sint.nat v.buf'.len) b
            (by simpa only [hb, List.singleton_append] using hitem)
        wp_apply wp_null_at c {v with pos' := pos, depth' := v.depth' + W64 1} xs b dq Hpos.1 hbi hnull
          $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        have hposstep : pos + W64 1 = pos + W64 item.bytes.length := by rw [hb]; rfl
        have houtstep : feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (item :: todo).length null =
            feltPointers (GF := GF) data ((done ++ [item]).map FeltWire.value) ++ List.replicate todo.length null := by
          simp [List.map_append, feltPointers_step, feltPointer, hv, List.replicate_succ, List.append_assoc]
        have hvaluesstep : feltStorage (done.map FeltWire.value) ++ List.replicate (item :: todo).length (zero_val (GoArray w64 4)) =
            feltStorage ((done ++ [item]).map FeltWire.value) ++ List.replicate todo.length (zero_val (GoArray w64 4)) := by
          simp [feltStorage, hv, List.replicate_succ, List.append_assoc]
        wp_for_post
        isimp only [hinc] at Hrange
        isimp only [hposstep] at Hc
        isimp only [houtstep] at Hout
        isimp only [hvaluesstep] at Hvalues
        iframe
        iexists (done ++ [item]), todo, (pos + W64 item.bytes.length), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
      | some words =>
        have valid : FeltValid (d + 1) item.bytes (some words) := by rw [← hv]; exact item.valid
        obtain ⟨b, tail, hb, hmajor⟩ := feltValid_some_head (d + 1) item.bytes words valid
        have hbi : xs[(sint.Z pos).toNat]? = some b :=
          input_suffix_head xs (tail ++ (todo.flatMap FeltWire.bytes ++ rem)) (sint.nat pos) (sint.nat v.buf'.len) b
            (by simpa only [hb, List.cons_append] using hitem)
        have hn : b ≠ W8 246 := by
          intro h; have hc := hmajor; rw [h] at hc; exact (by decide : W8 246 &&& W8 224 ≠ W8 128) hc
        have hu : b ≠ W8 247 := by
          intro h; have hc := hmajor; rw [h] at hc; exact (by decide : W8 247 &&& W8 224 ≠ W8 128) hc
        wp_apply wp_null_other c {v with pos' := pos, depth' := v.depth' + W64 1} xs b dq Hpos.1 hbi hn hu
          $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        have hdataindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z data.len := by word
        rw [ite_eq_left_of_eq_true _ _ (eq_true hdataindex)]
        wp_auto
        have hvlen : (feltStorage (done.map FeltWire.value)).length = done.length := by simp [feltStorage]
        have hlookup :
            (feltStorage (done.map FeltWire.value) ++ List.replicate (item :: todo).length (zero_val (GoArray w64 4)))[(sint.Z (W64 done.length)).toNat]? =
              some (zero_val (GoArray w64 4)) := by
          rw [hidxnat, ← hvlen]
          simp
        wp_apply wp_limbs_slice_valid c data
          (feltStorage (done.map FeltWire.value) ++ List.replicate (item :: todo).length (zero_val (GoArray w64 4)))
          (sint.Z (W64 done.length)) (zero_val (GoArray w64 4)) words {v with pos' := pos, depth' := v.depth' + W64 1}
          xs item.bytes (todo.flatMap FeltWire.bytes ++ rem) dq (d + 1) hdataindex.1 hlookup
          (by word) (by omega) valid Hpos hitem $$ [Hvalues Hc Hs] with ⟨Hvalues, Hc, Hs, %Hstep⟩
        · iframe Hvalues Hc Hs
        rw [ite_eq_left_of_eq_true _ _ (eq_true hdataindex)]
        wp_auto
        rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
        wp_apply wp_store_slice_index (t := go.PointerType felt.Felt.ty) out (sint.Z (W64 done.length))
          (feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (item :: todo).length null)
          (sliceIndexRef (GoArray w64 4) (sint.Z (W64 done.length)) data) $$ [Hout] with Hout
        · iframe Hout
          ipureintro
          simp only [List.length_append, feltPointers_length, List.length_map, List.length_cons, List.length_replicate]
          constructor <;> word
        have hidx : sint.Z (W64 done.length) = done.length := by word
        have houtstep :
            (feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (item :: todo).length null).set done.length
              (sliceIndexRef (GoArray w64 4) (done.length : Int) data) =
            feltPointers (GF := GF) data ((done ++ [item]).map FeltWire.value) ++ List.replicate todo.length null := by
          simpa only [List.length_cons, hpl, List.map_append, List.map_singleton, feltPointers_step,
            List.length_map, feltPointer, hv] using
            initialized_prefix_step (feltPointers (GF := GF) data (done.map FeltWire.value)) todo.length null
              (sliceIndexRef (GoArray w64 4) (done.length : Int) data)
        have hvaluesstep :
            (feltStorage (done.map FeltWire.value) ++ List.replicate (item :: todo).length (zero_val (GoArray w64 4))).set done.length words =
            feltStorage ((done ++ [item]).map FeltWire.value) ++ List.replicate todo.length (zero_val (GoArray w64 4)) := by
          simpa [feltStorage, hv] using
            initialized_prefix_step (feltStorage (done.map FeltWire.value)) todo.length (zero_val (GoArray w64 4)) words
        wp_for_post
        isimp only [hinc] at Hrange
        isimp only [hidxnat, hidx, Int.toNat_natCast, houtstep] at Hout
        isimp only [hidxnat, hvaluesstep] at Hvalues
        iframe
        iexists (done ++ [item]), todo, (pos + W64 item.bytes.length), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
  · have hcond : ¬ sint.Z (W64 done.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    have hdoneall : done = items := by simpa [htodo] using Hsplit.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length) := by
      rw [hdoneall] at Hposition; word
    wp_apply wp_leave c {v with pos' := pos} $$ [Hc] with Hc
    · iframe Hc
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hposfinal, hrestoredepth] at Hc
    isimp only [hdoneall, htodo, List.length_nil, List.replicate_zero, List.append_nil] at Hout Hvalues
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Hout Hvalues]
    · unfold FeltPointersOwned
      iexists data
      iframe Hout Hvalues
      ipureintro; exact ⟨Houtnn, Hdatann⟩
    · ipureintro
      simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1, Int.add_assoc] using And.intro Hnext.1 Hnext.2.2

/-- The actual pointer-slice reader realizes the independent nullable-array
    contract, including nil versus allocated empty, owned backing values and
    pointer table, restored nesting depth, unchanged input and exact suffix. -/
theorem wp_readFeltPointers_valid (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire rem : List w8) (value : Option (List (Option (GoArray w64 4)))) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : FeltPointersValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ FeltPointersResult out value ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
  cases valid with
  | @nullish head hn =>
    have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_readFeltPointers_null_at c dst old v xs head dq hp.1 hi hn $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    simp only [List.length_singleton, Int.natCast_one] at Hnext
    iapply HΦ
    isimp only [FeltPointersResult]
    iframe Hdst Hc Hs
    ipureintro; exact ⟨rfl, Hnext⟩
  | @array head payload count items enc hm size bound =>
    have heq : count = W64 items.length := by word
    rw [heq] at enc
    wp_apply wp_readFeltPointers_array c dst old v xs payload rem head dq d items enc hm hd bound hp hbytes
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hout⟩
    · iframe Hdst Hc Hs
    isimp only [FeltPointersResult, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] at HΦ
    iapply HΦ
    iframe Hdst Howned Hc Hs
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] using Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltPointers_array

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltPointers_valid
