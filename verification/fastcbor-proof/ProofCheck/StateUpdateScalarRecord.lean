import ProofCheck.StateUpdateScalarMap
import ProofCheck.StateUpdateBoundary

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

/-- Every independently accepted StateUpdate record in the scalar-only scope is realized by the actual
    generated reader, with exact owned contents, suffix and restored depth. -/
theorem wp_readStateUpdate_scalar_record_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : StateUpdateValue) (v : cursor.Cursor)
    (xs item rem : List w8) (dq : DFrac) (d : Nat)
    (valid : StateUpdateScalarRecordValid d old item rem final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateUpdate)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; StateUpdateOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases valid with
  | @nullish head suffix hn =>
    wp_apply wp_readStateUpdate_nullish c dst old v xs rem head dq hn hp hbytes
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; simpa only [Int.add_assoc] using Hnext
  | @map head payload body suffix argument final count enc major size depth pairs =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readStateUpdate_scalar_map_valid validator c dst old final v xs payload body rem head dq d count
      enc major hd depth pairs hp hbytes $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    have length : ((head :: payload) ++ body).length = 1 + payload.length + body.length := by simp; omega
    isimp only [length, Int.natCast_add, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; simpa only [Int.add_assoc] using Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateUpdate_scalar_record_valid
