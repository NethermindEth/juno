import ProofCheck.StateDiffOwnership
import ProofCheck.StateDiffRecordSpec
import ProofCheck.StateOpen
import ProofCheck.Header

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Direct null/undefined StateDiff decoding consumes one byte and preserves every
    prior field and child allocation, without entering a map. -/
theorem wp_readStateDiff_nullish (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiff)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hpkg, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  ihave #Htable := state_initialized_table (GF := GF) .stateDiff $$ Hpkg
  isimp only [StateTable, stateGlobal] at Htable
  icases Htable with ⟨%fields, #Hglobal, _⟩
  wp_func_call
  wp_call
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_nullish c fields v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_for
  wp_apply wp_nextField_done c _ fields $$ [s] with Hst
  · iframe
  wp_for_post
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  simpa only [List.length_singleton, Int.natCast_one] using Hnext

/-- Opening failures return from the actual StateDiff loop without modifying the
    record, cursor or enclosing depth; initialized categories determine errors. -/
theorem wp_readStateDiff_open_rejected (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltFailureReason)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiff)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       FeltFailureError err v.pos' reason (feltFailureKind reason kinds) }} := by
  iintro %Φ ⟨#Hpkg, Howned, Hc, Hs⟩ HΦ
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply cursor_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
  ihave #Htable := state_initialized_table (GF := GF) .stateDiff $$ Hpkg
  isimp only [StateTable, stateGlobal] at Htable
  icases Htable with ⟨%fields, #Hglobal, _⟩
  wp_func_call
  wp_call
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_failure c fields v xs wire dq d reason (feltFailureKind reason kinds) failure hd hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_for
  iapply HΦ
  iframe Howned Hc Hs
  iexists kinds
  iframe Hglobals Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiff_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiff_open_rejected
