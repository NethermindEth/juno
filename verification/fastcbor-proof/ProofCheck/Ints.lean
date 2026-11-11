import ProofCheck.IntsSpec
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

/-- Actual signed arrays preserve every element, owned storage, nesting depth
    and the exact suffix, including null elements and nonpreferred widths. -/
theorem wp_readInts_array (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac) (d : Nat) (items : List IntElementWire)
    (enc : HeadArgument head payload (W64 items.length)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ items.flatMap IntElementWire.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadInts)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ out ↦* (items.map IntElementWire.value) ∗ ⌜out.ptr ≠ null⌝ ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length)} ∗
       v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length)) =
           sint.Z v.pos' + 1 + payload.length + (items.flatMap IntElementWire.bytes).length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%k, #Hglobals⟩
  isimp only [BigIntErrorGlobals] at Hglobals
  icases Hglobals with ⟨_, _, #Htype, _, _⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ items.flatMap IntElementWire.bytes) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + (items.flatMap IntElementWire.bytes).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hitems := intElementWire_count_bound items
  have hfit : (items.length : Int) < 2 ^ 63 := by word
  have hcount : 0 ≤ sint.Z (W64 items.length) := by word
  have hi := input_suffix_head xs (payload ++ items.flatMap IntElementWire.bytes ++ rem)
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
      payload ++ (items.flatMap IntElementWire.bytes ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_ArrayLen_encoded c v xs payload (items.flatMap IntElementWire.bytes ++ rem) head (W64 items.length) dq
    enc hm hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_slice_make2_nonnil (V := w64) (W64 items.length) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  have hnat : sint.nat (W64 items.length) = items.length := by word
  isimp only [hnat] at Hout
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  have houtlen : sint.Z out.len = items.length := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  have Hhead := input_consumption xs (head :: payload) (items.flatMap IntElementWire.bytes ++ rem)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      items.flatMap IntElementWire.bytes ++ rem := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (IntElementWire)) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hout" ∷ out ↦* (done.map IntElementWire.value ++ List.replicate todo.length (W64 0)) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap IntElementWire.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap IntElementWire.bytes ++ rem⌝ : IProp GF)
      $$ [Hrange Hi Hout Hc]
  · iexists [], items, (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.map_nil, List.nil_append]
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
      have hidxnat : (sint.Z (W64 done.length)).toNat = done.length := by word
      have hlookup : (done.map IntElementWire.value ++ List.replicate (item :: todo).length (W64 0))[(sint.Z (W64 done.length)).toNat]? = some (W64 0) := by
        have hmaplen : (done.map IntElementWire.value).length = done.length := List.length_map _
        rw [hidxnat, ← hmaplen]
        simp
      wp_apply wp_load_slice_index (t := go.int) out (sint.Z (W64 done.length))
        (done.map IntElementWire.value ++ List.replicate (item :: todo).length (W64 0))
        (DFrac.own 1) (W64 0) hindex.1 $$ [Hout] with Hout
      · iframe Hout; ipureintro; exact hlookup
      have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item.bytes ++ (todo.flatMap IntElementWire.bytes ++ rem) := by
        simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
      have Hadvance := input_consumption xs item.bytes (todo.flatMap IntElementWire.bytes ++ rem) pos v.buf'.len Hpos Hlen.1 hitem
      have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
        simp only [List.length_append, List.length_singleton]; word
      have hnewlen : ((done ++ [item]).flatMap IntElementWire.bytes).length = (done.flatMap IntElementWire.bytes).length + item.bytes.length := by simp
      have houtstep : (done.map IntElementWire.value ++ List.replicate (item :: todo).length (W64 0)).set done.length item.value =
          (done ++ [item]).map IntElementWire.value ++ List.replicate todo.length (W64 0) := by
        simpa only [List.length_cons, List.length_map, List.map_append, List.map_singleton] using
          initialized_prefix_step (done.map IntElementWire.value) todo.length (W64 0) item.value
      cases item with
      | nullish b hn =>
        have hbi := input_suffix_head xs _ (sint.nat pos) (sint.nat v.buf'.len) b hitem
        have enc : HeadArgument b [] (W64 22) ∨ HeadArgument b [] (W64 23) := by
          rcases hn with rfl | rfl
          · exact Or.inl (.inline _ (by decide))
          · exact Or.inr (.inline _ (by decide))
        have hu : b &&& W8 224 ≠ W8 0 := by rcases hn with rfl | rfl <;> decide
        have ht : b &&& W8 224 ≠ W8 192 := by rcases hn with rfl | rfl <;> decide
        have hbody : subslice (sint.nat (pos + W64 1)) (sint.nat v.buf'.len) xs = [] ++ (todo.flatMap IntElementWire.bytes ++ rem) := by
          simpa only [IntElementWire.bytes, List.length_singleton, Int.natCast_one, List.nil_append] using Hadvance.2.2
        have hbound : sint.Z pos + 1 + ([] : List w8).length ≤ sint.Z v.buf'.len := by
          simp only [IntElementWire.bytes, List.length_singleton, Int.natCast_one] at Hadvance
          simpa using Hadvance.1 ▸ Hadvance.2.1
        rcases enc with enc | enc
        all_goals
          wp_apply wp_uint64_type c {v with pos' := pos, depth' := v.depth' + W64 1} xs []
            (todo.flatMap IntElementWire.bytes ++ rem) b _ dq (bigIntCategory k.typeError)
            enc hu ht Hpos.1 hbound hbi hbody $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
          · iframe Htype Hc Hs
          wp_apply wp_null_at c {v with pos' := pos, depth' := v.depth' + W64 1} xs b dq Hpos.1 hbi hn
            $$ [Hc Hs] with ⟨Hc, Hs⟩
          · iframe Hc Hs
          have houtnull : done.map IntElementWire.value ++ List.replicate (IntElementWire.nullish b hn :: todo).length (W64 0) =
              (done ++ [IntElementWire.nullish b hn]).map IntElementWire.value ++ List.replicate todo.length (W64 0) := by
            simp [IntElementWire.value, List.replicate_succ, List.append_assoc]
          isimp only [houtnull] at Hout
          wp_for_post
          isimp only [hinc] at Hrange
          iframe
          iexists (done ++ [IntElementWire.nullish b hn]), todo, (pos + W64 (IntElementWire.nullish b hn).bytes.length), (W64 done.length)
          isimp only [IntElementWire.bytes, List.length_cons, List.length_singleton, Int.natCast_add, Int.natCast_one] at Hc
          iframe
          ipureintro
          refine ⟨?_, ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · rw [Hadvance.1]; omega
          · rw [Hadvance.1, hnewlen]; omega
      | unsigned b body argument enc major bound =>
        have hbi := input_suffix_head xs _ (sint.nat pos) (sint.nat v.buf'.len) b hitem
        wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs
          (todo.flatMap IntElementWire.bytes ++ rem) ⟨b, body, argument, major, enc⟩ dq Hpos hitem
          $$ [Hc Hs] with ⟨Hc, Hs, %Hstep⟩
        · iframe Hc Hs
        have hgood : ¬ uint.Z (W64 9223372036854775807) < uint.Z argument := by
          change ¬ (9223372036854775807 : Int) < uint.Z argument
          omega
        simp only [hgood, _root_.decide_false]
        wp_auto
        rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
        wp_apply wp_store_slice_index (t := go.int) out (sint.Z (W64 done.length))
          (done.map IntElementWire.value ++ List.replicate (IntElementWire.unsigned b body argument enc major bound :: todo).length (W64 0))
          argument $$ [Hout] with Hout
        · iframe Hout
          ipureintro
          simp only [List.length_append, List.length_map, List.length_cons, List.length_replicate]
          constructor <;> word
        simp only [IntElementWire.value] at houtstep
        isimp only [hidxnat, houtstep] at Hout
        wp_for_post
        isimp only [hinc] at Hrange
        iframe
        iexists (done ++ [IntElementWire.unsigned b body argument enc major bound]), todo, (pos + W64 (IntElementWire.unsigned b body argument enc major bound).bytes.length), (W64 done.length)
        have hsize : ((IntElementWire.unsigned b body argument enc major bound).bytes.length : Int) = 1 + body.length := by
          simp only [IntElementWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one]; omega
        isimp only [UnsignedWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one,
          Int.add_comm (body.length : Int) 1] at Hc
        isimp only [← hsize] at Hc
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
      | negative b body argument enc major bound =>
        have hbi := input_suffix_head xs _ (sint.nat pos) (sint.nat v.buf'.len) b hitem
        have hu : b &&& W8 224 ≠ W8 0 := by rw [major]; decide
        have ht : b &&& W8 224 ≠ W8 192 := by rw [major]; decide
        have hbound : sint.Z pos + 1 + body.length ≤ sint.Z v.buf'.len := by
          simp only [IntElementWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one] at Hadvance
          omega
        have hbody : subslice (sint.nat (pos + W64 1)) (sint.nat v.buf'.len) xs = body ++ (todo.flatMap IntElementWire.bytes ++ rem) := by
          have hp1 : sint.nat (pos + W64 1) = sint.nat pos + 1 := by word
          rw [hp1]
          exact input_suffix_after xs [b] _ _ _ hitem
        wp_apply wp_uint64_type c {v with pos' := pos, depth' := v.depth' + W64 1} xs body
          (todo.flatMap IntElementWire.bytes ++ rem) b argument dq (bigIntCategory k.typeError)
          enc hu ht Hpos.1 hbound hbi hbody $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
        · iframe Htype Hc Hs
        have hn : b ≠ W8 246 := by
          intro h; rw [h] at major
          exact (by decide : W8 246 &&& W8 224 ≠ W8 32) major
        have hundef : b ≠ W8 247 := by
          intro h; rw [h] at major
          exact (by decide : W8 247 &&& W8 224 ≠ W8 32) major
        wp_apply wp_null_other c {v with pos' := pos, depth' := v.depth' + W64 1} xs b dq Hpos.1 hbi hn hundef
          $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        wp_apply wp_int64_encoded c {v with pos' := pos, depth' := v.depth' + W64 1} xs body
          (todo.flatMap IntElementWire.bytes ++ rem) b argument dq enc (Or.inr major) bound Hpos hitem
          $$ [Hc Hs] with ⟨Hc, Hs, %Hstep⟩
        · iframe Hc Hs
        simp only [major, ite_true]
        rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
        wp_apply wp_store_slice_index (t := go.int) out (sint.Z (W64 done.length))
          (done.map IntElementWire.value ++ List.replicate (IntElementWire.negative b body argument enc major bound :: todo).length (W64 0))
          (~~~argument) $$ [Hout] with Hout
        · iframe Hout
          ipureintro
          simp only [List.length_append, List.length_map, List.length_cons, List.length_replicate]
          constructor <;> word
        simp only [IntElementWire.value] at houtstep
        isimp only [hidxnat, houtstep] at Hout
        wp_for_post
        isimp only [hinc] at Hrange
        iframe
        iexists (done ++ [IntElementWire.negative b body argument enc major bound]), todo, (pos + W64 (IntElementWire.negative b body argument enc major bound).bytes.length), (W64 done.length)
        have hsize : ((IntElementWire.negative b body argument enc major bound).bytes.length : Int) = 1 + body.length := by
          simp only [IntElementWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one]; omega
        isimp only [← hsize] at Hc
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
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length) := by
      rw [hdoneall] at Hposition; word
    wp_apply wp_leave c {v with pos' := pos} $$ [Hc] with Hc
    · iframe Hc
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hposfinal, hrestoredepth] at Hc
    isimp only [hdoneall, htodo, List.length_nil, List.replicate_zero, List.append_nil] at Hout
    iapply HΦ
    iframe Hdst Hc Hs
    iframe Hout
    ipureintro
    refine ⟨Houtnn, ?_⟩
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1, Int.add_assoc] using And.intro Hnext.1 Hnext.2.2

theorem wp_readInts_null_at (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hn : b = W8 246 ∨ b = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadInts)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ slice.nil ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs b dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The actual signed-array reader realizes the independent nullable-array
    contract, including nil versus allocated empty, owned
    element storage, restored nesting depth, unchanged input and exact suffix. -/
theorem wp_readInts_valid (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire rem : List w8) (value : Option (List Int)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (_hdepth : d ≤ 32) (valid : IntsValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadInts)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ IntsResult out value ∗
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
    wp_apply wp_readInts_null_at c dst old v xs head dq hp.1 hi hn $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    simp only [List.length_singleton, Int.natCast_one] at Hnext
    iapply HΦ
    isimp only [IntsResult]
    iframe Hdst Hc Hs
    ipureintro; exact ⟨rfl, Hnext⟩
  | @array head payload count items enc hm size bound =>
    have heq : count = W64 items.length := by word
    rw [heq] at enc
    wp_apply wp_readInts_array c dst old v xs payload rem head dq d items enc hm hd bound hp hbytes
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, %Hnn, Hc, Hs, %Hout⟩
    · iframe Hdst Hc Hs
    isimp only [IntsResult, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Howned]
    · iexists (items.map IntElementWire.value)
      iframe Howned
      ipureintro
      exact ⟨Hnn, by simp only [List.map_map]; exact List.map_congr_left (fun x _ => intElementWire_value x)⟩
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] using Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readInts_array
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readInts_null_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readInts_valid
