import ProofCheck.LimbWideSpec

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

/-- With a nonempty destination and a rejected first element, the actual loop
    declines its shortcut, preserves every output slot and returns the exact
    inner diagnostic without changing the cursor or depth. -/
theorem wp_readLimbs_first_failure (c : Loc) (out : GoSlice) (old : GoArray w64 4)
    (others : List (GoArray w64 4)) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (depth offset : Nat) (reason : FeltFailureReason) (kind : GoInterface)
    (failure : LimbsFailure depth wire offset reason) (hd : sint.Z v.depth' = depth)
    (bound : depth < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ out ↦* (old :: others) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"readLimbs")) (Val #out))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       out ↦* (old :: others) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       FeltFailureError err (v.pos' + W64 offset) reason kind }} := by
  wp_start as ⟨#Hkind, Hout, Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  ihave %Houtlen := ownSlice_len _ _ _ $$ Hout
  have hn : sint.Z (W64 0) < sint.Z out.len := by simp only [List.length_cons, sint.nat] at Houtlen; word
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  wp_for
  simp only [hn, _root_.decide_true, ite_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hn⟩)]
  wp_apply +noauto wp_load_slice_index out _ (old :: others) (DFrac.own 1) old (by decide)
    $$ [Hout] with Hout
  · iframe Hout; ipureintro; rfl
  wp_pures
  wp_load
  wp_pures
  wp_store
  wp_pures
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs
    ⟨hp.1, hp.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  have hguard := wp_limbWideGuard (GF := GF) buf_ptr (slice.slice v.buf' w8 v.pos' v.buf'.len) wire dq
  unfold limbWideGuardExpr at hguard
  wp_apply hguard $$ [buf Hbuf] as %ok ⟨Hbufptr, Hbuf, %Hguard⟩
  · iframe buf Hbuf
  have hfalse : ok = false := by cases ok <;> simp_all [limbWideGuard_not_failure depth offset wire reason bound failure]
  subst ok
  wp_auto
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbuf Hpost
  have hpos : v.buf'.len - (v.buf'.len - v.pos') = v.pos' := by word
  isimp only [hpos] at Hc
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hn⟩)]
  wp_auto
  wp_apply wp_limbs_slice_failure c out (old :: others) (sint.Z (W64 0)) old v xs wire dq depth offset reason kind
    (by decide) rfl failure hd (by omega) hp hbytes $$ [Hout Hc Hs] as %err ⟨Hout, Hc, Hs, Herr⟩
  · iframe Hkind Hout Hc Hs
  wp_for_post
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readLimbs_first_failure
