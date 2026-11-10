import ProofCheck.HeaderStringComplete
import ProofCheck.HeaderOwnership
import ProofCheck.StringFailure

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

/-- Every complete string rejection preserves the Header field and private
    shared state, conditional on the production UTF-8 validator contract. -/
theorem wp_headerString_rejected (validator : Utf8ValidatorContract (GF := GF)) (c dst base : Loc) (h : Header) (old : HeaderValue) (seen : w64)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (reason : StringFailureReason) (kind : GoInterface)
    (failure : StringRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       stringFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #(structFieldRef Header go!"ProtocolVersion" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StringFailureError err v.pos' reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hh, Hbody, Hprices, Hc, Hs⟩
  icases headerString_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readString_rejected validator c (structFieldRef Header go!"ProtocolVersion" dst) h.ProtocolVersion'
    v xs wire dq reason kind failure hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hh := Hclose $$ %h.ProtocolVersion' Hfield
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerString_rejected
