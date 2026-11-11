import ProofCheck.IntElementFailure

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

/-- Failure after any valid signed-element prefix leaves the old destination
    unpublished, retains the entered array depth and the specified element
    progress, and returns the exact sentinel or owned diagnostic. -/
theorem wp_readInts_prefix_failure (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (dq : DFrac) (d count consumed : Nat)
    (items : List IntElementWire) (reason : IntElementFailureReason) (k : BigIntErrorKinds)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 128)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (more : items.length < count)
    (fit : count ≤ (items.flatMap IntElementWire.bytes ++ wire).length)
    (failure : IntElementRejected wire consumed reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ items.flatMap IntElementWire.bytes ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals k ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadInts)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       dst ↦ old ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length) + W64 consumed, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       IntElementFailureError result
         (v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length)) reason k }} := by
  wp_start as ⟨#Hglobals, Hdst, Hc, Hs⟩
  isimp only [BigIntErrorGlobals] at Hglobals
  icases Hglobals with ⟨#Htruncated, #Hsyntax, #Htype, #Hunsupported, %Hne⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ items.flatMap IntElementWire.bytes) wire v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + (items.flatMap IntElementWire.bytes).length ≤ sint.Z v.buf'.len := by
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
  have hi := input_suffix_head xs (payload ++ items.flatMap IntElementWire.bytes ++ wire)
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
      payload ++ (items.flatMap IntElementWire.bytes ++ wire) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_ArrayLen_encoded c v xs payload (items.flatMap IntElementWire.bytes ++ wire) head (W64 count) dq
    enc hm hp.1 (by omega) (by omega) hcountfit hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_slice_make2_nonnil (V := w64) (W64 count) as %out ⟨Hout, -, %Houtnn⟩
  · ipureintro; exact hcount
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  have hnat : sint.nat (W64 count) = count := by word
  isimp only [hnat] at Hout
  ihave %Houtsize := ownSlice_len _ _ _ $$ Hout
  have houtlen : sint.Z out.len = count := by
    simp only [List.length_replicate, sint.nat, sint.Z] at Houtsize ⊢; omega
  have Hhead := input_consumption xs (head :: payload) (items.flatMap IntElementWire.bytes ++ wire)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      items.flatMap IntElementWire.bytes ++ wire := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (IntElementWire)) (pos oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hout" ∷ out ↦* (done.map IntElementWire.value ++ List.replicate (count - done.length) (W64 0)) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap IntElementWire.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap IntElementWire.bytes ++ wire⌝ : IProp GF)
      $$ [Hrange Hi Hout Hc]
  · iexists [], items, (v.pos' + W64 (1 + payload.length)), (W64 0)
    isimp only [List.map_nil, List.nil_append]
    iframe
    ipureintro
    exact ⟨trivial, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hdone : done.length ≤ items.length := by omega
  have hslices : (done.map IntElementWire.value).length = done.length := List.length_map _
  have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  have hleft : 0 < count - done.length := by omega
  have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  have hidxnat : (sint.Z (W64 done.length)).toNat = done.length := by word
  have hlookup : (done.map IntElementWire.value ++ List.replicate (count - done.length) (W64 0))[(sint.Z (W64 done.length)).toNat]? = some (W64 0) := by
    rw [hidxnat, ← hslices]
    exact uninitialized_prefix_lookup _ _ _ (by simpa only [hslices] using hleft)
  wp_apply wp_load_slice_index (t := go.int) out (sint.Z (W64 done.length))
    (done.map IntElementWire.value ++ List.replicate (count - done.length) (W64 0))
    (DFrac.own 1) (W64 0) hindex.1 $$ [Hout] with Hout
  · iframe Hout
    ipureintro; exact hlookup
  cases todo with
  | nil =>
    have hdoneall : done = items := by simpa using Hsplit.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (items.flatMap IntElementWire.bytes).length) := by
      rw [hdoneall] at Hposition; word
    have hwire : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = wire := by simpa using Hremaining
    cases failure with
    | @overflow b body argument tail elemEnc major bound =>
      have hbad : uint.Z (W64 9223372036854775807) < uint.Z argument := by
        change (9223372036854775807 : Int) < uint.Z argument
        omega
      wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs tail
        ⟨b, body, argument, major, elemEnc⟩ dq Hpos hwire $$ [Hc Hs] with ⟨Hc, Hs, %Hstep⟩
      · iframe Hc Hs
      simp only [hbad, _root_.decide_true]
      wp_auto
      wp_for_post
      isimp only [UnsignedWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (body.length : Int) 1, hposfinal] at Hc
      iapply HΦ
      isimp only [Int.natCast_add, Int.natCast_one]
      iframe Hdst Hc Hs
      isimp only [IntElementFailureError]
      ipureintro; trivial
    | @fallback wire reason unsignedReason unsignedFailure notNull notUndefined signedFailure =>
      ihave #Hkind : (expectedFailureAddr (GF := GF) unsignedReason ↦□
          bigIntCategory (bigIntExpectedKind k unsignedReason) : IProp GF) $$ []
      · cases unsignedReason with
        | head _ reason => cases reason <;> isimp only [expectedFailureAddr, headFaultAddr, bigIntExpectedKind, bigIntHeadKind] <;> (iframe # <;> iframe)
        | incomplete | eof | typeMismatch _ | tag => isimp only [expectedFailureAddr, bigIntExpectedKind]; iframe # <;> iframe
      wp_apply wp_uint64_failure c {v with pos' := pos, depth' := v.depth' + W64 1} xs wire dq
        unsignedReason (bigIntCategory (bigIntExpectedKind k unsignedReason)) unsignedFailure Hpos hwire
        $$ [Hc Hs] as %discarded ⟨Hc, Hs, Hdiscarded⟩
      · iframe Hkind Hc Hs
      wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1} xs wire dq
        notNull notUndefined Hpos hwire $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      wp_apply wp_int64_rejected c {v with pos' := pos, depth' := v.depth' + W64 1} xs wire dq
        reason k signedFailure Hpos hwire $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · isimp only [BigIntErrorGlobals]
        iframe # <;> iframe
        ipureintro; exact Hne
      wp_for_post
      isimp only [hposfinal] at Hc Herr
      iapply HΦ
      have hz (x : w64) : x + W64 0 = x := by word
      isimp only [IntElementFailureError, Int.natCast_zero, hz]
      iframe Hdst Hc Hs
      iexists err
      iframe Herr
      ipureintro; rfl
  | cons item todo =>
    have hstepcount : count - done.length = (count - (done ++ [item]).length) + 1 := by
      simp only [List.length_cons] at hlength
      simp only [List.length_append, List.length_singleton]
      omega
    have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item.bytes ++ (todo.flatMap IntElementWire.bytes ++ wire) := by
      simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
    have Hadvance := input_consumption xs item.bytes (todo.flatMap IntElementWire.bytes ++ wire) pos v.buf'.len Hpos Hlen.1 hitem
    have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
      simp only [List.length_append, List.length_singleton]; word
    have hnewlen : ((done ++ [item]).flatMap IntElementWire.bytes).length = (done.flatMap IntElementWire.bytes).length + item.bytes.length := by simp
    have houtstep : (done.map IntElementWire.value ++ List.replicate (count - done.length) (W64 0)).set done.length item.value =
        (done ++ [item]).map IntElementWire.value ++ List.replicate (count - (done ++ [item]).length) (W64 0) := by
      simpa only [hstepcount, List.length_map, List.map_append, List.map_singleton] using
        initialized_prefix_step (done.map IntElementWire.value) (count - (done ++ [item]).length) (W64 0) item.value
    cases item with
    | nullish b hn =>
      have hbi := input_suffix_head xs _ (sint.nat pos) (sint.nat v.buf'.len) b hitem
      have enc : HeadArgument b [] (W64 22) ∨ HeadArgument b [] (W64 23) := by
        rcases hn with rfl | rfl
        · exact Or.inl (.inline _ (by decide))
        · exact Or.inr (.inline _ (by decide))
      have hu : b &&& W8 224 ≠ W8 0 := by rcases hn with rfl | rfl <;> decide
      have ht : b &&& W8 224 ≠ W8 192 := by rcases hn with rfl | rfl <;> decide
      have hbody : subslice (sint.nat (pos + W64 1)) (sint.nat v.buf'.len) xs = [] ++ (todo.flatMap IntElementWire.bytes ++ wire) := by
        simpa only [IntElementWire.bytes, List.length_singleton, Int.natCast_one, List.nil_append] using Hadvance.2.2
      have hbound : sint.Z pos + 1 + ([] : List w8).length ≤ sint.Z v.buf'.len := by
        simp only [IntElementWire.bytes, List.length_singleton, Int.natCast_one] at Hadvance
        simpa using Hadvance.1 ▸ Hadvance.2.1
      rcases enc with enc | enc
      all_goals
        wp_apply wp_uint64_type c {v with pos' := pos, depth' := v.depth' + W64 1} xs []
          (todo.flatMap IntElementWire.bytes ++ wire) b _ dq (bigIntCategory k.typeError)
          enc hu ht Hpos.1 hbound hbi hbody $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
        · iframe Htype Hc Hs
        wp_apply wp_null_at c {v with pos' := pos, depth' := v.depth' + W64 1} xs b dq Hpos.1 hbi hn
          $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        have houtnull : done.map IntElementWire.value ++ List.replicate (count - done.length) (W64 0) =
            (done ++ [IntElementWire.nullish b hn]).map IntElementWire.value ++ List.replicate (count - (done ++ [IntElementWire.nullish b hn]).length) (W64 0) := by
          rw [hstepcount]
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
        (todo.flatMap IntElementWire.bytes ++ wire) ⟨b, body, argument, major, enc⟩ dq Hpos hitem
        $$ [Hc Hs] with ⟨Hc, Hs, %Hstep⟩
      · iframe Hc Hs
      have hgood : ¬ uint.Z (W64 9223372036854775807) < uint.Z argument := by
        change ¬ (9223372036854775807 : Int) < uint.Z argument
        omega
      simp only [hgood, _root_.decide_false]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
      wp_apply wp_store_slice_index (t := go.int) out (sint.Z (W64 done.length))
        (done.map IntElementWire.value ++ List.replicate (count - done.length) (W64 0))
        argument $$ [Hout] with Hout
      · iframe Hout
        ipureintro
        simp only [List.length_append, List.length_map, List.length_replicate]
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
      have hbody : subslice (sint.nat (pos + W64 1)) (sint.nat v.buf'.len) xs = body ++ (todo.flatMap IntElementWire.bytes ++ wire) := by
        have hp1 : sint.nat (pos + W64 1) = sint.nat pos + 1 := by word
        rw [hp1]
        exact input_suffix_after xs [b] _ _ _ hitem
      wp_apply wp_uint64_type c {v with pos' := pos, depth' := v.depth' + W64 1} xs body
        (todo.flatMap IntElementWire.bytes ++ wire) b argument dq (bigIntCategory k.typeError)
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
        (todo.flatMap IntElementWire.bytes ++ wire) b argument dq enc (Or.inr major) bound Hpos hitem
        $$ [Hc Hs] with ⟨Hc, Hs, %Hstep⟩
      · iframe Hc Hs
      simp only [major, ite_true]
      rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
      wp_apply wp_store_slice_index (t := go.int) out (sint.Z (W64 done.length))
        (done.map IntElementWire.value ++ List.replicate (count - done.length) (W64 0))
        (~~~argument) $$ [Hout] with Hout
      · iframe Hout
        ipureintro
        simp only [List.length_append, List.length_map, List.length_replicate]
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
end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readInts_prefix_failure
