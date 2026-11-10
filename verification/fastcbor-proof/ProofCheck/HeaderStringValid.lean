import ProofCheck.HeaderStringComplete
import ProofCheck.HeaderFieldSuccess

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

/-- Complete successful ProtocolVersion values, including nullish preservation,
    update exactly that field and its seen bit in the Header loop invariant. -/
theorem wp_headerString_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst base : Loc) (h : Header) (old : HeaderValue) (seen : w64)
    (v : cursor.Cursor) (xs wire rem : List w8) (value : GoString) (dq : DFrac)
    (valid : StringValid old.protocolVersion wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ HeaderBody h old ∗ HeaderGasReady base h old seen ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #(structFieldRef Header go!"ProtocolVersion" dst)))
    {{ RET #interface.nil;
       dst ↦ {h with ProtocolVersion' := value} ∗ HeaderBody {h with ProtocolVersion' := value} {old with protocolVersion := value} ∗
       HeaderGasReady base {h with ProtocolVersion' := value} {old with protocolVersion := value} (headerKeySeen seen (some .protocolVersion)) ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hh, Hbody, Hready, Hc, Hs⟩ HΦ
  isimp only [HeaderBody] at Hbody
  icases Hbody with ⟨%Hscalars, Hchildren⟩
  ihave Hbody : HeaderBody h old $$ [Hchildren]
  · unfold HeaderBody
    iframe Hchildren
    ipureintro; exact Hscalars
  have version : h.ProtocolVersion' = old.protocolVersion := Hscalars.2.2.2.2.1
  have hvalid : StringValid h.ProtocolVersion' wire value := by simpa only [version] using valid
  icases headerString_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readString_valid validator c (structFieldRef Header go!"ProtocolVersion" dst)
    h.ProtocolVersion' v xs wire rem value dq hvalid hp bytes $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Hh := Hclose $$ %value Hfield
  ihave Hbody := headerBody_set_string h old value $$ Hbody
  ihave Hready := headerGasReady_mark base h old seen (some .protocolVersion) $$ Hready
  iapply HΦ
  isimp only [headerGasReady_set_string]
  iframe Hh Hbody Hready Hc Hs
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerString_valid
