import ProofCheck.HeaderZero

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

/-- An owned felt is nil exactly when its abstract value is absent. In
    particular an allocated zero felt is not an absent value. -/
theorem feltResult_null_iff (p : Loc) (value : Option (GoArray w64 4)) :
    FeltResult (GF := GF) p value ⊢ ⌜p = null ↔ value = none⌝ := by
  cases value with
  | none =>
    unfold FeltResult
    iintro %hp
    ipureintro
    simp [hp]
  | some words =>
    unfold FeltResult
    iintro ⟨_, %hp⟩
    ipureintro
    simp [hp]

/-- Nil signatures differ from an allocated empty signature slice. -/
theorem signaturesResult_nil_iff (out : GoSlice) (value : Option (List SignatureValue)) :
    SignaturesResult (GF := GF) out value ⊢ ⌜out = slice.nil ↔ value = none⌝ := by
  cases value with
  | none =>
    unfold SignaturesResult
    iintro %hs
    ipureintro
    simp [hs]
  | some values =>
    unfold SignaturesResult SignaturesOwned
    iintro ⟨%slices, _, _, %hn⟩
    have hs : out ≠ slice.nil := by intro h; subst out; exact hn rfl
    ipureintro
    simp [hs]

/-- Owned GasPrice storage is nonnull even when both of its amounts are nil. -/
theorem gasPriceResult_null_iff (p : Loc) (value : Option GasPriceValue) :
    GasPriceResult (GF := GF) p value ⊢ ⌜p = null ↔ value = none⌝ := by
  cases value with
  | none =>
    unfold GasPriceResult
    iintro %hp
    ipureintro
    simp [hp]
  | some value =>
    unfold GasPriceResult GasPriceOwned
    iintro ⟨%g, Hg, _⟩
    ihave %hn := typedPointsto_not_null p g (DFrac.own 1) $$ Hg
    ipureintro
    simp [hn]

/-- The concrete generated guard's zero test agrees exactly with the independent
    application value under the caller-visible ownership relation. -/
theorem headerChildren_zero_iff (h : Header) (value : HeaderValue) :
    HeaderChildren (GF := GF) h value ⊢ ⌜h = zero_val Header ↔ value = headerZeroValue⌝ := by
  unfold HeaderChildren HeaderBody
  iintro ⟨⟨%Hscalars, Hhash, Hparent, Hroot, Hsequencer, Heth, Hsignatures, Hstrk⟩, Hdata, Hl2⟩
  ihave %Hhash := feltResult_null_iff h.Hash' value.hash $$ Hhash
  ihave %Hparent := feltResult_null_iff h.ParentHash' value.parentHash $$ Hparent
  ihave %Hroot := feltResult_null_iff h.GlobalStateRoot' value.globalStateRoot $$ Hroot
  ihave %Hsequencer := feltResult_null_iff h.SequencerAddress' value.sequencerAddress $$ Hsequencer
  ihave %Heth := feltResult_null_iff h.L1GasPriceETH' value.l1GasPriceETH $$ Heth
  ihave %Hsignatures := signaturesResult_nil_iff h.Signatures' value.signatures $$ Hsignatures
  ihave %Hstrk := feltResult_null_iff h.L1GasPriceSTRK' value.l1GasPriceSTRK $$ Hstrk
  ihave %Hdata := gasPriceResult_null_iff h.L1DataGasPrice' value.l1DataGasPrice $$ Hdata
  ihave %Hl2 := gasPriceResult_null_iff h.L2GasPrice' value.l2GasPrice $$ Hl2
  ipureintro
  constructor
  · intro hz
    subst h
    have hh := Hhash.mp rfl
    have hp := Hparent.mp rfl
    have hr := Hroot.mp rfl
    have hseq := Hsequencer.mp rfl
    have heth := Heth.mp rfl
    have hsig := Hsignatures.mp rfl
    have hstrk := Hstrk.mp rfl
    have hdata := Hdata.mp rfl
    have hl2 := Hl2.mp rfl
    cases value
    simp_all [HeaderScalars, headerZeroValue]
    exact ⟨Hscalars.1.symm, Hscalars.2.1.symm, Hscalars.2.2.1.symm,
      Hscalars.2.2.2.1.symm, Hscalars.2.2.2.2.1.symm, Hscalars.2.2.2.2.2.symm⟩
  · intro hz
    subst value
    have hh := Hhash.mpr rfl
    have hp := Hparent.mpr rfl
    have hr := Hroot.mpr rfl
    have hseq := Hsequencer.mpr rfl
    have heth := Heth.mpr rfl
    have hsig := Hsignatures.mpr rfl
    have hstrk := Hstrk.mpr rfl
    have hdata := Hdata.mpr rfl
    have hl2 := Hl2.mpr rfl
    cases h
    simp_all [HeaderScalars, headerZeroValue]
    rfl


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltResult_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.signaturesResult_nil_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceResult_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.headerChildren_zero_iff
