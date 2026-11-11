import ProofCheck.ClassAPIGuard

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

/-- On initially nil interfaces, First realizes the independent class
    contract and returns the original unread suffix. Parse failures run actual
    cleanup and retain exact diagnostic ownership and private partial records. -/
theorem wp_decodeClassFirst_zero (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (dq : DFrac)
    (tags : CBORClassTags) (distinct : ClassTagsDistinct tags) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ interface.nil ∗ input ↦*{dq} wire }}
      (App (App (App (Val (@! DecodeClassDefinitionFirst)) (Val #input)) (Val #dst)) (Val #tags))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : ClassFirstResult,
       ⌜ClassFirstSpec tags interface.nil wire outcome⌝ ∗
       ClassFirstResultOwned dst input remaining wire result outcome }} := by
  wp_start as ⟨Hdst, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have hz (x : w64) : (zero_val cursor.Cursor).pos' + x = x := by change (0 : w64) + x = x; word
  wp_auto
  wp_apply wp_newCursor input
  wp_apply wp_readClass_complete validator c_ptr dst interface.nil tags
    {(zero_val cursor.Cursor) with buf' := input} wire wire dq 0 distinct rfl (by decide) hp hb
    $$ [c Hdst Hs] as %result ⟨Hs, %outcome, %Hspec, Houtcome⟩
  · iframe c Hdst Hs
  cases outcome with
  | success final rest =>
    isimp only [ClassReadOwned] at Houtcome
    icases Houtcome with ⟨%out, %consumed, Hdst, Hchild, Hc, %Hnext⟩
    obtain ⟨rfl, Hlength, Hprogress, Hbound, Hsuffix⟩ := Hnext
    simp only [hz, show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] at Hprogress Hbound Hsuffix
    isimp only [hz] at Hc
    wp_auto
    wp_apply wp_remaining c_ptr {(zero_val cursor.Cursor) with buf' := input, pos' := W64 consumed}
      ⟨by change 0 ≤ sint.Z (W64 consumed); rw [Hprogress]; omega, Hbound⟩ Hcap.2 $$ [Hc] with Hc
    · iframe Hc
    iapply HΦ
    iframe Hs
    iexists ClassFirstResult.success final rest
    isplitl []
    · ipureintro; exact .success Hspec.1
    isimp only [ClassFirstResultOwned]
    isplitl [Hdst Hchild]
    · unfold ClassValueDestination
      iexists out
      iframe Hdst Hchild
    iexists consumed
    ipureintro
    refine ⟨trivial, rfl, Hprogress, Hlength, ?_, Hsuffix⟩
    simp only [slice.slice]
    have hlen : (wire.length : Int) = sint.Z input.len := by
      simp only [sint.nat, sint.Z] at Hlen ⊢
      omega
    word
  | failure detail consumed offset extraDepth =>
    isimp only [ClassReadOwned] at Houtcome
    icases Houtcome with ⟨Hdst, Hc, Herr, %Hnext⟩
    isimp only [hz] at Herr
    cases returnShape : result with
    | nil => exact False.elim (Hnext.1 returnShape)
    | ok info =>
      wp_auto
      wp_apply wp_clearClass dst interface.nil $$ [Hdst] with Hdst
      · iframe Hdst
      iapply HΦ
      iframe Hs
      iexists ClassFirstResult.failure interface.nil (.decode offset detail)
      isplitl []
      · ipureintro; exact .rejected Hspec.1
      isimp only [ClassFirstResultOwned, ClassAPIErrorOwned]
      iframe Hdst Herr
      ipureintro
      simpa only [hz, show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] using Hnext.2.2.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeClassFirst_zero
