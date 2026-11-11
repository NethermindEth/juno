import ProofCheck.StateUpdateScalarAPIFirst

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

/-- Full decoding of every valid scalar-only StateUpdate record. The actual
    trailing-data path clears populated felt fields as well as zero values;
    the independent full API spec and owned diagnostics are preserved. -/
theorem wp_decodeStateUpdate_scalar_valid (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire item rest : List w8) (final : StateUpdateValue) (dq : DFrac)
    (split : wire = item ++ rest) (valid : StateUpdateScalarRecordValid 0 stateUpdateZero item rest final) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst stateUpdateZero ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeStateUpdate)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : StateUpdateDecodeResult,
       ⌜StateUpdateFullSpec stateUpdateZero wire outcome⌝ ∗ StateUpdateFullResultOwned dst result outcome }} := by
  wp_start as ⟨Howned, Hs⟩
  wp_auto
  wp_apply wp_decodeStateUpdateFirst_scalar_valid validator dst input wire item rest final dq split valid
    $$ [Howned Hs] as %remaining %result ⟨Hs, %Hspec, Houtcome⟩
  · iframe Howned Hs
  isimp only [StateUpdateFirstResultOwned] at Houtcome
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
    iexists StateUpdateDecodeResult.success final
    isplitl []
    · ipureintro; exact .success Hspec
    isimp only [StateUpdateFullResultOwned]
    iframe Howned
  · have hn : remaining.len ≠ W64 0 := by
      have hpos : 0 < rest.length := List.length_pos_iff.mpr empty
      word
    simp only [hn, _root_.decide_true]
    wp_auto
    wp_apply wp_clearStateUpdate_owned dst final $$ [Howned] with Howned
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
    iexists StateUpdateDecodeResult.failure stateUpdateZero (.trailing rest.length)
    isplitl []
    · ipureintro; exact .trailing Hspec empty
    isimp only [StateUpdateFullResultOwned, StateUpdateAPIErrorOwned]
    iframe Howned
    iexists _, err
    have heq : remaining.len = W64 rest.length := by word
    isimp only [heq] at Herr
    iframe Hextra Herr
    ipureintro
    exact ⟨rfl, by word⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdate_scalar_valid
