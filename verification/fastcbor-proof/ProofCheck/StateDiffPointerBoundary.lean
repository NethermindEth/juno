import ProofCheck.StateDiffPointerSpec
import ProofCheck.StateDiffBoundary
import ProofCheck.StateDiffEmpty

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def StateDiffPointerOwned (p : Loc) : Option StateDiffValue → IProp GF
  | none => iprop(⌜p = null⌝)
  | some value => StateDiffOwned p value

theorem stateDiffPointerOwned_zero (p : Loc) :
    p ↦ zero_val StateDiff ⊢ (StateDiffPointerOwned p (some stateDiffZero) : IProp GF) := by
  iintro Hp
  unfold StateDiffPointerOwned
  iapply stateDiffOwned_zero
  iexact Hp

/-- Null and undefined clear the caller's pointer without touching the old
    pointed-to record. Input and enclosing depth are preserved. -/
theorem wp_readStateDiffPointer_nullish (c dst old : Loc) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ (null : Loc) ∗ StateDiffPointerOwned null none ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  wp_auto
  wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  iapply HΦ
  isimp only [StateDiffPointerOwned]
  iframe Hdst Hc Hs
  ipureintro
  simpa only [List.length_singleton, Int.natCast_one] using Hnext

/-- Empty maps allocate and publish a fresh owned zero record; all map-head widths
    and all prior pointers are covered. This differs from nullish pointer clearing. -/
theorem wp_readStateDiffPointer_empty (c dst old : Loc) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c)) (Val #dst))
    {{ (out : Loc), RET #interface.nil; dst ↦ out ∗ StateDiffPointerOwned out (some stateDiffZero) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  have hn : ((head :: payload) ++ rest).head? ≠ some (W8 246) := by
    simp only [List.cons_append, List.head?_cons]
    intro eq
    have heq := Option.some.inj eq
    subst head
    contradiction
  have hu : ((head :: payload) ++ rest).head? ≠ some (W8 247) := by
    simp only [List.cons_append, List.head?_cons]
    intro eq
    have heq := Option.some.inj eq
    subst head
    contradiction
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ rest) dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  ihave Howned := stateDiffPointerOwned_zero «$r0_ptr» $$ «$r0»
  isimp only [StateDiffPointerOwned] at Howned
  wp_apply wp_readStateDiff_empty c «$r0_ptr» stateDiffZero v xs payload rest head dq d enc major hd depth hp hbytes
    $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
  · iframe Howned Hc Hs
  iapply HΦ $$ %«$r0_ptr»
  isimp only [StateDiffPointerOwned]
  iframe Hdst Howned Hc Hs
  ipureintro; exact Hstep

/-- On every opening failure the fresh zero record remains private and owned;
    the caller's original pointer, input and cursor/depth remain unchanged. -/
theorem wp_readStateDiffPointer_open_rejected (c dst old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltFailureReason)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ (∃ temporary : Loc, StateDiffOwned temporary stateDiffZero) ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       FeltFailureError err v.pos' reason (feltFailureKind reason kinds) }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  have excluded := structOpenFailure_not_nullish d wire reason failure
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq excluded.1 excluded.2 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  ihave Howned := stateDiffPointerOwned_zero «$r0_ptr» $$ «$r0»
  isimp only [StateDiffPointerOwned] at Howned
  wp_apply wp_readStateDiff_open_rejected c «$r0_ptr» stateDiffZero v xs wire dq d reason failure hd hp hbytes
    $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr⟩
  · iframe Howned Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs Herr
  iexists «$r0_ptr»
  iexact Howned

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointerOwned_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiffPointer_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiffPointer_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiffPointer_open_rejected
