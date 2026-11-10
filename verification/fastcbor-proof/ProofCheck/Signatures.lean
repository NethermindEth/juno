import ProofCheck.SignatureSlot

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

/-- The actual outer signature reader allocates its header array and decodes
    arbitrary nullable children, retaining nested ownership and the exact suffix. -/
theorem wp_readSignatures_array (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac) (d : Nat) (items : List (SignatureWire (d + 1)))
    (enc : HeadArgument head payload (W64 items.length)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ items.flatMap SignatureWire.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ SignaturesOwned out (items.map SignatureWire.value) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length)} ∗
       v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length)) =
           sint.Z v.pos' + 1 + payload.length + (items.flatMap SignatureWire.bytes).length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ items.flatMap SignatureWire.bytes) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + (items.flatMap SignatureWire.bytes).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hitems := signatureWire_count_bound (d + 1) items
  have hfit : (items.length : Int) < 2 ^ 63 := by word
  have hcount : 0 ≤ sint.Z (W64 items.length) := by word
  have hi := input_suffix_head xs (payload ++ items.flatMap SignatureWire.bytes ++ rem)
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
      payload ++ (items.flatMap SignatureWire.bytes ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_ArrayLen_encoded c v xs payload (items.flatMap SignatureWire.bytes ++ rem) head (W64 items.length) dq
    enc hm hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_slice_make2_nonnil (V := GoSlice) (W64 items.length) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  have hnat : sint.nat (W64 items.length) = items.length := by word
  isimp only [hnat] at Hout
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  have houtlen : sint.Z out.len = items.length := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  ihave Hchildren := signatureChildren_nil (GF := GF)
  have Hhead := input_consumption xs (head :: payload) (items.flatMap SignatureWire.bytes ++ rem)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      items.flatMap SignatureWire.bytes ++ rem := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (SignatureWire (d + 1))) (slices : List GoSlice) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hout" ∷ out ↦* (slices ++ List.replicate todo.length slice.nil) ∗
      "Hchildren" ∷ SignatureChildren slices (done.map SignatureWire.value) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap SignatureWire.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap SignatureWire.bytes ++ rem⌝ : IProp GF)
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
      have hlookup : (slices ++ List.replicate (item :: todo).length slice.nil)[(sint.Z (W64 done.length)).toNat]? = some slice.nil := by
        rw [hidxnat, ← hslices]
        simp
      wp_apply wp_load_slice_index (t := go.SliceType (go.PointerType felt.Felt.ty)) out (sint.Z (W64 done.length))
        (slices ++ List.replicate (item :: todo).length slice.nil)
        (DFrac.own 1) slice.nil hindex.1 $$ [Hout] with Hout
      · iframe Hout
        ipureintro; exact hlookup
      have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item.bytes ++ (todo.flatMap SignatureWire.bytes ++ rem) := by
        simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
      rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
      wp_auto
      wp_apply wp_readFeltPointers_slice_valid c out (slices ++ List.replicate (item :: todo).length slice.nil)
        (sint.Z (W64 done.length)) slice.nil item.value {v with pos' := pos, depth' := v.depth' + W64 1}
        xs item.bytes (todo.flatMap SignatureWire.bytes ++ rem) dq (d + 1) hindex.1 hlookup
        (by word) (by omega) item.valid Hpos hitem $$ [Hout Hc Hs] as %child ⟨Hout, Hchild, Hc, Hs, %Hadvance⟩
      · iframe Hout Hc Hs
      dsimp only at Hadvance
      have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
        simp only [List.length_append, List.length_singleton]; word
      have hnewlen : ((done ++ [item]).flatMap SignatureWire.bytes).length = (done.flatMap SignatureWire.bytes).length + item.bytes.length := by simp
      have houtstep : (slices ++ List.replicate (item :: todo).length slice.nil).set done.length child =
          (slices ++ [child]) ++ List.replicate todo.length slice.nil := by
        simpa only [List.length_cons, hslices] using initialized_prefix_step slices todo.length slice.nil child
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
  · have hcond : ¬ sint.Z (W64 done.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    have hdoneall : done = items := by simpa [htodo] using Hsplit.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (items.flatMap SignatureWire.bytes).length) := by
      rw [hdoneall] at Hposition; word
    wp_apply wp_leave c {v with pos' := pos} $$ [Hc] with Hc
    · iframe Hc
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hposfinal, hrestoredepth] at Hc
    isimp only [htodo, List.length_nil, List.replicate_zero, List.append_nil] at Hout
    isimp only [hdoneall] at Hchildren
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Hout Hchildren]
    · unfold SignaturesOwned
      iexists slices
      iframe Hout Hchildren
      ipureintro; exact Houtnn
    · ipureintro
      simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1, Int.add_assoc] using And.intro Hnext.1 Hnext.2.2

/-- The actual signature reader realizes the independent nullable nested-array
    contract, including nil versus allocated empty at both slice levels, owned
    nested values, restored nesting depth, unchanged input and exact suffix. -/
theorem wp_readSignatures_valid (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire rem : List w8) (value : Option (List SignatureValue)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (_hdepth : d ≤ 32) (valid : SignaturesValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ SignaturesResult out value ∗
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
    wp_apply wp_readSignatures_null_at c dst old v xs head dq hp.1 hi hn $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    simp only [List.length_singleton, Int.natCast_one] at Hnext
    iapply HΦ
    isimp only [SignaturesResult]
    iframe Hdst Hc Hs
    ipureintro; exact ⟨rfl, Hnext⟩
  | @array head payload count items enc hm size bound =>
    have heq : count = W64 items.length := by word
    rw [heq] at enc
    wp_apply wp_readSignatures_array c dst old v xs payload rem head dq d items enc hm hd bound hp hbytes
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hout⟩
    · iframe Hdst Hc Hs
    isimp only [SignaturesResult, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] at HΦ
    iapply HΦ
    iframe Hdst Howned Hc Hs
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] using Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSignatures_array

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSignatures_valid
