import ProofCheck.FeltHeadFault

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

/-- Skip's felt shortcut cannot hide a proved non-felt head error. This checks
    the actual shortcut/peek dispatch before invoking the proved head contract. -/
theorem wp_skip_head_reject (c : Loc) (v : cursor.Cursor) (depth : w64)
    (xs rem : List w8) (head : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hne : head ≠ W8 132)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem)
    (hhead :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"head")) (Val #()))
      {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hnext := input_consumption xs [head] rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' + 1 ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at hnext
    omega
  have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  by_cases hd : sint.Z depth < sint.Z (W64 32) <;> simp only [hd, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      by_cases hl : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - v.pos') <;>
        simp only [hl, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos'); word⟩)]
      icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
      isimp only [hbytes] at Hbuf
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (head :: rem) dq head (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; rfl
      simp only [hne, _root_.decide_false]
      wp_auto
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
        rw [hbytes]
        iframe Hpre Hbuf Hpost
  all_goals
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
    simp only [hlt, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp.1 $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    wp_apply hhead $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    wp_end

/-- Closed malformed-head rejection by Skip's worker at any supplied depth. -/
theorem wp_skip_fault (c : Loc) (v : cursor.Cursor) (depth : w64)
    (xs rem : List w8) (head : w8) (dq : DFrac) (reason : HeadError) (kind : GoInterface)
    (fault : HeadFault head reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err v.pos' head reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have Hnext := input_consumption xs [head] rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hnext
    omega
  have hinfo := headFault_info head reason fault
  have hne : head ≠ W8 132 := by intro h; subst head; contradiction
  wp_apply wp_skip_head_reject c v depth xs rem head dq
    iprop(headFaultAddr (GF := GF) reason ↦□ kind) (fun err => HeadFaultError err v.pos' head reason kind)
    hne hp hbytes
    (wp_head_reject c v xs head dq iprop(headFaultAddr (GF := GF) reason ↦□ kind)
      (fun err => HeadFaultError err v.pos' head reason kind) hp.1 hlt hi (by omega)
      (wp_headSlow_fault c v xs head dq reason kind fault hp.1 hlt hi))
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Public Skip retains the original offset and state on any malformed head. -/
theorem wp_Skip_fault (c : Loc) (v : cursor.Cursor)
    (xs rem : List w8) (head : w8) (dq : DFrac) (reason : HeadError) (kind : GoInterface)
    (fault : HeadFault head reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err v.pos' head reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_fault c v v.depth' xs rem head dq reason kind fault hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- A malformed extra after four limbs and arbitrary valid ignored contents
    rolls back the whole felt but preserves the offending head's diagnostic. -/
theorem wp_readFelt_extras_fault (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head bad : w8) (count : w64)
    (items : List UnsignedWire) (extras : List (List w8)) (dq : DFrac)
    (reason : HeadError) (kind : GoInterface)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload count) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (hmore : 4 + extras.length < uint.Z count)
    (valid : ∀ item ∈ extras, SkipValid (d + 1) item) (fault : HeadFault bad reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ (bad :: rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       HeadFaultError err (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)) bad reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  let pos := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items ++ extras.flatten)
    (bad :: rem) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpnext : 0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    dsimp only [pos]; constructor <;> word
  have hsuffix : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = bad :: rem := by
    simpa only [pos, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm, Int.add_left_comm, Int.add_assoc] using Hnext.2.2
  wp_apply wp_readFelt_extras_reject c dst old v xs payload (bad :: rem) head count items extras dq
    iprop(headFaultAddr (GF := GF) reason ↦□ kind)
    (fun err => HeadFaultError err pos bad reason kind)
    d hd hdepth enc hm hcount hmore valid hp hbytes
    (wp_Skip_fault c {v with pos' := pos, depth' := v.depth' + W64 1} xs rem bad dq reason kind fault hpnext hsuffix)
    $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_head_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_extras_fault
