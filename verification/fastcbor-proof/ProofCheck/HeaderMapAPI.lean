import ProofCheck.HeaderMap

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

/-- The public First API decodes arbitrary successful Header maps in the current
    composition family, returns the exact unread input slice and preserves the
    complete input. The returned Header owns its nested allocations. -/
theorem wp_decodeHeaderFirst_map_supported (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (item : HeaderMapWire 0) (rem : List w8)
    (kind : GoInterface) (qf qo qn qgf qgo qgn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ globalAddr cborGasPriceFields ↦□ gasFields ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ dst ↦ (zero_val Header) ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ RET (PairV #(slice.slice input w8 (W64 item.bytes.length) input.len) #interface.nil);
       HeaderOwned dst item.value ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       ⌜sint.Z (input.len - W64 item.bytes.length) = rem.length ∧
         subslice (sint.nat (W64 item.bytes.length)) (sint.nat input.len) (item.bytes ++ rem) = rem⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, #Hkind, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) (item.bytes ++ rem) = item.bytes ++ rem := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have hfit : (item.bytes.length : Int) ≤ sint.Z input.len := by
    simp only [List.length_append, sint.nat, sint.Z] at Hlen hp ⊢
    omega
  have hposition : W64 0 + W64 (1 + item.payload.length + (item.entries.flatMap HeaderEntry.bytes).length) =
      W64 item.bytes.length := by
    rw [headerMapWire_length]
    simp only [Int.natCast_add, Int.natCast_one]
    word
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor input
  wp_apply wp_readHeader_map_supported _ dst fields gasFields {(zero_val cursor.Cursor) with buf' := input}
    f gasF (item.bytes ++ rem) item.payload rem item.head kind qf qo qn qgf qgo qgn dq 0 item.entries
    item.enc item.major rfl item.bound item.fresh hp hb
    $$ [c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    with ⟨Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, %Hnext⟩
  · iframe Hglobal HgasGlobal Hkind c Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  ieval (change (c_ptr ↦ {(zero_val cursor.Cursor) with buf' := input, pos' := W64 0 + W64 (1 + item.payload.length + (item.entries.flatMap HeaderEntry.bytes).length)} : IProp GF) ⊢ _) at Hc
  isimp only [hposition] at Hc
  wp_apply wp_remaining _ {(zero_val cursor.Cursor) with buf' := input, pos' := W64 item.bytes.length}
    (by change 0 ≤ sint.Z (W64 item.bytes.length) ∧ sint.Z (W64 item.bytes.length) ≤ sint.Z input.len; word)
    Hcap.2 $$ [Hc] with Hc
  · iframe Hc
  iapply HΦ
  isimp only [HeaderMapWire.value]
  iframe Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  ipureintro
  constructor
  · simp only [List.length_append, sint.nat, sint.Z] at Hlen
    word
  · simpa only [show (zero_val cursor.Cursor).pos' = W64 0 by rfl, hposition] using Hnext.2

/-- Full decoding accepts the same arbitrary map when it occupies all input. -/
theorem wp_decodeHeader_map_supported (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (item : HeaderMapWire 0)
    (kind : GoInterface) (qf qo qn qgf qgo qgn dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ globalAddr cborGasPriceFields ↦□ gasFields ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ RET #interface.nil;
       HeaderOwned dst item.value ∗ input ↦*{dq} item.bytes ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, #Hkind, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_map_supported dst fields gasFields input f gasF item [] kind qf qo qn qgf qgo qgn dq
    $$ [Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    with ⟨Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, %Hrest⟩
  · isimp only [List.append_nil]
    iframe Hglobal HgasGlobal Hkind Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  have hz : input.len - W64 item.bytes.length = W64 0 := by
    have := Hrest.1
    simp only [List.length_nil, Int.natCast_zero] at this
    word
  simp only [slice.slice, hz]
  wp_auto
  isimp only [List.append_nil] at Hs
  wp_end

/-- Trailing bytes after any map in the composition family trigger the actual
    full-API cleanup. Every nested output is dropped and the diagnostic records
    exactly the unread byte count; the input remains unchanged. -/
theorem wp_decodeHeader_map_trailing (dst fields gasFields : Loc) (input : GoSlice)
    (f gasF : cursor.Fields) (item : HeaderMapWire 0) (rem : List w8)
    (kind extraKind : GoInterface) (qf qo qn qgf qgo qgn dq : DFrac) (hmore : rem ≠ []) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ globalAddr cborGasPriceFields ↦□ gasFields ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ globalAddr fastcbor.ErrExtraneousData ↦□ extraKind ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType fastcbor.DecodeError.ty) #err);
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       err ↦ {(zero_val fastcbor.DecodeError) with Kind' := extraKind, Extra' := W64 rem.length} }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, #Hkind, #HextraKind, Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_map_supported dst fields gasFields input f gasF item rem kind qf qo qn qgf qgo qgn dq
    $$ [Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames]
    with ⟨Hdst, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames, %Hrest⟩
  · iframe Hglobal HgasGlobal Hkind Hdst Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
  have hpositive := List.length_pos_iff.mpr hmore
  have hcount : input.len - W64 item.bytes.length = W64 rem.length := by have := Hrest.1; word
  have hn : input.len - W64 item.bytes.length ≠ W64 0 := by have := Hrest.1; word
  simp only [slice.slice, hn, decide_false]
  wp_auto
  isimp only [HeaderOwned] at Hdst
  icases Hdst with ⟨%h, Hdst, Hchildren⟩
  wp_apply wp_clearHeader_owned dst h item.value $$ [Hdst Hchildren] with Hdst
  · iframe Hdst Hchildren
  wp_alloc err as Herr
  wp_auto
  isimp only [hcount] at Herr
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_map_supported
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_map_supported
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_map_trailing
