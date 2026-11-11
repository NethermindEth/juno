import ProofCheck.IndexesOwnership

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

/-- The generated guard checks nilness, so allocated empty arrays are nonzero. -/
theorem wp_zeroIndexes_complete (dst : Loc) (h : BlockTransactionsIndexes) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗ dst ↦ h }}
      (App (Val (@! cborZeroBlockTransactionsIndexes)) (Val #dst))
    {{ (result : Bool), RET #result; dst ↦ h ∗ ⌜result = true ↔ h = zero_val BlockTransactionsIndexes⌝ }} := by
  wp_start as Hdst
  wp_auto
  by_cases ht : h.Transactions' = slice.nil
  · simp only [ht, _root_.decide_true]
    wp_auto
    by_cases hr : h.Receipts' = slice.nil
    · simp only [hr, _root_.decide_true]
      have hz : h = zero_val BlockTransactionsIndexes := by cases h; simp_all; rfl
      iapply HΦ
      iframe Hdst
      ipureintro
      simp [hz]
    · simp only [hr, _root_.decide_false]
      have hn : h ≠ zero_val BlockTransactionsIndexes := by intro hz; subst h; exact hr rfl
      iapply HΦ
      iframe Hdst
      ipureintro
      simp [hn]
  · simp only [ht, _root_.decide_false]
    wp_auto
    have hn : h ≠ zero_val BlockTransactionsIndexes := by intro hz; subst h; exact ht rfl
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]

/-- Cleanup clears both slice headers for any partial result. -/
theorem wp_clearIndexes (dst : Loc) (h : BlockTransactionsIndexes) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗ dst ↦ h }}
      (App (Val (@! cborClearBlockTransactionsIndexes)) (Val #dst))
    {{ RET #(); dst ↦ (zero_val BlockTransactionsIndexes) }} := by
  wp_start as Hdst
  wp_auto
  wp_end

/-- A nonzero destination is rejected before reading the input. -/
theorem wp_decodeIndexesFirst_nonzero (dst : Loc) (h : BlockTransactionsIndexes) (input : GoSlice)
    (kind : GoInterface) (hne : h ≠ zero_val BlockTransactionsIndexes) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cborerr.ErrUnsupported ↦□ kind ∗ dst ↦ h }}
      (App (App (Val (@! DecodeBlockTransactionsIndexesFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err));
       dst ↦ h ∗ err ↦ ({ (zero_val cborerr.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value" }) }} := by
  wp_start as ⟨#Hkind, Hdst⟩
  wp_auto
  wp_apply wp_zeroIndexes_complete dst h $$ [Hdst] as %result ⟨Hdst, %Hzero⟩
  · iframe Hdst
  have hr : result = false := by cases result <;> simp_all
  subst result
  wp_auto
  wp_alloc err as Herr
  wp_auto
  wp_end

/-- The full API preserves a nonzero destination and returns the same guard error. -/
theorem wp_decodeIndexes_nonzero (dst : Loc) (h : BlockTransactionsIndexes) (input : GoSlice)
    (kind : GoInterface) (hne : h ≠ zero_val BlockTransactionsIndexes) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cborerr.ErrUnsupported ↦□ kind ∗ dst ↦ h }}
      (App (App (Val (@! DecodeBlockTransactionsIndexes)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err);
       dst ↦ h ∗ err ↦ ({ (zero_val cborerr.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value" }) }} := by
  wp_start as ⟨#Hkind, Hdst⟩
  wp_auto
  wp_apply wp_decodeIndexesFirst_nonzero dst h input kind hne $$ [Hdst] as %err ⟨Hdst, Herr⟩
  · iframe Hkind Hdst
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroIndexes_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearIndexes
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexesFirst_nonzero

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeIndexes_nonzero
