import ProofCheck.FeltExtraReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- No complete unsigned item starts here: the input ends, the head has a
    different type/reserved information, or its fixed-width argument is short. -/
def UnsignedReject (wire : List w8) : Prop :=
  match wire with
  | [] => True
  | head :: rest =>
    24 ≤ uint.Z head ∧
    (head = W8 24 → rest.length < 1) ∧
    (head = W8 25 → rest.length < 2) ∧
    (head = W8 26 → rest.length < 4) ∧
    (head = W8 27 → rest.length < 8)

/-- The mixed-width shortcut declines a failed limb after any valid prefix,
    preserving both destination and input even after populating temporary words. -/
theorem wp_shortLimbs_prefix_reject (dst : Loc) (old : GoArray w64 4) (data : GoSlice)
    (items : List UnsignedWire) (rem : List w8) (dq : DFrac)
    (hcount : items.length < 4) (reject : UnsignedReject rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ data ↦*{dq} (W8 132 :: (unsignedBytes items ++ rem)) }}
      (App (App (Val (@! cursor.shortLimbs)) (Val #data)) (Val #dst))
    {{ RET #(W64 0); dst ↦ old ∗ data ↦*{dq} (W8 132 :: (unsignedBytes items ++ rem)) }} := by
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
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hmore : done.length < 4 := by omega
  have hcond : sint.Z (W64 done.length) < sint.Z (W64 4) := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  cases todo with
  | nil =>
    have hdoneall : done = items := by simpa using Hsplit.symm
    have hwidth := unsignedBytes_bound done
    have hpossmall : sint.Z pos ≤ 28 := by omega
    change subslice (sint.nat pos) (sint.nat data.len) (W8 132 :: (unsignedBytes items ++ rem)) = rem at Hremaining
    cases rem with
    | nil =>
      have hlen := congrArg List.length Hremaining
      simp only [subslice_length (sint.nat pos) (sint.nat data.len) (W8 132 :: (unsignedBytes items ++ [])) (by omega), List.length_nil] at hlen
      have heof : sint.Z data.len ≤ sint.Z pos := by
        simp only [sint.nat, sint.Z] at Hlen Hpos hpositive Hwf hlen ⊢
        omega
      simp only [heof, _root_.decide_true]
      wp_auto
      wp_for_post
      iapply HΦ
      iframe Hdst Hs
    | cons bad tail =>
      have hlength := congrArg List.length Hremaining
      simp only [subslice_length (sint.nat pos) (sint.nat data.len) (W8 132 :: (unsignedBytes items ++ (bad :: tail))) (by omega), List.length_cons] at hlength
      have htail : (tail.length : Int) = sint.Z data.len - sint.Z pos - 1 := by
        simp only [sint.nat, sint.Z] at Hlen Hpos hpositive Hwf hlength ⊢
        omega
      have hlt : ¬ sint.Z data.len ≤ sint.Z pos := by omega
      simp only [hlt, _root_.decide_false]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨Hpos.1, by omega⟩)]
      have hi := input_suffix_head _ tail (sint.nat pos) (sint.nat data.len) bad Hremaining
      wp_apply wp_load_slice_index data _ (W8 132 :: (unsignedBytes items ++ (bad :: tail))) dq bad Hpos.1
        $$ [Hs] with Hs
      · iframe Hs; ipureintro; exact hi
      have hsmall : ¬ uint.Z bad < uint.Z (W8 24) := by
        have h := reject.1; change ¬ uint.Z bad < 24; omega
      simp only [hsmall, _root_.decide_false]
      wp_auto
      by_cases h24 : bad = W8 24
      · have hshort := reject.2.1 h24
        have hfit : ¬ sint.Z (pos + W64 2) ≤ sint.Z data.len := by word
        simp only [h24, _root_.decide_true]
        wp_auto
        simp only [hfit, _root_.decide_false]
        wp_auto
        wp_for_post
        iapply HΦ
        iframe Hdst Hs
      · simp only [h24, _root_.decide_false]
        wp_auto
        by_cases h25 : bad = W8 25
        · have hshort := reject.2.2.1 h25
          have hfit : ¬ sint.Z (pos + W64 3) ≤ sint.Z data.len := by word
          simp only [h25, _root_.decide_true]
          wp_auto
          simp only [hfit, _root_.decide_false]
          wp_auto
          wp_for_post
          iapply HΦ
          iframe Hdst Hs
        · simp only [h25, _root_.decide_false]
          wp_auto
          by_cases h26 : bad = W8 26
          · have hshort := reject.2.2.2.1 h26
            have hfit : ¬ sint.Z (pos + W64 5) ≤ sint.Z data.len := by word
            simp only [h26, _root_.decide_true]
            wp_auto
            simp only [hfit, _root_.decide_false]
            wp_auto
            wp_for_post
            iapply HΦ
            iframe Hdst Hs
          · simp only [h26, _root_.decide_false]
            wp_auto
            by_cases h27 : bad = W8 27
            · have hshort := reject.2.2.2.2 h27
              have hfit : ¬ sint.Z (pos + W64 9) ≤ sint.Z data.len := by word
              simp only [h27, _root_.decide_true]
              wp_auto
              simp only [hfit, _root_.decide_false]
              wp_auto
              wp_for_post
              iapply HΦ
              iframe Hdst Hs
            · simp only [h27, _root_.decide_false]
              wp_auto
              wp_for_post
              iapply HΦ
              iframe Hdst Hs
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

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_shortLimbs_prefix_reject
