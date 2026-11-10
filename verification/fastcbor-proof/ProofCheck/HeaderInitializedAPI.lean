import ProofCheck.HeaderSharedAPI
import ProofCheck.HeaderZero
import ProofCheck.InitializationErrors
import ProofCheck.EntryErrorsInitialize

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/- These contracts use only the initialization of packages read by the decoder,
   plus input/destination ownership. No caller supplies tables, error globals or
   an assumption about core/planner startup. They retain the existing explicit
   Header input-family restrictions; they are not the complete Header theorem. -/

/-- A caller can recover the actual cursor unsupported-error value from schema
    startup, without separately supplying global memory ownership. -/
theorem schema_initialized_unsupported :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ⊢
      ∃ kind : GoInterface, globalAddr cursor.ErrUnsupported ↦□ kind := by
  iintro #Hpkg
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, Hglobals⟩
  isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
  icases Hglobals with ⟨⟨_, _, #Hunsupported, _⟩, _⟩
  iexists kinds.unsupportedKind
  iframe Hunsupported

/-- Actual DecodeHeaderFirst success, deriving its metadata and error globals from startup. -/
theorem wp_decodeHeaderFirst_map_initialized (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (rem : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} (item.bytes ++ rem) }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ RET (PairV #(slice.slice input w8 (W64 item.bytes.length) input.len) #interface.nil); HeaderOwned dst item.value ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       ⌜sint.Z (input.len - W64 item.bytes.length) = rem.length ∧
         subslice (sint.nat (W64 item.bytes.length)) (sint.nat input.len) (item.bytes ++ rem) = rem⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave Hglobal := schema_initialized_unsupported (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobal with ⟨%kind, #Hkind⟩
  wp_apply wp_decodeHeaderFirst_map_shared dst input item rem kind dq $$ [Hdst Hs] with ⟨Hdst, Hs, %Hrest⟩
  · iframe Hkind Hdst Hs
  iapply HΦ
  iframe Hdst Hs
  ipureintro; exact Hrest

/-- Actual DecodeHeader success, deriving its metadata and error globals from startup. -/
theorem wp_decodeHeader_map_initialized (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ RET #interface.nil; HeaderOwned dst item.value ∗ input ↦*{dq} item.bytes }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave Hglobal := schema_initialized_unsupported (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobal with ⟨%kind, #Hkind⟩
  wp_apply wp_decodeHeader_map_shared dst input item kind dq $$ [Hdst Hs] with ⟨Hdst, Hs⟩
  · iframe Hkind Hdst Hs
  iapply HΦ
  iframe Hdst Hs

/-- DecodeHeaderFirst rejects any nonzero destination with the initialized canonical
    unsupported error, preserving that destination without reading input. -/
theorem wp_decodeHeaderFirst_nonzero_initialized (dst : Loc) (h : Header) (input : GoSlice)
    (hne : h ≠ zero_val Header) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗ dst ↦ h }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err)); ∃ kind : GoInterface,
       globalAddr cborerr.ErrUnsupported ↦□ kind ∗ dst ↦ h ∗
       err ↦ ({(zero_val cborerr.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value"}) }} := by
  wp_start_folded as Hdst
  ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
  · solve_pkg_init
  isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
  icases Hobjects with ⟨⟨%unsupported, #Hkind, _⟩, _⟩
  wp_apply wp_decodeHeaderFirst_nonzero_complete dst h input _ hne $$ [Hdst] as %err ⟨Hdst, Herr⟩
  · iframe Hkind Hdst
  iapply HΦ
  iexists _
  iframe Hkind Hdst Herr

/-- DecodeHeader rejects any nonzero destination with the initialized canonical
    unsupported error, preserving that destination without reading input. -/
theorem wp_decodeHeader_nonzero_initialized (dst : Loc) (h : Header) (input : GoSlice)
    (hne : h ≠ zero_val Header) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗ dst ↦ h }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err); ∃ kind : GoInterface,
       globalAddr cborerr.ErrUnsupported ↦□ kind ∗ dst ↦ h ∗
       err ↦ ({(zero_val cborerr.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value"}) }} := by
  wp_start_folded as Hdst
  ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
  · solve_pkg_init
  isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
  icases Hobjects with ⟨⟨%unsupported, #Hkind, _⟩, _⟩
  wp_apply wp_decodeHeader_nonzero_complete dst h input _ hne $$ [Hdst] as %err ⟨Hdst, Herr⟩
  · iframe Hkind Hdst
  iapply HΦ
  iexists _
  iframe Hkind Hdst Herr

/-- Trailing bytes cause actual cleanup and report the initialized canonical
    extraneous-data error, retaining the exact suffix length. -/
theorem wp_decodeHeader_map_trailing_initialized (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (rem : List w8) (dq : DFrac) (hmore : rem ≠ []) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} (item.bytes ++ rem) }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err);
       ∃ extraKind : GoInterface, globalAddr cborerr.ErrExtraneousData ↦□ extraKind ∗
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := extraKind, Extra' := W64 rem.length} }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave Hglobal := schema_initialized_unsupported (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobal with ⟨%kind, #Hkind⟩
  ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
  · solve_pkg_init
  isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
  icases Hobjects with ⟨_, ⟨%extra, #Hextra, _⟩⟩
  wp_apply wp_decodeHeader_map_trailing_shared dst input item rem kind _ dq hmore $$ [Hdst Hs]
    as %err ⟨Hdst, Hs, Herr⟩
  · iframe Hkind Hextra Hdst Hs
  iapply HΦ
  iexists _
  iframe Hextra Hdst Hs Herr

/-- Actual DecodeHeaderFirst for the specified lookup-failure family, obtaining
    canonical error categories from proved initialization. -/
theorem wp_decodeHeaderFirst_lookup_failure_initialized (dst : Loc) (input : GoSlice)
    (value : HeaderValue) (wire : List w8) (offset : Nat)
    (reason : HeaderLookupFailureReason) (dq : DFrac)
    (failure : HeaderLookupFailure 0 wire value offset reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)); ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗
       HeaderLookupFailureError err (W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_decodeHeaderFirst_lookup_failure_shared dst input value wire offset reason kinds dq failure $$ [Hdst Hs]
    as %err ⟨Hdst, Hs, Herr, %Hprogress⟩
  · iframe Hglobals Hdst Hs
  iapply HΦ
  iexists kinds
  iframe Hglobals Hdst Hs Herr
  ipureintro; exact Hprogress

/-- Actual DecodeHeader for the specified lookup-failure family, obtaining
    canonical error categories from proved initialization. -/
theorem wp_decodeHeader_lookup_failure_initialized (dst : Loc) (input : GoSlice)
    (value : HeaderValue) (wire : List w8) (offset : Nat)
    (reason : HeaderLookupFailureReason) (dq : DFrac)
    (failure : HeaderLookupFailure 0 wire value offset reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗
       HeaderLookupFailureError err (W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_decodeHeader_lookup_failure_shared dst input value wire offset reason kinds dq failure $$ [Hdst Hs]
    as %err ⟨Hdst, Hs, Herr, %Hprogress⟩
  · iframe Hglobals Hdst Hs
  iapply HΦ
  iexists kinds
  iframe Hglobals Hdst Hs Herr
  ipureintro; exact Hprogress

/-- Actual DecodeHeaderFirst for the specified value-failure family, obtaining
    canonical error categories from proved initialization. -/
theorem wp_decodeHeaderFirst_value_failure_initialized (dst : Loc) (input : GoSlice)
    (item : HeaderValueFailureWire 0) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)); ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderOwned dst headerZeroValue ∗ input ↦*{dq} item.bytes ∗
       HeaderValueFailureError err (W64 item.errorOffset) item.reason (headerValueFailureKind item.reason kinds) ∗
       ⌜sint.Z (W64 item.errorOffset) = item.errorOffset ∧ item.errorOffset ≤ item.bytes.length⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_decodeHeaderFirst_value_failure_shared dst input item kinds dq $$ [Hdst Hs]
    as %err ⟨Hdst, Hs, Herr, %Hprogress⟩
  · iframe Hglobals Hdst Hs
  iapply HΦ
  iexists kinds
  iframe Hglobals Hdst Hs Herr
  ipureintro; exact Hprogress

/-- Actual DecodeHeader for the specified value-failure family, obtaining
    canonical error categories from proved initialization. -/
theorem wp_decodeHeader_value_failure_initialized (dst : Loc) (input : GoSlice)
    (item : HeaderValueFailureWire 0) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderOwned dst headerZeroValue ∗ input ↦*{dq} item.bytes ∗
       HeaderValueFailureError err (W64 item.errorOffset) item.reason (headerValueFailureKind item.reason kinds) ∗
       ⌜sint.Z (W64 item.errorOffset) = item.errorOffset ∧ item.errorOffset ≤ item.bytes.length⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  wp_apply wp_decodeHeader_value_failure_shared dst input item kinds dq $$ [Hdst Hs]
    as %err ⟨Hdst, Hs, Herr, %Hprogress⟩
  · iframe Hglobals Hdst Hs
  iapply HΦ
  iexists kinds
  iframe Hglobals Hdst Hs Herr
  ipureintro; exact Hprogress

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.schema_initialized_unsupported
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_map_initialized
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_map_initialized
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_nonzero_initialized
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_nonzero_initialized
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_map_trailing_initialized
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_lookup_failure_initialized
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_lookup_failure_initialized
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_value_failure_initialized
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_value_failure_initialized
