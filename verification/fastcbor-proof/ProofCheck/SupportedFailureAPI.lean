import ProofCheck.SupportedHeaderFailureSpec
import ProofCheck.HeaderInitializedAPI

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

/- The public contracts below connect existing production-proved failure
   families to the complete independent specification. Their successful prefix
   and failure witnesses retain the restrictions of the underlying theorems. -/

/-- The actual lookup failure meets the independent First API contract. -/
theorem wp_decodeHeaderFirst_lookup_failure_spec (dst : Loc) (input : GoSlice)
    (value : HeaderValue) (wire : List w8) (offset : Nat)
    (reason : HeaderLookupFailureReason) (dq : DFrac)
    (failure : HeaderLookupFailure 0 wire value offset reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)); ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗
       HeaderLookupFailureError err (W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length ∧
         HeaderFirstSpec headerZeroValue wire (.failure headerZeroValue (.decode offset (headerLookupRecordReason reason)))⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bound : wire.length < 2^64 := by word
  have first : HeaderFirstSpec headerZeroValue wire (.failure headerZeroValue (.decode offset (headerLookupRecordReason reason))) :=
    .rejected (headerLookupFailure_rejected 0 offset wire value reason bound failure)
  wp_apply wp_decodeHeaderFirst_lookup_failure_initialized dst input value wire offset reason dq failure $$ [Hdst Hs]
    as %err ⟨%kinds, Hglobals, Hdst, Hs, Herr, %Hprogress⟩
  · iframe Hdst Hs
  iapply HΦ
  iexists kinds
  iframe Hglobals Hdst Hs Herr
  ipureintro
  exact ⟨Hprogress.1, Hprogress.2, first⟩

/-- The actual lookup failure meets the independent full API contract. -/
theorem wp_decodeHeader_lookup_failure_spec (dst : Loc) (input : GoSlice)
    (value : HeaderValue) (wire : List w8) (offset : Nat)
    (reason : HeaderLookupFailureReason) (dq : DFrac)
    (failure : HeaderLookupFailure 0 wire value offset reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderOwned dst headerZeroValue ∗ input ↦*{dq} wire ∗
       HeaderLookupFailureError err (W64 offset) reason (headerLookupFailureKind reason kinds) ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length ∧
         HeaderFullSpec headerZeroValue wire (.failure headerZeroValue (.decode offset (headerLookupRecordReason reason)))⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bound : wire.length < 2^64 := by word
  have first : HeaderFirstSpec headerZeroValue wire (.failure headerZeroValue (.decode offset (headerLookupRecordReason reason))) :=
    .rejected (headerLookupFailure_rejected 0 offset wire value reason bound failure)
  wp_apply wp_decodeHeader_lookup_failure_initialized dst input value wire offset reason dq failure $$ [Hdst Hs]
    as %err ⟨%kinds, Hglobals, Hdst, Hs, Herr, %Hprogress⟩
  · iframe Hdst Hs
  iapply HΦ
  iexists kinds
  iframe Hglobals Hdst Hs Herr
  ipureintro
  exact ⟨Hprogress.1, Hprogress.2, .firstError first⟩

/-- The actual value failure meets the independent First API contract. -/
theorem wp_decodeHeaderFirst_value_failure_spec (dst : Loc) (input : GoSlice)
    (item : HeaderValueFailureWire 0) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)); ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderOwned dst headerZeroValue ∗ input ↦*{dq} item.bytes ∗
       HeaderValueFailureError err (W64 item.errorOffset) item.reason (headerValueFailureKind item.reason kinds) ∗
       ⌜sint.Z (W64 item.errorOffset) = item.errorOffset ∧ item.errorOffset ≤ item.bytes.length ∧
         HeaderFirstSpec headerZeroValue item.bytes (.failure headerZeroValue (.decode item.errorOffset (.value item.reason)))⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bound : item.bytes.length < 2^64 := by word
  have first : HeaderFirstSpec headerZeroValue item.bytes (.failure headerZeroValue (.decode item.errorOffset (.value item.reason))) :=
    .rejected (headerValueFailureWire_rejected 0 item bound)
  wp_apply wp_decodeHeaderFirst_value_failure_initialized dst input item dq $$ [Hdst Hs]
    as %err ⟨%kinds, Hglobals, Hdst, Hs, Herr, %Hprogress⟩
  · iframe Hdst Hs
  iapply HΦ
  iexists kinds
  iframe Hglobals Hdst Hs Herr
  ipureintro
  exact ⟨Hprogress.1, Hprogress.2, first⟩

/-- The actual value failure meets the independent full API contract. -/
theorem wp_decodeHeader_value_failure_spec (dst : Loc) (input : GoSlice)
    (item : HeaderValueFailureWire 0) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderOwned dst headerZeroValue ∗ input ↦*{dq} item.bytes ∗
       HeaderValueFailureError err (W64 item.errorOffset) item.reason (headerValueFailureKind item.reason kinds) ∗
       ⌜sint.Z (W64 item.errorOffset) = item.errorOffset ∧ item.errorOffset ≤ item.bytes.length ∧
         HeaderFullSpec headerZeroValue item.bytes (.failure headerZeroValue (.decode item.errorOffset (.value item.reason)))⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bound : item.bytes.length < 2^64 := by word
  have first : HeaderFirstSpec headerZeroValue item.bytes (.failure headerZeroValue (.decode item.errorOffset (.value item.reason))) :=
    .rejected (headerValueFailureWire_rejected 0 item bound)
  wp_apply wp_decodeHeader_value_failure_initialized dst input item dq $$ [Hdst Hs]
    as %err ⟨%kinds, Hglobals, Hdst, Hs, Herr, %Hprogress⟩
  · iframe Hdst Hs
  iapply HΦ
  iexists kinds
  iframe Hglobals Hdst Hs Herr
  ipureintro
  exact ⟨Hprogress.1, Hprogress.2, .firstError first⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_lookup_failure_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_lookup_failure_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_value_failure_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_value_failure_spec
