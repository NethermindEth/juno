import ProofCheck.FeltSequence

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

theorem unsigned_head_info (head : w8) (hm : head &&& W8 224 = W8 0) : head &&& W8 31 = head := by
  have h : ∀ head : w8, head &&& W8 224 = W8 0 → head &&& W8 31 = head := by decide
  exact h head hm

/-- A known unsigned item fits in the remaining input, including its complete argument. -/
theorem input_wire_bound (xs rest : List w8) (item : UnsignedWire) (pos len : w64)
    (hp : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z len)
    (hlen : xs.length = sint.nat len)
    (hbytes : subslice (sint.nat pos) (sint.nat len) xs = item.bytes ++ rest) :
    sint.Z pos + 1 + item.payload.length ≤ sint.Z len := by
  have hcount := congrArg List.length hbytes
  simp only [subslice_length (sint.nat pos) (sint.nat len) xs (by omega),
    UnsignedWire.bytes, List.length_append, List.length_cons] at hcount
  simp only [sint.nat, sint.Z] at hlen hcount hp ⊢
  omega


/-- The five standard definite unsigned encodings, retaining the item's identity for loop proofs. -/
inductive UnsignedShape (item : UnsignedWire) : Prop where
  | inline (hsmall : uint.Z item.head < 24) (hpayload : item.payload = [])
      (hvalue : item.value = W64 (uint.Z item.head))
  | byte (b : w8) (hhead : item.head = W8 24) (hpayload : item.payload = [b])
      (hvalue : item.value = W64 (uint.Z b))
  | u16 (a b : w8) (hhead : item.head = W8 25) (hpayload : item.payload = [a, b])
      (hvalue : item.value = W64 (uint.Z (W16 (uint.Z b) ||| (W16 (uint.Z a) <<< W64 8))))
  | u32 (hhead : item.head = W8 26) (hlen : item.payload.length = 4)
      (hvalue : item.value = W64 (uint.Z (leToU32 item.payload.reverse)))
  | u64 (hhead : item.head = W8 27) (hlen : item.payload.length = 8)
      (hvalue : item.value = leToU64 item.payload.reverse)

theorem unsignedWire_shape (item : UnsignedWire) : UnsignedShape item := by
  rcases item with ⟨head, payload, value, hm, enc⟩
  have hlow := unsigned_head_info head hm
  cases enc with
  | inline hs => exact .inline (by simpa only [hlow] using hs) rfl (by simp only [hlow])
  | byte b hi => exact .byte b (by simpa only [hlow] using hi) rfl rfl
  | u16 a b hi => exact .u16 a b (by simpa only [hlow] using hi) rfl rfl
  | u32 bs hlen hi => exact .u32 (by simpa only [hlow] using hi) hlen rfl
  | u64 bs hlen hi => exact .u64 (by simpa only [hlow] using hi) hlen rfl

theorem wp_bigEndian_offset32 (data : GoSlice) (xs bs rem : List w8) (off high : w64) (dq : DFrac) (hhigh : high = data.len)
    (hbound : 0 ≤ sint.Z off ∧ sint.Z off ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat off) (sint.nat data.len) xs = bs ++ rem)
    (hlen : bs.length = 4) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.encoding.binary ∗ data ↦*{dq} xs }}
      (App (Val ((globalAddr encoding.binary.BigEndian) @!!
        go.PointerType encoding.binary.bigEndian.ty @!! go!"Uint32"))
        (Val #(slice.slice data w8 off high)))
    {{ RET #(leToU32 bs.reverse); data ↦*{dq} xs }} := by
  subst high
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice off data.len data dq xs
      ⟨hbound.1, hbound.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes] at Hbytes
  wp_apply encoding.binary.wp_BigEndian_Uint32 (slice.slice data w8 off data.len)
    bs dq rem hlen $$ [Hbytes] with Hbytes
  · iframe Hbytes
  iapply HΦ
  iapply (ownSlice_slice off data.len data dq xs
      ⟨hbound.1, hbound.2, Int.le_refl _⟩).2
  rw [hbytes]
  iframe Hpre Hbytes Hpost

theorem wp_bigEndian_offset16 (data : GoSlice) (xs rem : List w8) (a b : w8) (off high : w64) (dq : DFrac) (hhigh : high = data.len)
    (hbound : 0 ≤ sint.Z off ∧ sint.Z off ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat off) (sint.nat data.len) xs = [a, b] ++ rem)
    :
    {{ isPkgInit (PROP := IProp GF) pkg_id.encoding.binary ∗ data ↦*{dq} xs }}
      (App (Val ((globalAddr encoding.binary.BigEndian) @!!
        go.PointerType encoding.binary.bigEndian.ty @!! go!"Uint16"))
        (Val #(slice.slice data w8 off high)))
    {{ RET #(W16 (uint.Z b) ||| (W16 (uint.Z a) <<< W64 8)); data ↦*{dq} xs }} := by
  subst high
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice off data.len data dq xs
      ⟨hbound.1, hbound.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes, List.cons_append, List.nil_append] at Hbytes
  wp_apply wp_binaryBigEndian_u16 (slice.slice data w8 off data.len) a b rem dq $$ [Hbytes] with Hbytes
  · iframe Hbytes
  iapply HΦ
  iapply (ownSlice_slice off data.len data dq xs
      ⟨hbound.1, hbound.2, Int.le_refl _⟩).2
  rw [hbytes]
  simp only [List.cons_append, List.nil_append]
  iframe Hpre Hbytes Hpost

/-- The mixed-width shortcut accepts four independently encoded unsigned limbs. -/
theorem wp_shortLimbs_four (dst : Loc) (old : GoArray w64 4) (data : GoSlice)
    (items : List UnsignedWire) (rem : List w8) (dq : DFrac) (hcount : items.length = 4) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ data ↦*{dq} (W8 132 :: (unsignedBytes items ++ rem)) }}
      (App (App (Val (@! cursor.shortLimbs)) (Val #data)) (Val #dst))
    {{ RET #(W64 (1 + (unsignedBytes items).length)); dst ↦ paddedLimbs items ∗
       data ↦*{dq} (W8 132 :: (unsignedBytes items ++ rem)) }} := by
  wp_start as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hlen : 1 + (unsignedBytes items).length + rem.length = sint.nat data.len := by
    simpa only [List.length_cons, List.length_append, Nat.add_assoc, Nat.add_comm, Nat.add_left_comm] using Hlen.1
  have hpositive : 0 < sint.Z data.len := by simp only [sint.nat, sint.Z] at Hlen ⊢; simp only [List.length_cons, List.length_append] at Hlen; omega
  have hn : data.len ≠ W64 0 := by intro h; rw [h] at hpositive; contradiction
  wp_auto
  simp only [hn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hpositive⟩)]
  wp_apply wp_load_slice_index data _ (W8 132 :: (unsignedBytes items ++ rem)) dq (W8 132)
    (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  rename_i key_ptr
  irename : (key_ptr ↦ zero_val w64 : IProp GF) => Hkey
  ihave HI : (∃ (done todo : List UnsignedWire) (pos key : w64),
      "Hi" ∷ i_ptr ↦ W64 done.length ∗
      "Hkey" ∷ key_ptr ↦ key ∗
      "Hn" ∷ n_ptr ↦ pos ∗
      "Hlimbs" ∷ limbs_ptr ↦ paddedLimbs done ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z data.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = 1 + (unsignedBytes done).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat data.len)
        (W8 132 :: (unsignedBytes items ++ rem)) = unsignedBytes todo ++ rem⌝ : IProp GF)
      $$ [i Hkey n limbs]
  · iexists [], items, (W64 1), (W64 0)
    iframe
    ipureintro
    refine ⟨rfl, ⟨by decide, by change 1 ≤ sint.Z data.len; omega⟩, rfl, ?_⟩
    rw [← Hlen.1]
    simp only [subslice, List.take_length]
    rfl
  have hf : (#false : val) ≠ #true := fun h => Bool.noConfusion (go.intoVal_inj h)
  wp_for HI
  have hlength : done.length + todo.length = 4 := by rw [← List.length_append, ← Hsplit, hcount]
  have hdone : done.length ≤ 4 := by omega
  by_cases hmore : done.length < 4
  · have hcond : sint.Z (W64 done.length) < sint.Z (W64 4) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    cases todo with
    | nil => simp only [List.length_nil, Nat.add_zero] at hlength; omega
    | cons item todo =>
      have hbound := input_wire_bound (W8 132 :: (unsignedBytes items ++ rem))
        (unsignedBytes todo ++ rem) item pos data.len Hpos Hlen.1
        (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
      have hlt : ¬ sint.Z data.len ≤ sint.Z pos := by omega
      simp only [hlt, _root_.decide_false]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨Hpos.1, by omega⟩)]
      have hi := input_suffix_head (W8 132 :: (unsignedBytes items ++ rem))
        (item.payload ++ (unsignedBytes todo ++ rem)) (sint.nat pos) (sint.nat data.len) item.head
        (by simpa [unsignedBytes, UnsignedWire.bytes, List.append_assoc] using Hremaining)
      wp_apply wp_load_slice_index data _ (W8 132 :: (unsignedBytes items ++ rem)) dq item.head Hpos.1
        $$ [Hs] with Hs
      · iframe Hs; ipureintro; exact hi
      have hindex : sint.Z (W64 done.length) = done.length := by word
      have hidx : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < 4 := by rw [hindex]; omega
      have hw := headArgument_width item.head item.payload item.value item.argument
      have hafter : subslice (sint.nat (pos + W64 1)) (sint.nat data.len)
          (W8 132 :: (unsignedBytes items ++ rem)) = item.payload ++ (unsignedBytes todo ++ rem) := by
        have hn : sint.nat (pos + W64 1) = sint.nat pos + 1 := by word
        rw [hn]
        exact input_suffix_after _ [item.head] _ _ _ (by
          simpa [UnsignedWire.bytes, unsignedBytes, List.append_assoc] using Hremaining)
      icases array_acc limbs_ptr (done.length : Int) (DFrac.own 1) 4 (paddedLimbs done) (W64 0)
          (by omega) (by simpa using paddedLimbs_next done hmore) $$ Hlimbs with ⟨Hword, Hrestore⟩
      cases unsignedWire_shape item with
      | inline hsmall hpayload hvalue =>
        have hs : uint.Z item.head < uint.Z (W8 24) := hsmall
        simp only [hs, _root_.decide_true]
        wp_auto
        rw [ite_eq_left_of_eq_true _ _ (eq_true hidx)]
        simp only [hindex]
        wp_auto
        isimp only [← hvalue] at Hword
        have hsize : item.bytes.length = 1 := by simp [UnsignedWire.bytes, hpayload]
        ihave Hlimbs := Hrestore $$ %item.value Hword
        isimp only [Int.toNat_natCast, paddedLimbs_step done item hmore] at Hlimbs
        have hnstep : sint.Z (pos + W64 1) = sint.Z pos + item.bytes.length := by
          simp only [hsize]; word
        have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
          simp only [List.length_append, List.length_singleton]; word
        have hnewlen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by
          simp [unsignedBytes]
        wp_for_post
        isimp only [hinc] at Hi
        iframe
        iexists (done ++ [item]), todo, (pos + W64 1), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, ?_⟩, ?_, ?_⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [hnstep]; omega
        · have hsz : item.bytes.length = 1 + item.payload.length := by simp [UnsignedWire.bytes]; omega
          rw [hnstep, hsz]; omega
        · rw [hnstep, hnewlen]; omega
        · have hn : sint.nat (pos + W64 1) = sint.nat pos + item.bytes.length := by
            simp only [hsize]; word
          rw [hn]
          exact input_suffix_after _ item.bytes _ _ _ (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
      | byte b hhead hpayload hvalue =>
        have hwidth : item.payload.length = 1 := by simp [hpayload]
        have hfit : sint.Z (pos + W64 2) ≤ sint.Z data.len := by word
        repeat (first | simp only [hhead, hfit, _root_.decide_true] | wp_auto)
        have hidxbyte : 0 ≤ sint.Z (pos + W64 1) ∧ sint.Z (pos + W64 1) < sint.Z data.len := by word
        rw [ite_eq_left_of_eq_true _ _ (eq_true hidxbyte)]
        wp_apply wp_load_slice_index data _ (W8 132 :: (unsignedBytes items ++ rem)) dq b hidxbyte.1
            $$ [Hs] with Hs
        · iframe Hs
          ipureintro
          change (W8 132 :: (unsignedBytes items ++ rem))[sint.nat (pos + W64 1)]? = some b
          exact input_suffix_head _ (unsignedBytes todo ++ rem) (sint.nat (pos + W64 1)) (sint.nat data.len) b
            (by simpa only [hpayload, List.singleton_append] using hafter)
        rw [ite_eq_left_of_eq_true _ _ (eq_true hidx)]
        simp only [hindex]
        wp_auto
        isimp only [← hvalue] at Hword
        have hsize : item.bytes.length = 2 := by simp [UnsignedWire.bytes, hwidth]
        ihave Hlimbs := Hrestore $$ %item.value Hword
        isimp only [Int.toNat_natCast, paddedLimbs_step done item hmore] at Hlimbs
        have hnstep : sint.Z (pos + W64 2) = sint.Z pos + item.bytes.length := by
          simp only [hsize]; word
        have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
          simp only [List.length_append, List.length_singleton]; word
        have hnewlen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by
          simp [unsignedBytes]
        wp_for_post
        isimp only [hinc] at Hi
        iframe
        iexists (done ++ [item]), todo, (pos + W64 2), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, ?_⟩, ?_, ?_⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [hnstep]; omega
        · have hsz : item.bytes.length = 1 + item.payload.length := by simp [UnsignedWire.bytes]; omega
          rw [hnstep, hsz]; omega
        · rw [hnstep, hnewlen]; omega
        · have hn : sint.nat (pos + W64 2) = sint.nat pos + item.bytes.length := by
            simp only [hsize]; word
          rw [hn]
          exact input_suffix_after _ item.bytes _ _ _ (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
      | u16 a b hhead hpayload hvalue =>
        have hwidth : item.payload.length = 2 := by simp [hpayload]
        have hfit : sint.Z (pos + W64 3) ≤ sint.Z data.len := by word
        repeat (first | simp only [hhead, hfit, _root_.decide_true] | wp_auto)
        have hsliceb : 0 ≤ sint.Z (pos + W64 1) ∧ sint.Z (pos + W64 1) ≤ sint.Z data.len ∧
            sint.Z data.len ≤ sint.Z data.cap := ⟨by word, by word, Hwf.2⟩
        rw [ite_eq_left_of_eq_true _ _ (eq_true hsliceb)]
        wp_apply wp_bigEndian_offset16 data (W8 132 :: (unsignedBytes items ++ rem))
            (unsignedBytes todo ++ rem) a b (pos + W64 1) data.len dq rfl
            ⟨hsliceb.1, hsliceb.2.1⟩ (by simpa only [hpayload] using hafter) $$ [Hs] with Hs
        · iframe Hs
        rw [ite_eq_left_of_eq_true _ _ (eq_true hidx)]
        simp only [hindex]
        wp_auto
        isimp only [← hvalue] at Hword
        have hsize : item.bytes.length = 3 := by simp [UnsignedWire.bytes, hwidth]
        ihave Hlimbs := Hrestore $$ %item.value Hword
        isimp only [Int.toNat_natCast, paddedLimbs_step done item hmore] at Hlimbs
        have hnstep : sint.Z (pos + W64 3) = sint.Z pos + item.bytes.length := by
          simp only [hsize]; word
        have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
          simp only [List.length_append, List.length_singleton]; word
        have hnewlen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by
          simp [unsignedBytes]
        wp_for_post
        isimp only [hinc] at Hi
        iframe
        iexists (done ++ [item]), todo, (pos + W64 3), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, ?_⟩, ?_, ?_⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [hnstep]; omega
        · have hsz : item.bytes.length = 1 + item.payload.length := by simp [UnsignedWire.bytes]; omega
          rw [hnstep, hsz]; omega
        · rw [hnstep, hnewlen]; omega
        · have hn : sint.nat (pos + W64 3) = sint.nat pos + item.bytes.length := by
            simp only [hsize]; word
          rw [hn]
          exact input_suffix_after _ item.bytes _ _ _ (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
      | u32 hhead hwidth hvalue =>
        have hfit : sint.Z (pos + W64 5) ≤ sint.Z data.len := by word
        repeat (first | simp only [hhead, hfit, _root_.decide_true] | wp_auto)
        have hsliceb : 0 ≤ sint.Z (pos + W64 1) ∧ sint.Z (pos + W64 1) ≤ sint.Z data.len ∧
            sint.Z data.len ≤ sint.Z data.cap := ⟨by word, by word, Hwf.2⟩
        rw [ite_eq_left_of_eq_true _ _ (eq_true hsliceb)]
        wp_apply wp_bigEndian_offset32 data (W8 132 :: (unsignedBytes items ++ rem)) item.payload
            (unsignedBytes todo ++ rem) (pos + W64 1) data.len dq rfl
            ⟨hsliceb.1, hsliceb.2.1⟩ hafter hwidth $$ [Hs] with Hs
        · iframe Hs
        rw [ite_eq_left_of_eq_true _ _ (eq_true hidx)]
        simp only [hindex]
        wp_auto
        isimp only [← hvalue] at Hword
        have hsize : item.bytes.length = 5 := by simp [UnsignedWire.bytes, hwidth]
        ihave Hlimbs := Hrestore $$ %item.value Hword
        isimp only [Int.toNat_natCast, paddedLimbs_step done item hmore] at Hlimbs
        have hnstep : sint.Z (pos + W64 5) = sint.Z pos + item.bytes.length := by
          simp only [hsize]; word
        have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
          simp only [List.length_append, List.length_singleton]; word
        have hnewlen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by
          simp [unsignedBytes]
        wp_for_post
        isimp only [hinc] at Hi
        iframe
        iexists (done ++ [item]), todo, (pos + W64 5), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, ?_⟩, ?_, ?_⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [hnstep]; omega
        · have hsz : item.bytes.length = 1 + item.payload.length := by simp [UnsignedWire.bytes]; omega
          rw [hnstep, hsz]; omega
        · rw [hnstep, hnewlen]; omega
        · have hn : sint.nat (pos + W64 5) = sint.nat pos + item.bytes.length := by
            simp only [hsize]; word
          rw [hn]
          exact input_suffix_after _ item.bytes _ _ _ (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
      | u64 hhead hwidth hvalue =>
        have hfit : sint.Z (pos + W64 9) ≤ sint.Z data.len := by word
        repeat (first | simp only [hhead, hfit, _root_.decide_true] | wp_auto)
        have hsliceb : 0 ≤ sint.Z (pos + W64 1) ∧ sint.Z (pos + W64 1) ≤ sint.Z data.len ∧
            sint.Z data.len ≤ sint.Z data.cap := ⟨by word, by word, Hwf.2⟩
        rw [ite_eq_left_of_eq_true _ _ (eq_true hsliceb)]
        wp_apply wp_bigEndian_offset data (W8 132 :: (unsignedBytes items ++ rem)) item.payload
            (unsignedBytes todo ++ rem) (pos + W64 1) data.len dq rfl
            ⟨hsliceb.1, hsliceb.2.1⟩ hafter hwidth $$ [Hs] with Hs
        · iframe Hs
        rw [ite_eq_left_of_eq_true _ _ (eq_true hidx)]
        simp only [hindex]
        wp_auto
        isimp only [← hvalue] at Hword
        have hsize : item.bytes.length = 9 := by simp [UnsignedWire.bytes, hwidth]
        ihave Hlimbs := Hrestore $$ %item.value Hword
        isimp only [Int.toNat_natCast, paddedLimbs_step done item hmore] at Hlimbs
        have hnstep : sint.Z (pos + W64 9) = sint.Z pos + item.bytes.length := by
          simp only [hsize]; word
        have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by
          simp only [List.length_append, List.length_singleton]; word
        have hnewlen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by
          simp [unsignedBytes]
        wp_for_post
        isimp only [hinc] at Hi
        iframe
        iexists (done ++ [item]), todo, (pos + W64 9), (W64 done.length)
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, ?_⟩, ?_, ?_⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [hnstep]; omega
        · have hsz : item.bytes.length = 1 + item.payload.length := by simp [UnsignedWire.bytes]; omega
          rw [hnstep, hsz]; omega
        · rw [hnstep, hnewlen]; omega
        · have hn : sint.nat (pos + W64 9) = sint.nat pos + item.bytes.length := by
            simp only [hsize]; word
          rw [hn]
          exact input_suffix_after _ item.bytes _ _ _ (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
  · have hcond : ¬ sint.Z (W64 done.length) < sint.Z (W64 4) := by word
    simp only [hcond, hf, _root_.decide_true, _root_.decide_false, Bool.false_eq_true, ite_true, ite_false]
    wp_auto
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    have hdoneall : done = items := by simpa [htodo] using Hsplit.symm
    have hposfinal : pos = W64 (1 + (unsignedBytes items).length) := by rw [hdoneall] at Hposition; word
    isimp only [hdoneall] at Hdst
    rw [hposfinal]
    iapply HΦ
    iframe Hdst Hs

/-- Every encoded unsigned limb occupies at most nine bytes. -/
theorem unsignedBytes_bound (items : List UnsignedWire) : (unsignedBytes items).length ≤ 9 * items.length := by
  induction items with
  | nil => simp [unsignedBytes]
  | cons item items ih =>
    have hw := headArgument_width item.head item.payload item.value item.argument
    simp only [unsignedBytes, List.flatMap_cons, UnsignedWire.bytes, List.length_append, List.length_cons] at *
    omega

/-- The wide head fixes both payload width and decoded word. -/
theorem unsignedWire_wide (item : UnsignedWire) (hhead : item.head = W8 27) :
    item.payload.length = 8 ∧ item.value = leToU64 item.payload.reverse := by
  cases unsignedWire_shape item with
  | inline h _ _ => rw [hhead] at h; contradiction
  | byte _ h _ _ => rw [hhead] at h; contradiction
  | u16 _ _ h _ _ => rw [hhead] at h; contradiction
  | u32 h _ _ => rw [hhead] at h; contradiction
  | u64 _ hlen hvalue => exact ⟨hlen, hvalue⟩

/-- On four valid unsigned items, the hard-coded offsets recognize exactly the wide heads. -/
theorem four_wide_guards (a b c d : UnsignedWire) (rem : List w8)
    (h1 : (W8 132 :: (unsignedBytes [a,b,c,d] ++ rem))[1]? = some (W8 27))
    (h10 : (W8 132 :: (unsignedBytes [a,b,c,d] ++ rem))[10]? = some (W8 27))
    (h19 : (W8 132 :: (unsignedBytes [a,b,c,d] ++ rem))[19]? = some (W8 27))
    (h28 : (W8 132 :: (unsignedBytes [a,b,c,d] ++ rem))[28]? = some (W8 27)) :
    a.head = W8 27 ∧ b.head = W8 27 ∧ c.head = W8 27 ∧ d.head = W8 27 := by
  have ha : a.head = W8 27 := by simpa [unsignedBytes, UnsignedWire.bytes] using h1
  have hla := (unsignedWire_wide a ha).1
  have hb : b.head = W8 27 := by
    simpa [unsignedBytes, UnsignedWire.bytes, List.getElem?_append, hla] using h10
  have hlb := (unsignedWire_wide b hb).1
  have hc : c.head = W8 27 := by
    simpa [unsignedBytes, UnsignedWire.bytes, List.getElem?_append, hla, hlb] using h19
  have hlc := (unsignedWire_wide c hc).1
  have hd : d.head = W8 27 := by
    simpa [unsignedBytes, UnsignedWire.bytes, List.getElem?_append, hla, hlb, hlc] using h28
  exact ⟨ha, hb, hc, hd⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.unsigned_head_info
#print axioms Perennial.github_com.NethermindEth.juno.core.input_wire_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_shortLimbs_four

#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedWire_shape

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigEndian_offset16

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigEndian_offset32

#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedBytes_bound

#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedWire_wide

#print axioms Perennial.github_com.NethermindEth.juno.core.four_wide_guards
