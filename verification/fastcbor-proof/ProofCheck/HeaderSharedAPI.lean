import ProofCheck.FieldsShared
import ProofCheck.HeaderMapAPI
import ProofCheck.HeaderLookupFailureAPI
import ProofCheck.HeaderValueFailureAPI

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/- These contracts consume only the destination and input ownership. Metadata is
   persistent, so independent invocations can use the same tables without dividing
   exclusive ownership. They retain the input-family restrictions of the underlying
   Header proofs; package initialization still has to establish HeaderMetadata. -/

/-- Successful DecodeHeaderFirst using persistent published metadata. -/
theorem wp_decodeHeaderFirst_map_shared (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (rem : List w8) (kind : GoInterface) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       HeaderMetadata ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} (item.bytes ++ rem) }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ RET (PairV #(slice.slice input w8 (W64 item.bytes.length) input.len) #interface.nil);
       HeaderOwned dst item.value ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       ⌜sint.Z (input.len - W64 item.bytes.length) = rem.length ∧
         subslice (sint.nat (W64 item.bytes.length)) (sint.nat input.len) (item.bytes ++ rem) = rem⌝ }} := by
  wp_start_folded as ⟨#Hmeta, #Hkind, Hdst, Hs⟩
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  wp_apply wp_decodeHeaderFirst_map_supported dst fields gasFields input f gasF item rem kind
    DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq $$ [Hdst Hs]
    with ⟨Hdst, Hs, _, _, _, _, _, _, %Hrest⟩
  · iframe Hglobal HgasGlobal Hfields Horder Hnames HgasFields HgasOrder HgasNames Hkind Hdst Hs
  iapply HΦ
  iframe Hdst Hs
  ipureintro; exact Hrest

/-- Successful DecodeHeader using persistent published metadata. -/
theorem wp_decodeHeader_map_shared (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (kind : GoInterface) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       HeaderMetadata ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ RET #interface.nil;
       HeaderOwned dst item.value ∗ input ↦*{dq} item.bytes }} := by
  wp_start_folded as ⟨#Hmeta, #Hkind, Hdst, Hs⟩
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  wp_apply wp_decodeHeader_map_supported dst fields gasFields input f gasF item kind
    DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq $$ [Hdst Hs]
    with ⟨Hdst, Hs, _, _, _, _, _, _⟩
  · iframe Hglobal HgasGlobal Hfields Horder Hnames HgasFields HgasOrder HgasNames Hkind Hdst Hs
  iapply HΦ
  iframe Hdst Hs

/-- Specified lookup failures through DecodeHeaderFirst, sharing immutable metadata. -/
theorem wp_decodeHeaderFirst_lookup_failure_shared (dst : Loc) (input : GoSlice)
    (value : HeaderValue) (wire : List w8) (offset : Nat)
    (reason : HeaderLookupFailureReason) (kinds : FeltErrorKinds) (dq : DFrac)
    (failure : HeaderLookupFailure 0 wire value offset reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       HeaderMetadata ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗
       HeaderLookupFailureError err (W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start_folded as ⟨#Hmeta, Hglobals, Hdst, Hs⟩
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  wp_apply wp_decodeHeaderFirst_lookup_failure dst fields gasFields input value f gasF wire offset reason kinds
    DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq
    failure $$ [Hglobals Hdst Hs]
    as %err ⟨Hdst, Hs, _, _, _, _, _, _, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hfields Horder Hnames HgasFields HgasOrder HgasNames Hglobals Hdst Hs
  iapply HΦ
  iframe Hdst Hs Herr
  ipureintro; exact Hprogress

/-- Specified lookup failures through DecodeHeader, sharing immutable metadata. -/
theorem wp_decodeHeader_lookup_failure_shared (dst : Loc) (input : GoSlice)
    (value : HeaderValue) (wire : List w8) (offset : Nat)
    (reason : HeaderLookupFailureReason) (kinds : FeltErrorKinds) (dq : DFrac)
    (failure : HeaderLookupFailure 0 wire value offset reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       HeaderMetadata ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗
       HeaderLookupFailureError err (W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start_folded as ⟨#Hmeta, Hglobals, Hdst, Hs⟩
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  wp_apply wp_decodeHeader_lookup_failure dst fields gasFields input value f gasF wire offset reason kinds
    DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq
    failure $$ [Hglobals Hdst Hs]
    as %err ⟨Hdst, Hs, _, _, _, _, _, _, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hfields Horder Hnames HgasFields HgasOrder HgasNames Hglobals Hdst Hs
  iapply HΦ
  iframe Hdst Hs Herr
  ipureintro; exact Hprogress

/-- Specified value failures through DecodeHeaderFirst, sharing immutable metadata. -/
theorem wp_decodeHeaderFirst_value_failure_shared (dst : Loc) (input : GoSlice)
    (item : HeaderValueFailureWire 0) (kinds : FeltErrorKinds) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       HeaderMetadata ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} item.bytes ∗
       HeaderValueFailureError err (W64 item.errorOffset) item.reason (headerValueFailureKind item.reason kinds) ∗
       ⌜sint.Z (W64 item.errorOffset) = item.errorOffset ∧ item.errorOffset ≤ item.bytes.length⌝ }} := by
  wp_start_folded as ⟨#Hmeta, Hglobals, Hdst, Hs⟩
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  wp_apply wp_decodeHeaderFirst_value_failure dst fields gasFields input f gasF item kinds
    DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq $$ [Hglobals Hdst Hs]
    as %err ⟨Hdst, Hs, _, _, _, _, _, _, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hfields Horder Hnames HgasFields HgasOrder HgasNames Hglobals Hdst Hs
  iapply HΦ
  iframe Hdst Hs Herr
  ipureintro; exact Hprogress

/-- Specified value failures through DecodeHeader, sharing immutable metadata. -/
theorem wp_decodeHeader_value_failure_shared (dst : Loc) (input : GoSlice)
    (item : HeaderValueFailureWire 0) (kinds : FeltErrorKinds) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       HeaderMetadata ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} item.bytes ∗
       HeaderValueFailureError err (W64 item.errorOffset) item.reason (headerValueFailureKind item.reason kinds) ∗
       ⌜sint.Z (W64 item.errorOffset) = item.errorOffset ∧ item.errorOffset ≤ item.bytes.length⌝ }} := by
  wp_start_folded as ⟨#Hmeta, Hglobals, Hdst, Hs⟩
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  wp_apply wp_decodeHeader_value_failure dst fields gasFields input f gasF item kinds
    DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq $$ [Hglobals Hdst Hs]
    as %err ⟨Hdst, Hs, _, _, _, _, _, _, Herr, %Hprogress⟩
  · iframe Hglobal HgasGlobal Hfields Horder Hnames HgasFields HgasOrder HgasNames Hglobals Hdst Hs
  iapply HΦ
  iframe Hdst Hs Herr
  ipureintro; exact Hprogress

/-- Trailing bytes trigger actual cleanup even when all metadata is shared. -/
theorem wp_decodeHeader_map_trailing_shared (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (rem : List w8) (kind extraKind : GoInterface) (dq : DFrac)
    (hmore : rem ≠ []) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       HeaderMetadata ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       globalAddr fastcbor.ErrExtraneousData ↦□ extraKind ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} (item.bytes ++ rem) }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType fastcbor.DecodeError.ty) #err);
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       err ↦ {(zero_val fastcbor.DecodeError) with Kind' := extraKind, Extra' := W64 rem.length} }} := by
  wp_start_folded as ⟨#Hmeta, #Hkind, #HextraKind, Hdst, Hs⟩
  isimp only [HeaderMetadata] at Hmeta
  icases Hmeta with ⟨%fields, %gasFields, #Hglobal, #HgasGlobal, #Htable, #HgasTable⟩
  isimp only [StaticFieldsShared] at Htable HgasTable
  icases Htable with ⟨%f, #Hfields, #Horder, #Hnames, %_⟩
  icases HgasTable with ⟨%gasF, #HgasFields, #HgasOrder, #HgasNames, %_⟩
  wp_apply wp_decodeHeader_map_trailing dst fields gasFields input f gasF item rem kind extraKind
    DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard DFrac.discard dq hmore
    $$ [Hdst Hs] as %err ⟨Hdst, Hs, _, _, _, _, _, _, Herr⟩
  · iframe Hglobal HgasGlobal Hfields Horder Hnames HgasFields HgasOrder HgasNames Hkind HextraKind Hdst Hs
  iapply HΦ
  iframe Hdst Hs Herr

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_map_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_map_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_lookup_failure_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_lookup_failure_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_value_failure_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_value_failure_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_map_trailing_shared
