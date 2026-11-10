import ProofCheck.HeaderScalars
import ProofCheck.HeaderSignatures

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

/-- A specified scalar rejection preserves every Header field and child,
    returning its actual owned diagnostic without consuming the value. -/
theorem wp_headerScalar_failure (c dst : Loc) (h : Header) (old : HeaderValue) (field : HeaderScalarField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ScalarFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ h ∗ HeaderBody h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (if headerScalarMode field then (@! cborReadMode) else (@! cborReadUint64))) (Val #c))
        (Val #(structFieldRef Header (headerScalarName field) dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderBody h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 0) reason kind }} := by
  iintro %Φ ⟨#Hpkg, #Hkind, Hh, Hbody, Hc, Hs⟩ HΦ
  icases headerScalar_acc dst h field $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readScalar_failure (headerScalarMode field) c (structFieldRef Header (headerScalarName field) dst)
    (headerScalarWord field h) v xs wire dq reason kind failure hp hbytes $$ [Hfield Hc Hs]
    as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hh := Hclose $$ %(headerScalarWord field h) Hfield
  isimp only [headerSetScalarWord_original] at Hh
  wp_end

/-- Failure inside arbitrarily nested signatures publishes no replacement
    signature slice. Prior Header ownership remains intact, while the cursor
    retains the child's specified progress and entered nesting levels. -/
theorem wp_headerSignatures_failure (c dst : Loc) (h : Header) (old : HeaderValue)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat)
    (reason : FeltFailureReason) (kind : GoInterface)
    (failure : SignaturesFailure d wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ h ∗ HeaderBody h old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #(structFieldRef Header go!"Signatures" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ h ∗ HeaderBody h old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ FeltFailureError err (v.pos' + W64 offset) reason kind ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hkind, Hh, Hbody, Hc, Hs⟩
  icases headerSignatures_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
  wp_apply wp_readSignatures_failure_bounded c (structFieldRef Header go!"Signatures" dst) h.Signatures'
    v xs wire dq d consumed offset extraDepth reason kind failure hd hdepth hp hbytes $$ [Hfield Hc Hs]
    as %err ⟨Hfield, Hc, Hs, Herr, %Hprogress⟩
  · iframe Hkind Hfield Hc Hs
  ihave Hh := Hclose $$ %h.Signatures' Hfield
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerScalar_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerSignatures_failure
