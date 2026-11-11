import ProofCheck.StateUpdateAPIBaseFirst

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

/-- Full decoding of null and undefined returns success exactly when the
    supplied suffix is empty. Otherwise the actual wrapper clears the output
    and reports the exact trailing-byte count with its initialized category. -/
theorem wp_decodeStateUpdate_nullish_spec (dst : Loc) (input : GoSlice) (head : w8)
    (rest : List w8) (dq : DFrac) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst stateUpdateZero ∗ input ↦*{dq} (head :: rest) }}
      (App (App (Val (@! DecodeStateUpdate)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} (head :: rest) ∗ ∃ outcome : StateUpdateDecodeResult,
       ⌜StateUpdateFullSpec stateUpdateZero (head :: rest) outcome⌝ ∗ StateUpdateFullResultOwned dst result outcome }} := by
  wp_start as ⟨Howned, Hs⟩
  wp_auto
  wp_apply wp_decodeStateUpdateFirst_nullish_spec dst input head rest dq hn
    $$ [Howned Hs] as %remaining %result ⟨Hs, %Hspec, Houtcome⟩
  · iframe Howned Hs
  isimp only [StateUpdateFirstResultOwned] at Houtcome
  icases Houtcome with ⟨Howned, %consumed, %Hnext⟩
  obtain ⟨rfl, Hslice, Hprogress, Hlength, Hremaining, Hsuffix⟩ := Hnext
  wp_auto
  by_cases empty : rest = []
  · subst rest
    simp only [List.length_nil, Int.natCast_zero] at Hremaining
    have hz : remaining.len = W64 0 := by word
    simp only [hz, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hs
    iexists StateUpdateDecodeResult.success stateUpdateZero
    isplitl []
    · ipureintro; exact .success Hspec
    isimp only [StateUpdateFullResultOwned]
    iframe Howned
  · have hn : remaining.len ≠ W64 0 := by
      have hpos : 0 < rest.length := List.length_pos_iff.mpr empty
      word
    simp only [hn, _root_.decide_true]
    wp_auto
    wp_apply wp_clearStateUpdate_owned dst stateUpdateZero $$ [Howned] with Howned
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

/-- Full decoding of every zero-count map width returns success exactly when the
    supplied suffix is empty. Otherwise the actual wrapper clears the output
    and reports the exact trailing-byte count with its initialized category. -/
theorem wp_decodeStateUpdate_empty_spec (dst : Loc) (input : GoSlice) (head : w8)
    (payload rest : List w8) (dq : DFrac) (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst stateUpdateZero ∗ input ↦*{dq} ((head :: payload) ++ rest) }}
      (App (App (Val (@! DecodeStateUpdate)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} ((head :: payload) ++ rest) ∗ ∃ outcome : StateUpdateDecodeResult,
       ⌜StateUpdateFullSpec stateUpdateZero ((head :: payload) ++ rest) outcome⌝ ∗ StateUpdateFullResultOwned dst result outcome }} := by
  wp_start as ⟨Howned, Hs⟩
  wp_auto
  wp_apply wp_decodeStateUpdateFirst_empty_spec dst input head payload rest dq enc major
    $$ [Howned Hs] as %remaining %result ⟨Hs, %Hspec, Houtcome⟩
  · iframe Howned Hs
  isimp only [StateUpdateFirstResultOwned] at Houtcome
  icases Houtcome with ⟨Howned, %consumed, %Hnext⟩
  obtain ⟨rfl, Hslice, Hprogress, Hlength, Hremaining, Hsuffix⟩ := Hnext
  wp_auto
  by_cases empty : rest = []
  · subst rest
    simp only [List.length_nil, Int.natCast_zero] at Hremaining
    have hz : remaining.len = W64 0 := by word
    simp only [hz, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hs
    iexists StateUpdateDecodeResult.success stateUpdateZero
    isplitl []
    · ipureintro; exact .success Hspec
    isimp only [StateUpdateFullResultOwned]
    iframe Howned
  · have hn : remaining.len ≠ W64 0 := by
      have hpos : 0 < rest.length := List.length_pos_iff.mpr empty
      word
    simp only [hn, _root_.decide_true]
    wp_auto
    wp_apply wp_clearStateUpdate_owned dst stateUpdateZero $$ [Howned] with Howned
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
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdate_nullish_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdate_empty_spec
