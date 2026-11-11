import ProofCheck.IndexesValid

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

/-- Every specified successful record passes the actual First API, yielding
    owned output and the exact original input suffix. -/
theorem wp_decodeIndexesFirst_valid (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire rem : List w8) (final : IndexesValue) (dq : DFrac)
    (valid : IndexesRecordValid 0 indexesZeroValue wire final) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val BlockTransactionsIndexes) ∗ input ↦*{dq} (wire ++ rem) }}
      (App (App (Val (@! DecodeBlockTransactionsIndexesFirst)) (Val #input)) (Val #dst))
    {{ RET (PairV #(slice.slice input w8 (W64 wire.length) input.len) #interface.nil);
       IndexesOwned dst final ∗ input ↦*{dq} (wire ++ rem) ∗
       ⌜sint.Z (input.len - W64 wire.length) = rem.length ∧
         subslice (sint.nat (W64 wire.length)) (sint.nat input.len) (wire ++ rem) = rem⌝ }} := by
  wp_start as ⟨Hdst, Hs⟩
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
  wp_apply wp_zeroIndexes_complete dst (zero_val BlockTransactionsIndexes) $$ [Hdst] as %result ⟨Hdst, %Hzero⟩
  · iframe Hdst
  have hr : result = true := Hzero.mpr rfl
  subst result
  wp_auto
  wp_apply wp_newCursor input
  ihave Hchildren := indexesChildren_zero (GF := GF)
  wp_apply wp_readIndexes_valid validator _ dst (zero_val BlockTransactionsIndexes) indexesZeroValue
    {(zero_val cursor.Cursor) with buf' := input} (wire ++ rem) wire rem final dq 0 valid rfl (by omega) hp hb
    $$ [c Hdst Hchildren Hs] as %out ⟨Hdst, Hchildren, Hc, Hs, %Hnext⟩
  · iframe c Hdst Hchildren Hs
  ieval (change (c_ptr ↦ {(zero_val cursor.Cursor) with buf' := input, pos' := W64 0 + W64 wire.length} : IProp GF) ⊢ _) at Hc
  isimp only [hposition] at Hc
  wp_apply wp_remaining _ {(zero_val cursor.Cursor) with buf' := input, pos' := W64 wire.length}
    (by change 0 ≤ sint.Z (W64 wire.length) ∧ sint.Z (W64 wire.length) ≤ sint.Z input.len; word)
    Hcap.2 $$ [Hc] with Hc
  · iframe Hc
  iapply HΦ
  iframe Hs
  isplitl [Hdst Hchildren]
  · unfold IndexesOwned
    iexists out
    iframe Hdst Hchildren
  ipureintro
  constructor
  · simp only [List.length_append, sint.nat, sint.Z] at Hlen
    word
  · simpa only [show (zero_val cursor.Cursor).pos' = W64 0 by rfl, hposition] using Hnext.2.2

/-- Full decoding accepts exactly the same successful record without a suffix. -/
theorem wp_decodeIndexes_valid (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (final : IndexesValue) (dq : DFrac)
    (valid : IndexesRecordValid 0 indexesZeroValue wire final) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ (zero_val BlockTransactionsIndexes) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeBlockTransactionsIndexes)) (Val #input)) (Val #dst))
    {{ RET #interface.nil; IndexesOwned dst final ∗ input ↦*{dq} wire }} := by
  wp_start as ⟨Hdst, Hs⟩
  wp_auto
  wp_apply wp_decodeIndexesFirst_valid validator dst input wire [] final dq valid $$ [Hdst Hs] with ⟨Hdst, Hs, %Hrest⟩
  · isimp only [List.append_nil]
    iframe Hdst Hs
  have hz : input.len - W64 wire.length = W64 0 := by
    have := Hrest.1
    simp only [List.length_nil, Int.natCast_zero] at this
    word
  simp only [slice.slice, hz]
  wp_auto
  isimp only [List.append_nil] at Hs
  wp_end

/-- Trailing-data rejection clears every field after any successful record and
    reports the exact unread count while preserving input ownership. -/
theorem wp_decodeIndexes_valid_trailing (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire rem : List w8) (final : IndexesValue) (dq : DFrac) (extraKind : GoInterface)
    (valid : IndexesRecordValid 0 indexesZeroValue wire final) (hmore : rem ≠ []) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cborerr.ErrExtraneousData ↦□ extraKind ∗
       dst ↦ (zero_val BlockTransactionsIndexes) ∗ input ↦*{dq} (wire ++ rem) }}
      (App (App (Val (@! DecodeBlockTransactionsIndexes)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err);
       IndexesOwned dst indexesZeroValue ∗ input ↦*{dq} (wire ++ rem) ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := extraKind, Extra' := W64 rem.length} }} := by
  wp_start as ⟨#Hextra, Hdst, Hs⟩
  wp_auto
  wp_apply wp_decodeIndexesFirst_valid validator dst input wire rem final dq valid $$ [Hdst Hs] with ⟨Hdst, Hs, %Hrest⟩
  · iframe Hdst Hs
  have hpositive := List.length_pos_iff.mpr hmore
  have hcount : input.len - W64 wire.length = W64 rem.length := by have := Hrest.1; word
  have hn : input.len - W64 wire.length ≠ W64 0 := by have := Hrest.1; word
  simp only [slice.slice, hn, decide_false]
  wp_auto
  isimp only [IndexesOwned] at Hdst
  icases Hdst with ⟨%h, Hdst, Hchildren⟩
  wp_apply wp_clearIndexes_owned dst h final $$ [Hdst Hchildren] with Hdst
  · iframe Hdst Hchildren
  wp_alloc err as Herr
  wp_auto
  isimp only [hcount] at Herr
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexesFirst_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexes_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexes_valid_trailing
