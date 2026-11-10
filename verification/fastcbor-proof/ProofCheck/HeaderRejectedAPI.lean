import ProofCheck.HeaderRecordComplete

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

/-- Every complete Header record rejection propagates
    through actual public cleanup. Earlier fields and private failed allocations
    are dropped under the GC/affine model; the caller owns a zero Header, unchanged
    input and the exact diagnostic, and First returns a nil suffix. The
    production UTF-8 validator contract remains an explicit premise. -/
theorem wp_decodeHeaderFirst_rejected (validator : Utf8ValidatorContract (GF := GF))
    (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (wire : List w8) (retained : HeaderValue)
    (consumed offset extraDepth : Nat) (reason : HeaderRecordFailureReason)
    (failure : HeaderRecordRejected 0 headerZeroValue wire retained consumed offset extraDepth reason)
    (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       HeaderRecordFailureError err (W64 offset) reason (headerRecordFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, Hglobals, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have Hbounds := headerRecordRejected_bounds 0 consumed offset extraDepth headerZeroValue retained wire reason (by decide) failure
  have hoffset : sint.Z (W64 offset) = offset := by
    simp only [sint.nat, sint.Z] at Hlen hp
    word
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor input
  wp_apply wp_readHeader_rejected validator _ dst fields gasFields {(zero_val cursor.Cursor) with buf' := input}
    f gasF wire wire retained kinds qf qo qn qgf qgo qgn dq 0 consumed offset extraDepth reason failure rfl (by decide) hp hb
    $$ [Hglobals c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    as %err ⟨Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hglobals c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  isimp only [HeaderOwned] at Hdst
  icases Hdst with ⟨%h, Hdst, Hchildren⟩
  wp_apply wp_clearHeader_owned dst h retained
    $$ [Hdst Hchildren] with Hdst
  · iframe Hdst Hchildren
  ieval (change (HeaderRecordFailureError err (W64 0 + W64 offset)
    reason (headerRecordFailureKind reason kinds) : IProp GF) ⊢ _) at Herr
  have ho : W64 0 + W64 offset = W64 offset := by word
  isimp only [ho] at Herr
  iapply HΦ
  iframe Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
  ipureintro
  exact ⟨hoffset, Hbounds.2.1⟩

/-- Full decoding returns the same complete record error after actual cleanup,
    with no trailing-data check replacing the diagnostic. -/
theorem wp_decodeHeader_rejected (validator : Utf8ValidatorContract (GF := GF))
    (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (wire : List w8) (retained : HeaderValue)
    (consumed offset extraDepth : Nat) (reason : HeaderRecordFailureReason)
    (failure : HeaderRecordRejected 0 headerZeroValue wire retained consumed offset extraDepth reason)
    (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       HeaderRecordFailureError err (W64 offset) reason (headerRecordFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, Hglobals, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_rejected validator dst fields gasFields input f gasF wire retained consumed offset extraDepth reason failure kinds qf qo qn qgf qgo qgn dq
    $$ [Hglobals Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    as %err ⟨Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hglobals Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_rejected
