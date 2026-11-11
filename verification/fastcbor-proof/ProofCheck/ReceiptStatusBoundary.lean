import ProofCheck.ReceiptStatusOwnership
import ProofCheck.ProjectionInitialize
import ProofCheck.ProjectionTagRead
import ProofCheck.StructOpenSound
import ProofCheck.WantedBoundary

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

/-- Malformed tag heads return directly from the actual generated receipt projection
    record function, preserving its owned destination and the original cursor. -/
theorem wp_readReceiptStatus_tag_malformed (c dst : Loc)
    (old : ReceiptStatusValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : ExpectedFailureReason) (category : GoInterface)
    (tagged : projectionHasTag wire) (failure : ExpectedFailure (W8 192) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       expectedFailureAddr (GF := GF) reason ↦□ category ∗ ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadReceiptExecutionStatusProjection)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 192) reason category }} := by
  all_goals
    wp_start as ⟨#Hcategory, Howned, Hc, Hs⟩
    wp_auto
    wp_apply wp_projectionTag_malformed c v xs wire dq reason category tagged failure hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hcategory Hc Hs
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptStatus_tag_malformed
