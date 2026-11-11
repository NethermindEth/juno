import ProofCheck.StateUpdateFeltFields
import ProofCheck.StateUpdateScalarSpec

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

/-- Independent successful field decoding determines the exact result of the
    actual call, including nullish preservation and the exact unread suffix. -/
theorem wp_stateUpdate_scalar_field_valid (c dst : Loc) (old final : StateUpdateValue) (field : StateUpdateField) (v : cursor.Cursor)
    (xs item rem : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (valid : StateUpdateFieldValid d field old item rem final) (scalar : field ≠ .diff)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #(structFieldRef StateUpdate (stateUpdateFieldName field) dst)))
    {{ RET #interface.nil;
       StateUpdateOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  cases valid with
  | blockHash valid =>
    simpa only [stateUpdateFieldName] using
      wp_stateUpdate_blockHash_valid (GF := GF) c dst old v xs item rem _ dq d valid hd hdepth hp hbytes
  | newRoot valid =>
    simpa only [stateUpdateFieldName] using
      wp_stateUpdate_newRoot_valid (GF := GF) c dst old v xs item rem _ dq d valid hd hdepth hp hbytes
  | oldRoot valid =>
    simpa only [stateUpdateFieldName] using
      wp_stateUpdate_oldRoot_valid (GF := GF) c dst old v xs item rem _ dq d valid hd hdepth hp hbytes
  | diff => exact False.elim (scalar rfl)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_scalar_field_valid
