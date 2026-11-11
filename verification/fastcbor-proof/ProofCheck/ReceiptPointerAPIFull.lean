import ProofCheck.ReceiptPointerAPIComplete
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

/-- Full value decoding composes First, preserving guard/parser errors and
    clearing otherwise successful output before reporting trailing bytes. -/
theorem wp_decodeReceiptPointer_complete (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (old : Option ReceiptValue) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       ReceiptPointerDestination dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeTransactionReceiptPointer)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : ReceiptPointerDecodeResult,
       ⌜ReceiptPointerFullSpec old wire outcome⌝ ∗ ReceiptPointerFullResultOwned dst result outcome }} := by
  wp_start as ⟨Howned, Hs⟩
  wp_auto
  wp_apply wp_decodeReceiptPointerFirst_complete validator dst old input wire dq $$ [Howned Hs]
    as %remaining %result ⟨Hs, %outcome, %Hspec, Houtcome⟩
  · iframe Howned Hs
  cases outcome with
  | failure final error =>
    isimp only [ReceiptPointerFirstResultOwned] at Houtcome
    icases Houtcome with ⟨Howned, Herr, %Hremaining⟩
    ihave %HnonNil := receiptPointerAPIErrorOwned_nonNil result error $$ Herr
    cases returnShape : result with
    | nil => exact False.elim (HnonNil returnShape)
    | ok info =>
      wp_auto
      iapply HΦ
      iframe Hs
      iexists ReceiptPointerDecodeResult.failure final error
      isplitl []
      · ipureintro; exact .firstError Hspec
      isimp only [ReceiptPointerFullResultOwned]
      iframe Howned Herr
  | success final rest =>
    isimp only [ReceiptPointerFirstResultOwned] at Houtcome
    icases Houtcome with ⟨Howned, %consumed, %Hnext⟩
    obtain ⟨rfl, Hslice, Hprogress, Hlength, Hremaining, Hsuffix⟩ := Hnext
    wp_auto
    by_cases empty : rest = []
    · simp only [empty, List.length_nil, Int.natCast_zero] at Hremaining Hspec
      have hz : remaining.len = W64 0 := by word
      simp only [hz, _root_.decide_false]
      wp_auto
      iapply HΦ
      iframe Hs
      iexists ReceiptPointerDecodeResult.success final
      isplitl []
      · ipureintro; exact .success Hspec
      isimp only [ReceiptPointerFullResultOwned]
      iframe Howned
    · have hn : remaining.len ≠ W64 0 := by
        have hpos : 0 < rest.length := List.length_pos_iff.mpr empty
        word
      simp only [hn, _root_.decide_true]
      wp_auto
      wp_apply wp_clearReceiptPointer_owned dst final $$ [Howned] with Howned
      · iframe Howned
      ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
      · solve_pkg_init
      isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
      icases Hobjects with ⟨_, ⟨%extra, #Hextra, _⟩⟩
      wp_auto
      wp_alloc err as Herr
      wp_auto
      iapply HΦ
      iframe Hs
      iexists ReceiptPointerDecodeResult.failure none (.trailing rest.length)
      isplitl []
      · ipureintro; exact .trailing Hspec empty
      isimp only [ReceiptPointerFullResultOwned, ReceiptPointerAPIErrorOwned]
      iframe Howned
      iexists _, err
      have heq : remaining.len = W64 rest.length := by word
      isimp only [heq] at Herr
      iframe Hextra Herr
      ipureintro
      exact ⟨rfl, by word⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptPointer_complete
