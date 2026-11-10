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

/-- The public First API decodes every complete successful Header record, returns the exact unread input slice and preserves the
    complete input. The returned Header owns its nested allocations. The
    production UTF-8 validator contract remains an explicit premise. -/
theorem wp_decodeHeaderFirst_valid (validator : Utf8ValidatorContract (GF := GF))
    (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (wire : List w8) (final : HeaderValue) (valid : HeaderRecordValid 0 headerZeroValue wire final) (rem : List w8)
    (kind : GoInterface) (qf qo qn qgf qgo qgn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ dst ↦ (zero_val Header) ∗ input ↦*{dq} (wire ++ rem) ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ RET (PairV #(slice.slice input w8 (W64 wire.length) input.len) #interface.nil);
       HeaderOwned dst final ∗ input ↦*{dq} (wire ++ rem) ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       ⌜sint.Z (input.len - W64 wire.length) = rem.length ∧
         subslice (sint.nat (W64 wire.length)) (sint.nat input.len) (wire ++ rem) = rem⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, #Hkind, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) (wire ++ rem) = wire ++ rem := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have hfit : (wire.length : Int) ≤ sint.Z input.len := by
    simp only [List.length_append, sint.nat, sint.Z] at Hlen hp ⊢
    omega
  have hposition : W64 0 + W64 wire.length = W64 wire.length := by word
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor input
  wp_apply wp_readHeader_valid validator _ dst fields gasFields {(zero_val cursor.Cursor) with buf' := input}
    f gasF (wire ++ rem) wire rem final kind qf qo qn qgf qgo qgn dq 0 valid rfl hp hb
    $$ [c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    with ⟨Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, %Hnext⟩
  · iframe Hglobal HgasGlobal Hkind c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  ieval (change (c_ptr ↦ {(zero_val cursor.Cursor) with buf' := input, pos' := W64 0 + W64 wire.length} : IProp GF) ⊢ _) at Hc
  isimp only [hposition] at Hc
  wp_apply wp_remaining _ {(zero_val cursor.Cursor) with buf' := input, pos' := W64 wire.length}
    (by change 0 ≤ sint.Z (W64 wire.length) ∧ sint.Z (W64 wire.length) ≤ sint.Z input.len; word)
    Hcap.2 $$ [Hc] with Hc
  · iframe Hc
  iapply HΦ
  iframe Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  ipureintro
  constructor
  · simp only [List.length_append, sint.nat, sint.Z] at Hlen
    word
  · simpa only [show (zero_val cursor.Cursor).pos' = W64 0 by rfl, hposition] using Hnext.2.2

/-- Full decoding accepts the same complete record when it occupies all input. -/
theorem wp_decodeHeader_valid (validator : Utf8ValidatorContract (GF := GF))
    (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (wire : List w8) (final : HeaderValue) (valid : HeaderRecordValid 0 headerZeroValue wire final)
    (kind : GoInterface) (qf qo qn qgf qgo qgn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ dst ↦ (zero_val Header) ∗ input ↦*{dq} wire ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ RET #interface.nil;
       HeaderOwned dst final ∗ input ↦*{dq} wire ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, #Hkind, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_valid validator dst fields gasFields input f gasF wire final valid [] kind qf qo qn qgf qgo qgn dq
    $$ [Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    with ⟨Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, %Hrest⟩
  · isimp only [List.append_nil]
    iframe Hglobal HgasGlobal Hkind Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  have hz : input.len - W64 wire.length = W64 0 := by
    have := Hrest.1
    simp only [List.length_nil, Int.natCast_zero] at this
    word
  simp only [slice.slice, hz]
  wp_auto
  isimp only [List.append_nil] at Hs
  wp_end

/-- Trailing bytes after any complete successful record trigger the actual
    full-API cleanup. Every nested output is dropped and the diagnostic records
    exactly the unread byte count; the input remains unchanged. -/
theorem wp_decodeHeader_valid_trailing (validator : Utf8ValidatorContract (GF := GF))
    (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (wire : List w8) (final : HeaderValue) (valid : HeaderRecordValid 0 headerZeroValue wire final) (rem : List w8)
    (kind extraKind : GoInterface) (qf qo qn qgf qgo qgn dq : DFrac) (hmore : rem ≠ []) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ globalAddr cborerr.ErrExtraneousData ↦□ extraKind ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} (wire ++ rem) ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err);
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} (wire ++ rem) ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := extraKind, Extra' := W64 rem.length} }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, #Hkind, #HextraKind, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_valid validator dst fields gasFields input f gasF wire final valid rem kind qf qo qn qgf qgo qgn dq
    $$ [Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    with ⟨Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, %Hrest⟩
  · iframe Hglobal HgasGlobal Hkind Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  have hpositive := List.length_pos_iff.mpr hmore
  have hcount : input.len - W64 wire.length = W64 rem.length := by have := Hrest.1; word
  have hn : input.len - W64 wire.length ≠ W64 0 := by have := Hrest.1; word
  simp only [slice.slice, hn, decide_false]
  wp_auto
  isimp only [HeaderOwned] at Hdst
  icases Hdst with ⟨%h, Hdst, Hchildren⟩
  wp_apply wp_clearHeader_owned dst h final $$ [Hdst Hchildren] with Hdst
  · iframe Hdst Hchildren
  wp_alloc err as Herr
  wp_auto
  isimp only [hcount] at Herr
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_valid_trailing
