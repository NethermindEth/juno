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

/-- Every zero-count map width runs the actual open/close path, preserving the
    prior record, restoring enclosing depth and consuming exactly the map head. -/
theorem wp_readStateDiff_empty (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiff)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; StateDiffOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hpkg, Howned, Hc, Hs⟩ HΦ
  ihave #Htable := state_initialized_table (GF := GF) .stateDiff $$ Hpkg
  isimp only [StateTable, stateGlobal] at Htable
  icases Htable with ⟨%fields, #Hglobal, _⟩
  wp_func_call
  wp_call
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_map c fields v xs payload rest head (W64 0) dq d enc major hd depth
    (by word) hp hbytes $$ [Hc Hs] with ⟨Hc, Hs, %Hopen⟩
  · iframe Hc Hs
  wp_for
  wp_apply wp_nextField_close c s_ptr fields {v with pos' := v.pos' + W64 (1 + payload.length)}
    (W64 0) (W64 0) $$ [s Hc] with ⟨Hst, Hc⟩
  · iframe
  wp_for_post
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  exact ⟨Hopen.1, Hopen.2.1, Hopen.2.2.1⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiff_empty
