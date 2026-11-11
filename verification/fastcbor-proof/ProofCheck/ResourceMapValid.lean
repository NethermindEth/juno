import ProofCheck.ResourceMapBoundary
import ProofCheck.ResourceBoundsRecordComplete
import ProofCheck.ResourceMapEntriesSpec

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

/-- The actual map loop allocates and decodes every entry from zero, replaces
    duplicate keys in wire order and publishes only the complete map. -/
theorem wp_readResourceMap_map (validator : Utf8ValidatorContract (GF := GF)) (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (dq : DFrac) (d : Nat) (count : Nat) (body : List w8) (values : List (w32 × ResourceBoundsValue))
    (valid : ResourceMapEntriesValid (d + 1) count body rem values)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadResourceBoundsMap)) (Val #c)) (Val #dst))
    {{ (out : Loc), RET #interface.nil; dst ↦ out ∗ ResourceMapOwned out (resourceMapOfEntries values) ∗
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
  have hitems := resourceMapEntries_count (d + 1) count body rem values valid
  have hfit : (count : Int) < 2 ^ 63 := by word
  have hcount : 0 ≤ sint.Z (W64 count) := by word
  have hi := input_suffix_head xs (payload ++ body ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by
    intro h
    have hc := hm
    rw [h] at hc
    exact (by decide : W8 246 &&& W8 224 ≠ W8 160) hc
  have hu : head ≠ W8 247 := by
    intro h
    have hc := hm
    rw [h] at hc
    exact (by decide : W8 247 &&& W8 224 ≠ W8 160) hc
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (body ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_encoded c v xs payload (body ++ rem) head (W64 count) dq
    enc hm hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply (wp_map_make2 (K := w32) (V := ResourceBounds) (W64 count)) with %out Hmap
  irename count => Hcount
  ihave Hchildren : (ResourceMapChildren (∅ : GMap w32 ResourceBounds) ∅ : IProp GF) $$ []
  · iapply resourceMapChildren_empty
  have Hhead := input_consumption xs (head :: payload) (body ++ rem)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body ++ rem := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (w32 × ResourceBoundsValue)) (stored : GMap w32 ResourceBounds)
      (front tail : List w8) (pos : w64),
      "Hcount" ∷ count_ptr ↦ W64 todo.length ∗
      "Hmap" ∷ out ↦$ stored ∗
      "Hchildren" ∷ ResourceMapChildren stored (resourceMapOfEntries done) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜values = done ++ todo⌝ ∗
      "%Hwire" ∷ ⌜body = front ++ tail⌝ ∗
      "%Hvalid" ∷ ⌜ResourceMapEntriesValid (d + 1) todo.length tail rem todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + front.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = tail ++ rem⌝ : IProp GF)
      $$ [Hcount Hmap Hchildren Hc]
  · iexists [], values, ∅, [], body, (v.pos' + W64 (1 + payload.length))
    isimp only [List.length_nil, List.nil_append, hitems.1, resourceMapOfEntries, resourceMapApply, List.foldl_nil]
    iframe
    ipureintro
    exact ⟨trivial, trivial, valid, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have hlength : count = done.length + todo.length := by rw [← hitems.1, Hsplit, List.length_append]
  by_cases more : 0 < todo.length
  · have hcond : sint.Z (W64 0) < sint.Z (W64 todo.length) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    generalize htodoCount : todo.length = todoCount at Hvalid
    cases Hvalid with
    | nil => simp only [List.length_nil] at more; omega
    | @step n item tail suffix entry todo first rest =>
      have hrestcount := (resourceMapEntries_count (d + 1) n tail rem todo rest).1
      have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ (tail ++ rem) := by
        simpa only [List.append_assoc] using Hremaining
      have Hadvance := input_consumption xs item (tail ++ rem) pos v.buf'.len Hpos Hlen.1 hitem
      cases first with
      | @pair keyItem valueItem suffix key value keyValid record =>
        have hn := resourceMapKeyValid_not_nullish keyItem (valueItem ++ (tail ++ rem)) key keyValid
        have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = keyItem ++ (valueItem ++ (tail ++ rem)) := by
          simpa only [List.append_assoc] using hitem
        wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1}
          xs (keyItem ++ (valueItem ++ (tail ++ rem))) dq hn.1 hn.2 Hpos hkey $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        cases keyValid with
        | @unsigned keyHead keyPayload argument keyEnc keyMajor fits =>
          wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs
            (valueItem ++ (tail ++ rem)) ⟨keyHead, keyPayload, argument, keyMajor, keyEnc⟩ dq Hpos hkey
            $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
          · iframe Hc Hs
          have hgood : ¬ uint.Z (W64 4294967295) < uint.Z argument := by change ¬ (4294967295 : Int) < uint.Z argument; omega
          simp only [hgood, _root_.decide_false]
          wp_auto
          isimp only [UnsignedWire.bytes] at Hc
          ihave Howned := resourceBoundsOwned_zero value_ptr $$ value
          wp_apply wp_readResourceBounds_record_valid validator c value_ptr resourceBoundsZero value
            {v with pos' := pos + W64 (keyHead :: keyPayload).length, depth' := v.depth' + W64 1}
            xs valueItem (tail ++ rem) dq (d + 1) record (by word)
            ⟨by simp only [UnsignedWire.bytes] at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
            $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
          · iframe Howned Hc Hs
          icases resourceMapValue_take value_ptr value $$ Howned with ⟨%newStored, Hvalue, Hchild⟩
          wp_auto
          wp_apply wp_mapInsert $$ Hmap with Hmap
          ihave Hchildren := resourceMapChildren_insert stored (resourceMapOfEntries done)
            (W32 (uint.Z argument)) newStored value $$ [Hchildren Hchild]
          · iframe Hchildren Hchild
          wp_for_post
          have hdec : W64 ((n + 1 : Nat) : Int) - W64 1 = W64 todo.length := by word
          isimp only [hdec] at Hcount
          isimp only [← resourceMapOfEntries_append_one] at Hchildren
          iframe
          iexists (done ++ [(W32 (uint.Z argument), value)]), todo,
            (<[W32 (uint.Z argument) := newStored]> stored),
            (front ++ ((keyHead :: keyPayload) ++ valueItem)), tail,
            (pos + W64 (keyHead :: keyPayload).length + W64 valueItem.length)
          iframe
          ipureintro
          refine ⟨?_, ?_, ?_, ⟨?_, Hstep.2.1⟩, ?_, Hstep.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa only [List.append_assoc] using Hwire
          · simpa only [hrestcount] using rest
          · simp only at Hstep; rw [Hstep.1]; simp only [UnsignedWire.bytes] at Hkey; rw [Hkey.1]; omega
          · simp only [List.length_append, Int.natCast_add]
            simp only at Hstep; rw [Hstep.1]; simp only [UnsignedWire.bytes] at Hkey; rw [Hkey.1]; omega
  · have hcond : ¬ sint.Z (W64 0) < sint.Z (W64 todo.length) := by word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    have hdoneall : done = values := by simpa [htodo] using Hsplit.symm
    have htail : tail = [] := by rw [htodo] at Hvalid; cases Hvalid; rfl
    have hfront : front = body := by simpa [htail] using Hwire.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + body.length) := by
      rw [hfront] at Hposition; word
    wp_apply wp_leave c {v with pos' := pos} $$ [Hc] with Hc
    · iframe Hc
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hposfinal, hrestoredepth] at Hc
    isimp only [hdoneall] at Hchildren
    iapply HΦ $$ %out
    iframe Hdst Hc Hs
    isplitl [Hmap Hchildren]
    · unfold ResourceMapOwned
      iexists stored
      iframe Hmap Hchildren
    · ipureintro
      simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1, Int.add_assoc] using And.intro Hnext.1 Hnext.2.2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readResourceMap_map
