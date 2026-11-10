import ProofCheck.NullableFailure

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

/-- A failed child after any valid nullable prefix does not publish the partial
    allocation. The cursor retains the entered outer array and points at the
    failed child; Limbs supplies the exact owned diagnostic. -/
theorem wp_readFeltPointers_prefix_failure (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (dq : DFrac) (d count offset : Nat) (items : List (FeltWire (d + 1)))
    (reason : FeltFailureReason) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (more : items.length < count)
    (fit : count ≤ (items.flatMap FeltWire.bytes ++ wire).length)
    (failure : FeltFailure (d + 1) wire offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ items.flatMap FeltWire.bytes ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       FeltFailureError err
         (v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length) + W64 offset) reason kind }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ items.flatMap FeltWire.bytes) wire v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + (items.flatMap FeltWire.bytes).length ≤ sint.Z v.buf'.len := by
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
  have hi := input_suffix_head xs (payload ++ items.flatMap FeltWire.bytes ++ wire)
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
      payload ++ (items.flatMap FeltWire.bytes ++ wire) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_ArrayLen_encoded c v xs payload (items.flatMap FeltWire.bytes ++ wire) head (W64 count) dq
    enc hm hp.1 (by omega) (by omega) hcountfit hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_slice_make2_nonnil (V := Loc) (W64 count) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  wp_apply wp_slice_make2_nonnil (V := GoArray w64 4) (W64 count) as %data ⟨Hvalues, -, %Hdatann⟩
  · ipureintro; exact hcount
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  have hnat : sint.nat (W64 count) = count := by word
  isimp only [hnat] at Hout Hvalues
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  ihave %Hdatasize := ownSlice_len _ _ _ $$ Hvalues
  have houtlen : sint.Z out.len = count := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  have hdatalen : sint.Z data.len = count := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Hdatasize ⊢; omega
  have Hhead := input_consumption xs (head :: payload) (items.flatMap FeltWire.bytes ++ wire)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      items.flatMap FeltWire.bytes ++ wire := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (FeltWire (d + 1))) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hout" ∷ out ↦* (feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (count - done.length) null) ∗
      "Hvalues" ∷ data ↦* (feltStorage (done.map FeltWire.value) ++ List.replicate (count - done.length) (zero_val (GoArray w64 4))) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap FeltWire.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap FeltWire.bytes ++ wire⌝ : IProp GF)
      $$ [Hrange Hi Hout Hvalues Hc]
  · iexists [], items, (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.map_nil, feltPointers, List.mapIdx_nil, feltStorage, List.nil_append]
    iframe
    ipureintro
    exact ⟨trivial, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hdone : done.length ≤ items.length := by omega
  have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  have hleft : 0 < count - done.length := by omega
  have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  have hpl : (feltPointers (GF := GF) data (done.map FeltWire.value)).length = done.length := by simp [feltPointers]
  have hidxnat : (sint.Z (W64 done.length)).toNat = done.length := by word
  wp_apply wp_load_slice_index (t := go.PointerType felt.Felt.ty) out (sint.Z (W64 done.length))
    (feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (count - done.length) null)
    (DFrac.own 1) null hindex.1 $$ [Hout] with Hout
  · iframe Hout
    ipureintro
    rw [hidxnat, ← hpl]
    exact uninitialized_prefix_lookup _ _ _ (by simpa [feltPointers, feltStorage] using hleft)
  cases todo with
  | nil =>
    have hdoneall : done = items := by simpa using Hsplit.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (items.flatMap FeltWire.bytes).length) := by
      rw [hdoneall] at Hposition; word
    have hwire : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = wire := by simpa using Hremaining
    have hn := feltFailure_not_nullish (d + 1) offset wire reason failure
    wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1}
      xs wire dq hn.1 hn.2 Hpos hwire $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    have hdataindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z data.len := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true hdataindex)]
    wp_auto
    have hvlen : (feltStorage (done.map FeltWire.value)).length = done.length := by simp [feltStorage]
    have hlookup :
        (feltStorage (done.map FeltWire.value) ++ List.replicate (count - done.length) (zero_val (GoArray w64 4)))[(sint.Z (W64 done.length)).toNat]? =
          some (zero_val (GoArray w64 4)) := by
      rw [hidxnat, ← hvlen]
      exact uninitialized_prefix_lookup _ _ _ (by simpa [feltPointers, feltStorage] using hleft)
    wp_apply wp_limbs_slice_failure c data
      (feltStorage (done.map FeltWire.value) ++ List.replicate (count - done.length) (zero_val (GoArray w64 4)))
      (sint.Z (W64 done.length)) (zero_val (GoArray w64 4)) {v with pos' := pos, depth' := v.depth' + W64 1}
      xs wire dq (d + 1) offset reason kind hdataindex.1 hlookup (LimbsFailure.typed failure)
      (by word) (by omega) Hpos hwire $$ [Hvalues Hc Hs] as %err ⟨Hvalues, Hc, Hs, Herr⟩
    · iframe Hkind Hvalues Hc Hs
    wp_for_post
    isimp only [hposfinal] at Hc Herr
    iapply HΦ
    iframe Hdst Hc Hs Herr
  | cons item todo =>
    have hstepcount : count - done.length = (count - (done ++ [item]).length) + 1 := by
      simp only [List.length_cons] at hlength
      simp only [List.length_append, List.length_singleton]
      omega
    have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item.bytes ++ (todo.flatMap FeltWire.bytes ++ wire) := by
      simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
    have Hadvance := input_consumption xs item.bytes (todo.flatMap FeltWire.bytes ++ wire) pos v.buf'.len Hpos Hlen.1 hitem
    have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
      simp only [List.length_append, List.length_singleton]; word
    have hnewlen : ((done ++ [item]).flatMap FeltWire.bytes).length = (done.flatMap FeltWire.bytes).length + item.bytes.length := by simp
    cases hv : item.value with
    | none =>
      obtain ⟨b, hb, hnull⟩ := feltValid_none (d + 1) item.bytes (by rw [← hv]; exact item.valid)
      have hbi : xs[(sint.Z pos).toNat]? = some b :=
        input_suffix_head xs (todo.flatMap FeltWire.bytes ++ wire) (sint.nat pos) (sint.nat v.buf'.len) b
          (by simpa only [hb, List.singleton_append] using hitem)
      wp_apply wp_null_at c {v with pos' := pos, depth' := v.depth' + W64 1} xs b dq Hpos.1 hbi hnull
        $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      have hposstep : pos + W64 1 = pos + W64 item.bytes.length := by rw [hb]; rfl
      have houtstep : feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (count - done.length) null =
          feltPointers (GF := GF) data ((done ++ [item]).map FeltWire.value) ++ List.replicate (count - (done ++ [item]).length) null := by
        rw [hstepcount]
        simp [List.map_append, feltPointers_step, feltPointer, hv, List.replicate_succ, List.append_assoc]
      have hvaluesstep : feltStorage (done.map FeltWire.value) ++ List.replicate (count - done.length) (zero_val (GoArray w64 4)) =
          feltStorage ((done ++ [item]).map FeltWire.value) ++ List.replicate (count - (done ++ [item]).length) (zero_val (GoArray w64 4)) := by
        rw [hstepcount]
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
        input_suffix_head xs (tail ++ (todo.flatMap FeltWire.bytes ++ wire)) (sint.nat pos) (sint.nat v.buf'.len) b
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
          (feltStorage (done.map FeltWire.value) ++ List.replicate (count - done.length) (zero_val (GoArray w64 4)))[(sint.Z (W64 done.length)).toNat]? =
            some (zero_val (GoArray w64 4)) := by
        rw [hidxnat, ← hvlen]
        exact uninitialized_prefix_lookup _ _ _ (by simpa [feltPointers, feltStorage] using hleft)
      wp_apply wp_limbs_slice_valid c data
        (feltStorage (done.map FeltWire.value) ++ List.replicate (count - done.length) (zero_val (GoArray w64 4)))
        (sint.Z (W64 done.length)) (zero_val (GoArray w64 4)) words {v with pos' := pos, depth' := v.depth' + W64 1}
        xs item.bytes (todo.flatMap FeltWire.bytes ++ wire) dq (d + 1) hdataindex.1 hlookup
        (by word) (by omega) valid Hpos hitem $$ [Hvalues Hc Hs] with ⟨Hvalues, Hc, Hs, %Hstep⟩
      · iframe Hvalues Hc Hs
      rw [ite_eq_left_of_eq_true _ _ (eq_true hdataindex)]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
      wp_apply wp_store_slice_index (t := go.PointerType felt.Felt.ty) out (sint.Z (W64 done.length))
        (feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (count - done.length) null)
        (sliceIndexRef (GoArray w64 4) (sint.Z (W64 done.length)) data) $$ [Hout] with Hout
      · iframe Hout
        ipureintro
        simp only [List.length_append, feltPointers_length, List.length_map, List.length_replicate]
        constructor <;> word
      have hidx : sint.Z (W64 done.length) = done.length := by word
      have houtstep :
          (feltPointers (GF := GF) data (done.map FeltWire.value) ++ List.replicate (count - done.length) null).set done.length
            (sliceIndexRef (GoArray w64 4) (done.length : Int) data) =
          feltPointers (GF := GF) data ((done ++ [item]).map FeltWire.value) ++ List.replicate (count - (done ++ [item]).length) null := by
        simpa only [hstepcount, hpl, List.map_append, List.map_singleton, feltPointers_step,
          List.length_map, feltPointer, hv] using
          initialized_prefix_step (feltPointers (GF := GF) data (done.map FeltWire.value)) (count - (done ++ [item]).length) null
            (sliceIndexRef (GoArray w64 4) (done.length : Int) data)
      have hvaluesstep :
          (feltStorage (done.map FeltWire.value) ++ List.replicate (count - done.length) (zero_val (GoArray w64 4))).set done.length words =
          feltStorage ((done ++ [item]).map FeltWire.value) ++ List.replicate (count - (done ++ [item]).length) (zero_val (GoArray w64 4)) := by
        simpa [hstepcount, feltStorage, hv] using
          initialized_prefix_step (feltStorage (done.map FeltWire.value)) (count - (done ++ [item]).length) (zero_val (GoArray w64 4)) words
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

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltPointers_prefix_failure
