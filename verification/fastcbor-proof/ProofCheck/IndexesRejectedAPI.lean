import ProofCheck.IndexesRejected
import ProofCheck.IndexesValidAPI

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

/-- Every specified parse failure clears partial output, returns a nil suffix
    and retains the exact diagnostic or bare sentinel at the public First API. -/
theorem wp_decodeIndexesFirst_rejected (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (final : IndexesValue) (dq : DFrac)
    (consumed offset extraDepth : Nat) (reason : IndexesRecordFailureReason) (k : BigIntErrorKinds) (depthKind : Loc)
    (failure : IndexesRecordRejected 0 indexesZeroValue wire final consumed offset extraDepth reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ∗
       dst ↦ (zero_val BlockTransactionsIndexes) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeBlockTransactionsIndexesFirst)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET (PairV #slice.nil #result);
       IndexesOwned dst indexesZeroValue ∗ input ↦*{dq} wire ∗ IndexesRecordError result (W64 offset) reason k depthKind ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start as ⟨#Hglobals, #Hdepth, Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have hz (x : w64) : W64 0 + x = x := by word
  wp_auto
  wp_apply wp_zeroIndexes_complete dst (zero_val BlockTransactionsIndexes) $$ [Hdst] as %guard ⟨Hdst, %Hzero⟩
  · iframe Hdst
  have hg : guard = true := Hzero.mpr rfl
  subst guard
  wp_auto
  wp_apply wp_newCursor input
  ihave Hchildren := indexesChildren_zero (GF := GF)
  wp_apply wp_readIndexes_rejected validator _ dst (zero_val BlockTransactionsIndexes) indexesZeroValue
    {(zero_val cursor.Cursor) with buf' := input} wire wire dq 0 consumed offset extraDepth final reason k depthKind
    failure rfl (by omega) hp hb $$ [c Hdst Hchildren Hs] as %result %out ⟨Hdst, Hchildren, Hc, Hs, Herr, %Hnext⟩
  · iframe Hglobals Hdepth c Hdst Hchildren Hs
  isimp only [show (zero_val cursor.Cursor).pos' = W64 0 by rfl, hz] at Herr
  ihave %Hshape := indexesRecordError_shape result (W64 offset) reason k depthKind $$ Herr
  rcases Hshape with heq | ⟨err, heq⟩
  all_goals
    subst result
    try simp only [bigIntCategory]
    wp_auto
    wp_apply wp_clearIndexes_owned dst out final $$ [Hdst Hchildren] with Hdst
    · iframe Hdst Hchildren
    iapply HΦ
    iframe Hdst Hs Herr
    ipureintro
    have bounds := indexesRecordRejected_bounds 0 consumed offset extraDepth indexesZeroValue final wire reason (by omega) failure
    refine ⟨?_, bounds.2.1⟩
    simpa only [show (zero_val cursor.Cursor).pos' = W64 0 by rfl, hz, show sint.Z (W64 0) = 0 by rfl, Int.zero_add] using Hnext.2.1

/-- The full API preserves parse errors and the cleared destination. It does
    not reinterpret a parser error as trailing data. -/
theorem wp_decodeIndexes_rejected (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (final : IndexesValue) (dq : DFrac)
    (consumed offset extraDepth : Nat) (reason : IndexesRecordFailureReason) (k : BigIntErrorKinds) (depthKind : Loc)
    (failure : IndexesRecordRejected 0 indexesZeroValue wire final consumed offset extraDepth reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ∗
       dst ↦ (zero_val BlockTransactionsIndexes) ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeBlockTransactionsIndexes)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       IndexesOwned dst indexesZeroValue ∗ input ↦*{dq} wire ∗ IndexesRecordError result (W64 offset) reason k depthKind ∗
       ⌜sint.Z (W64 offset) = offset ∧ offset ≤ wire.length⌝ }} := by
  wp_start as ⟨#Hglobals, #Hdepth, Hdst, Hs⟩
  wp_auto
  wp_apply wp_decodeIndexesFirst_rejected validator dst input wire final dq consumed offset extraDepth reason k depthKind failure
    $$ [Hdst Hs] as %result ⟨Hdst, Hs, Herr, %Hoffset⟩
  · iframe Hglobals Hdepth Hdst Hs
  ihave %Hshape := indexesRecordError_shape result (W64 offset) reason k depthKind $$ Herr
  rcases Hshape with heq | ⟨err, heq⟩
  all_goals
    subst result
    try simp only [bigIntCategory]
    wp_auto
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexesFirst_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexes_rejected
