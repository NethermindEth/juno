import ProofCheck.HeadFault

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

/-- The three head-error cases cover every reserved/indefinite information code. -/
theorem headFault_classify (head : w8) (hinfo : 27 < uint.Z (head &&& W8 31)) :
    ∃ reason, HeadFault head reason := by
  by_cases hindef : head &&& W8 31 = W8 31 ∧
      64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160
  · exact ⟨.indefinite, .indefinite hindef.1 hindef.2⟩
  by_cases hbreak : head &&& W8 31 = W8 31 ∧ head &&& W8 224 = W8 224
  · exact ⟨.breakCode, .breakCode hbreak.1 hbreak.2⟩
  exact ⟨.invalid, .invalid hinfo hindef hbreak⟩

/-- Malformed heads are rejected by the mixed-width limb shortcut. -/
theorem unsignedReject_fault (head : w8) (rest : List w8) (reason : HeadError)
    (fault : HeadFault head reason) : UnsignedReject (head :: rest) := by
  have hi := headFault_info head reason fault
  have h : ∀ b : w8, uint.Z (b &&& W8 31) ≤ uint.Z b := by decide
  have hb := h head
  have hbound : 28 ≤ uint.Z head := by omega
  refine ⟨by omega, ?_, ?_, ?_, ?_⟩
  all_goals intro heq; subst head; contradiction

/-- Every malformed limb head after a valid prefix retains its inner diagnostic
    while the typed felt reader rolls back and preserves its previous pointer. -/
theorem wp_readFelt_limb_fault (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head bad : w8) (count : w64)
    (items : List UnsignedWire) (dq : DFrac) (reason : HeadError) (kind : GoInterface)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hprefix : items.length < 4) (hcount : items.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ (bad :: rem))
    (fault : HeadFault bad reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       HeadFaultError err (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)) bad reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items)
    (bad :: rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = bad :: rem := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  have hi := input_suffix_head xs rem (sint.nat pos) (sint.nat v.buf'.len) bad hsuffix
  have Hbad := input_consumption xs [bad] rem pos v.buf'.len hpnext Hlen.1 hsuffix
  have hlt : sint.Z pos < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hbad
    omega
  wp_apply wp_readFelt_prefix_reject c dst old v xs payload (bad :: rem) head count items dq
    iprop(headFaultAddr (GF := GF) reason ↦□ kind)
    (fun err => HeadFaultError err pos bad reason kind)
    enc hm (unsignedReject_fault bad rem reason fault) hprefix hcount hp hdepth hbytes
    (wp_uint64_fault c {v with pos' := pos, depth' := v.depth' + W64 1} xs bad dq reason kind fault hpnext.1 hlt hi)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

/-- Public Limbs propagates an actual array-head expectation failure. At the
    depth limit it still checks the head first, before rejecting the array depth. -/
theorem wp_limbs_expect_reject (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hne : head ≠ W8 132)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hexpect :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #(W8 128)))
      {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  have hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
    wp_start as ⟨HR, Hdst, Hc, Hs⟩
    wp_auto
    wp_apply hexpect $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    wp_end
  by_cases hd : sint.Z (W64 32) ≤ sint.Z v.depth'
  · wp_start as ⟨HR, Hdst, Hc, Hs⟩
    wp_auto
    simp only [hd, _root_.decide_true]
    wp_auto
    wp_apply hslow $$ [HR Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe HR Hdst Hc Hs
    wp_end
  · exact wp_limbs_nonfast_reject c dst old v xs rest head dq R E hne hp (by change ¬ 32 ≤ sint.Z v.depth' at hd; omega) hbytes hslow

/-- A failed array head cannot publish the typed reader's temporary allocation. -/
theorem wp_readFelt_expect_reject (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hne : head ≠ W8 132) (hn : head ≠ W8 246) (hu : head ≠ W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hexpect :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #(W8 128)))
      {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       R ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hdst, Hc, Hs⟩
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_limbs_expect_reject c «$r0_ptr» (zero_val (GoArray w64 4)) v xs rest head dq R E
    hne hp hbytes hexpect $$ [HR «$r0» Hc Hs] as %err ⟨Hv, Hc, Hs, Herr⟩
  · iframe HR «$r0» Hc Hs
  wp_end

/-- A malformed felt-array head is diagnosed before depth entry, at the original
    offset, without changing the old typed pointer or any cursor/input state. -/
theorem wp_readFelt_head_fault (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (reason : HeadError) (kind : GoInterface)
    (fault : HeadFault head reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err v.pos' head reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hnext
    omega
  have hinfo := headFault_info head reason fault
  have hne : head ≠ W8 132 := by intro h; subst head; contradiction
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_apply wp_readFelt_expect_reject c dst old v xs rest head dq
    iprop(headFaultAddr (GF := GF) reason ↦□ kind) (fun err => HeadFaultError err v.pos' head reason kind)
    hne hn hu hp hbytes
    (wp_expect_fault c v xs head (W8 128) dq reason kind fault (by decide) hp.1 hlt hi)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headFault_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedReject_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_limb_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_expect_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_expect_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_head_fault
