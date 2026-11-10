import ProofCheck.HeaderLookupFailure

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

/-- Lookup rejection after any supported successful prefix executes the actual
    First-API cleanup, including nested signatures and both shared price outputs.
    The API returns a nil suffix, owned zero Header, unchanged input, and the exact
    owned duplicate/malformed-key diagnostic at its nonwrapping input offset. -/
theorem wp_decodeHeaderFirst_lookup_failure (dst fields gasFields : Loc) (input : GoSlice)
    (value : HeaderValue) (f gasF : cursor.Fields) (wire : List w8) (offset : Nat)
    (reason : HeaderLookupFailureReason) (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac)
    (failure : HeaderLookupFailure 0 wire value offset reason) :
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
       HeaderLookupFailureError err (W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, Hglobals, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have Hbounds := headerLookupFailure_bounds 0 offset wire value reason failure
  have hoffset : sint.Z (W64 offset) = offset := by
    simp only [sint.nat, sint.Z] at Hlen hp
    word
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor input
  wp_apply wp_readHeader_lookup_failure _ dst fields gasFields value {(zero_val cursor.Cursor) with buf' := input}
    f gasF wire wire kinds qf qo qn qgf qgo qgn dq 0 offset reason failure rfl hp hb
    $$ [Hglobals c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    as %err ⟨Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hglobals c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  isimp only [HeaderOwned] at Hdst
  icases Hdst with ⟨%h, Hdst, Hchildren⟩
  wp_apply wp_clearHeader_owned dst h value $$ [Hdst Hchildren] with Hdst
  · iframe Hdst Hchildren
  ieval (change (HeaderLookupFailureError err (W64 0 + W64 offset) reason (headerLookupFailureKind reason kinds) : IProp GF) ⊢ _) at Herr
  have hz : W64 0 + W64 offset = W64 offset := by word
  isimp only [hz] at Herr
  iapply HΦ
  iframe Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
  ipureintro
  exact ⟨hoffset, Hbounds.1⟩

/-- Full decoding propagates the same owned lookup diagnostic after cleanup;
    it does not replace the parse error with a trailing-data error. -/
theorem wp_decodeHeader_lookup_failure (dst fields gasFields : Loc) (input : GoSlice)
    (value : HeaderValue) (f gasF : cursor.Fields) (wire : List w8) (offset : Nat)
    (reason : HeaderLookupFailureReason) (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac)
    (failure : HeaderLookupFailure 0 wire value offset reason) :
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
       HeaderLookupFailureError err (W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, Hglobals, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_lookup_failure dst fields gasFields input value f gasF wire offset reason kinds qf qo qn qgf qgo qgn dq
    failure $$ [Hglobals Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    as %err ⟨Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hglobals Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_lookup_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_lookup_failure
