import ProofCheck.SupportedSpec
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

/-- The existing production-proved map family meets the complete independent
    First specification, with actual output ownership and exact suffix aliasing.
    HeaderMapWire retains the explicit key/nullish-string restrictions. -/
theorem wp_decodeHeaderFirst_map_spec (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (rem : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} (item.bytes ++ rem) }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ RET (PairV #(slice.slice input w8 (W64 item.bytes.length) input.len) #interface.nil);
       HeaderOwned dst item.value ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       ⌜sint.Z (input.len - W64 item.bytes.length) = rem.length ∧
         subslice (sint.nat (W64 item.bytes.length)) (sint.nat input.len) (item.bytes ++ rem) = rem ∧
         HeaderFirstSpec headerZeroValue (item.bytes ++ rem) (.success item.value rem)⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bound : item.bytes.length < 2^64 := by
    simp only [List.length_append] at Hlen
    word
  have spec : HeaderFirstSpec headerZeroValue (item.bytes ++ rem) (.success item.value rem) :=
    .success (headerMapWire_valid 0 item bound)
  wp_apply wp_decodeHeaderFirst_map_initialized dst input item rem dq $$ [Hdst Hs] with ⟨Hdst, Hs, %Hrest⟩
  · iframe Hdst Hs
  iapply HΦ
  iframe Hdst Hs
  ipureintro
  exact ⟨Hrest.1, Hrest.2, spec⟩

/-- Full decoding of the production-proved map family satisfies the complete
    specification and preserves the original input ownership. -/
theorem wp_decodeHeader_map_spec (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} item.bytes }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ RET #interface.nil; HeaderOwned dst item.value ∗ input ↦*{dq} item.bytes ∗
       ⌜HeaderFullSpec headerZeroValue item.bytes (.success item.value)⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bound : item.bytes.length < 2^64 := by word
  have first : HeaderFirstSpec headerZeroValue item.bytes (.success item.value []) := by
    simpa only [List.append_nil] using
      (HeaderFirstSpec.success (rest := []) (headerMapWire_valid 0 item bound))
  wp_apply wp_decodeHeader_map_initialized dst input item dq $$ [Hdst Hs] with ⟨Hdst, Hs⟩
  · iframe Hdst Hs
  iapply HΦ
  iframe Hdst Hs
  ipureintro
  exact .success first

/-- Nonempty suffixes after a supported successful map satisfy the complete
    trailing-error specification, including actual cleanup and owned diagnostics. -/
theorem wp_decodeHeader_map_trailing_spec (dst : Loc) (input : GoSlice)
    (item : HeaderMapWire 0) (rem : List w8) (dq : DFrac) (hmore : rem ≠ []) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       dst ↦ (zero_val Header) ∗ input ↦*{dq} (item.bytes ++ rem) }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err);
       ∃ extraKind : GoInterface, globalAddr cborerr.ErrExtraneousData ↦□ extraKind ∗
       HeaderOwned dst headerZeroValue ∗ input ↦*{dq} (item.bytes ++ rem) ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := extraKind, Extra' := W64 rem.length} ∗
       ⌜HeaderFullSpec headerZeroValue (item.bytes ++ rem) (.failure headerZeroValue (.trailing rem.length))⌝ }} := by
  wp_start_folded as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bound : item.bytes.length < 2^64 := by
    simp only [List.length_append] at Hlen
    word
  have first : HeaderFirstSpec headerZeroValue (item.bytes ++ rem) (.success item.value rem) :=
    .success (headerMapWire_valid 0 item bound)
  wp_apply wp_decodeHeader_map_trailing_initialized dst input item rem dq hmore $$ [Hdst Hs]
    as %err ⟨%kind, #Hkind, Hdst, Hs, Herr⟩
  · iframe Hdst Hs
  iapply HΦ
  iexists kind
  iframe Hkind Hdst Hs Herr
  ipureintro
  exact .trailing first hmore

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_map_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_map_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_map_trailing_spec
