import ProofCheck.FeltMapFromBoundary

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

/-- The actual FeltMap reader publishes nil for null/undefined,
    consumes one byte and leaves depth unchanged. -/
theorem wp_readFeltMap_null (c dst : Loc) (old : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ map.nil ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_readFeltMapFrom_null c dst old spare_ptr slice.nil (W64 0) v xs head dq hp hi hn
    $$ [spare Hdst Hc Hs] with ⟨Hspare, Hdst, Hc, Hs⟩
  · iframe spare Hdst Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltMap_null
