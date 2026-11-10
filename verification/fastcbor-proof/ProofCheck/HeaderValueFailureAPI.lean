import ProofCheck.HeaderMapValueFailure

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

/-- A specified value failure after an arbitrary HeaderEntry prefix propagates
    through actual public cleanup. Earlier fields and private failed allocations
    are dropped under the GC/affine model; the caller owns a zero Header, unchanged
    input and the exact diagnostic, and First returns a nil suffix. -/
theorem wp_decodeHeaderFirst_value_failure (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (item : HeaderValueFailureWire 0)
    (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} item.bytes ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       HeaderValueFailureError err (W64 item.errorOffset) item.reason (headerValueFailureKind item.reason kinds) ∗
       ⌜sint.Z (W64 item.errorOffset) = item.errorOffset ∧ item.errorOffset ≤ item.bytes.length⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, Hglobals, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) item.bytes = item.bytes := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have Hbounds := headerValueFailureWire_bounds 0 item
  have hoffset : sint.Z (W64 item.errorOffset) = item.errorOffset := by
    simp only [sint.nat, sint.Z] at Hlen hp
    word
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor input
  wp_apply wp_readHeader_map_value_failure _ dst fields gasFields {(zero_val cursor.Cursor) with buf' := input}
    f gasF item.bytes item.payload item.tail item.head kinds qf qo qn qgf qgo qgn dq
    0 item.count item.consumed item.offset item.extraDepth item.entries item.key item.reason
    item.enc item.major rfl item.bound item.more item.fit item.fresh item.failure hp hb
    $$ [Hglobals c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    as %err ⟨Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hglobals c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  isimp only [HeaderOwned] at Hdst
  icases Hdst with ⟨%h, Hdst, Hchildren⟩
  wp_apply wp_clearHeader_owned dst h (item.entries.foldl HeaderEntry.apply headerZeroValue)
    $$ [Hdst Hchildren] with Hdst
  · iframe Hdst Hchildren
  ieval (change (HeaderValueFailureError err
    (W64 0 + W64 (1 + item.payload.length + (item.entries.flatMap HeaderEntry.bytes).length) + W64 item.key.bytes.length + W64 item.offset)
    item.reason (headerValueFailureKind item.reason kinds) : IProp GF) ⊢ _) at Herr
  have ho : W64 0 + W64 (1 + item.payload.length + (item.entries.flatMap HeaderEntry.bytes).length) + W64 item.key.bytes.length + W64 item.offset =
      W64 item.errorOffset := by
    simp only [HeaderValueFailureWire.errorOffset, HeaderValueFailureWire.valueStart]
    word
  isimp only [ho] at Herr
  iapply HΦ
  iframe Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
  ipureintro
  exact ⟨hoffset, Hbounds.2.1⟩

/-- Full decoding returns the same specified value error after actual cleanup,
    with no trailing-data check replacing the diagnostic. -/
theorem wp_decodeHeader_value_failure (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (item : HeaderValueFailureWire 0)
    (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} item.bytes ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       HeaderValueFailureError err (W64 item.errorOffset) item.reason (headerValueFailureKind item.reason kinds) ∗
       ⌜sint.Z (W64 item.errorOffset) = item.errorOffset ∧ item.errorOffset ≤ item.bytes.length⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, Hglobals, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_value_failure dst fields gasFields input f gasF item kinds qf qo qn qgf qgo qgn dq
    $$ [Hglobals Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    as %err ⟨Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hglobals Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_value_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_value_failure
