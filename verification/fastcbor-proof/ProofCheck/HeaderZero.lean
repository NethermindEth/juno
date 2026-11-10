import ProofCheck.HeaderOwnership

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

/-- The generated guard checks all fifteen actual fields, including nil rather
    than merely empty signatures. This contract is independent of input bytes. -/
theorem wp_zeroHeader_complete (dst : Loc) (h : Header) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗ dst ↦ h }}
      (App (Val (@! cborZeroHeader)) (Val #dst))
    {{ (result : Bool), RET #result; dst ↦ h ∗ ⌜result = true ↔ h = zero_val Header⌝ }} := by
  wp_start as Hdst
  wp_auto
  by_cases h0 : h.Hash' ≠ null
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h0 rfl
    simp only [h0, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h0' : h.Hash' = null := Classical.not_not.mp h0
  simp only [h0', _root_.decide_true]
  wp_auto
  by_cases h1 : h.ParentHash' ≠ null
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h1 rfl
    simp only [h1, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h1' : h.ParentHash' = null := Classical.not_not.mp h1
  simp only [h1', _root_.decide_true]
  wp_auto
  by_cases h2 : h.Number' ≠ W64 0
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h2 rfl
    simp only [h2, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h2' : h.Number' = W64 0 := Classical.not_not.mp h2
  simp only [h2', _root_.decide_true]
  wp_auto
  by_cases h3 : h.GlobalStateRoot' ≠ null
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h3 rfl
    simp only [h3, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h3' : h.GlobalStateRoot' = null := Classical.not_not.mp h3
  simp only [h3', _root_.decide_true]
  wp_auto
  by_cases h4 : h.SequencerAddress' ≠ null
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h4 rfl
    simp only [h4, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h4' : h.SequencerAddress' = null := Classical.not_not.mp h4
  simp only [h4', _root_.decide_true]
  wp_auto
  by_cases h5 : h.TransactionCount' ≠ W64 0
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h5 rfl
    simp only [h5, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h5' : h.TransactionCount' = W64 0 := Classical.not_not.mp h5
  simp only [h5', _root_.decide_true]
  wp_auto
  by_cases h6 : h.EventCount' ≠ W64 0
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h6 rfl
    simp only [h6, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h6' : h.EventCount' = W64 0 := Classical.not_not.mp h6
  simp only [h6', _root_.decide_true]
  wp_auto
  by_cases h7 : h.Timestamp' ≠ W64 0
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h7 rfl
    simp only [h7, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h7' : h.Timestamp' = W64 0 := Classical.not_not.mp h7
  simp only [h7', _root_.decide_true]
  wp_auto
  by_cases h8 : h.ProtocolVersion' ≠ []
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h8 rfl
    simp only [h8, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h8' : h.ProtocolVersion' = [] := Classical.not_not.mp h8
  simp only [h8', _root_.decide_true]
  wp_auto
  by_cases h9 : h.L1GasPriceETH' ≠ null
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h9 rfl
    simp only [h9, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h9' : h.L1GasPriceETH' = null := Classical.not_not.mp h9
  simp only [h9', _root_.decide_true]
  wp_auto
  by_cases h10 : h.Signatures' ≠ slice.nil
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h10 rfl
    simp only [h10, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h10' : h.Signatures' = slice.nil := Classical.not_not.mp h10
  simp only [h10', _root_.decide_true]
  wp_auto
  by_cases h11 : h.L1GasPriceSTRK' ≠ null
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h11 rfl
    simp only [h11, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h11' : h.L1GasPriceSTRK' = null := Classical.not_not.mp h11
  simp only [h11', _root_.decide_true]
  wp_auto
  by_cases h12 : h.L1DAMode' ≠ W64 0
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h12 rfl
    simp only [h12, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h12' : h.L1DAMode' = W64 0 := Classical.not_not.mp h12
  simp only [h12', _root_.decide_true]
  wp_auto
  by_cases h13 : h.L1DataGasPrice' ≠ null
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h13 rfl
    simp only [h13, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h13' : h.L1DataGasPrice' = null := Classical.not_not.mp h13
  simp only [h13', _root_.decide_true]
  wp_auto
  by_cases h14 : h.L2GasPrice' ≠ null
  · have hn : h ≠ zero_val Header := by
      intro heq
      subst h
      exact h14 rfl
    simp only [h14, _root_.decide_false]
    iapply HΦ
    iframe Hdst
    ipureintro
    simp [hn]
  have h14' : h.L2GasPrice' = null := Classical.not_not.mp h14
  simp only [h14', _root_.decide_true]
  have heq : h = zero_val Header := by
    cases h
    simp_all
    rfl
  iapply HΦ
  iframe Hdst
  ipureintro
  simp [heq]

/-- Any nonzero Header is rejected unchanged, regardless of which field makes
    it nonzero. The input need not be owned because the API never reads it. -/
theorem wp_decodeHeaderFirst_nonzero_complete (dst : Loc) (h : Header) (input : GoSlice)
    (kind : GoInterface) (hne : h ≠ zero_val Header) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr fastcbor.ErrUnsupported ↦□ kind ∗ dst ↦ h }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType fastcbor.DecodeError.ty) #err));
       dst ↦ h ∗ err ↦ ({ (zero_val fastcbor.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value" }) }} := by
  wp_start as ⟨#Hkind, Hdst⟩
  wp_auto
  wp_apply wp_zeroHeader_complete dst h $$ [Hdst] as %result ⟨Hdst, %Hzero⟩
  · iframe Hdst
  have hr : result = false := by cases result <;> simp_all
  subst result
  wp_auto
  wp_alloc err as Herr
  wp_auto
  wp_end

/-- The full-input API also preserves every nonzero destination and returns
    the same rejection, without reading input or running failure cleanup. -/
theorem wp_decodeHeader_nonzero_complete (dst : Loc) (h : Header) (input : GoSlice)
    (kind : GoInterface) (hne : h ≠ zero_val Header) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr fastcbor.ErrUnsupported ↦□ kind ∗ dst ↦ h }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType fastcbor.DecodeError.ty) #err);
       dst ↦ h ∗ err ↦ ({ (zero_val fastcbor.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value" }) }} := by
  wp_start as ⟨#Hkind, Hdst⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_nonzero_complete dst h input kind hne $$ [Hdst] as %err ⟨Hdst, Herr⟩
  · iframe Hkind Hdst
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroHeader_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_nonzero_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_nonzero_complete
