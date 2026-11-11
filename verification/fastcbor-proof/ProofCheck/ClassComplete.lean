import ProofCheck.ClassOwnership

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

/-- The actual dispatcher realizes each independently specified outcome. A
    failed payload remains private and never replaces the caller's interface. -/
theorem wp_readClass_spec (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoInterface) (tags : CBORClassTags) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (raw : ClassDispatchResult)
    (distinct : ClassTagsDistinct tags) (spec : ClassDispatchSpec d tags wire raw)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadClassDefinition)) (Val #c)) (Val #dst)) (Val #tags))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ClassReadOwned c dst old v xs wire result (classResult raw) }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  cases spec with
  | nullish head rest hn =>
    wp_apply wp_class_nullish c dst old tags v xs rest head dq hn hp hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hstep⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    isimp only [classResult, ClassReadOwned]
    iexists interface.nil, (1 : Nat)
    isimp only [ClassValueOwned]
    iframe Hdst Hc
    ipureintro
    exact ⟨trivial, by simp; omega, Hstep⟩
  | @tagFailure wire reason hn hu failure =>
    wp_apply wp_class_tag_rejected c dst old tags v xs wire dq reason hn hu failure hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, %kinds, Hglobals, Herr⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    isimp only [classResult, ClassReadOwned]
    have hzero : v.pos' + W64 (0 : Nat) = v.pos' := by word
    have hdzero : v.depth' + W64 (0 : Nat) = v.depth' := by word
    isimp only [hzero, hdzero]
    iframe Hdst Hc
    isplitl [Hglobals Herr]
    · isimp only [ClassErrorOwned]
      iexists err, kinds
      iframe Hglobals Herr
      ipureintro; rfl
    ipureintro
    exact ⟨by simp, by simp, by simp, by omega, hp.2⟩
  | @unknown head payload rest tag enc major unknown =>
    wp_apply wp_class_unknown_tag c dst old tags v xs payload rest head tag dq enc major unknown hp hbytes
      $$ [Hdst Hc Hs] as %result ⟨Hdst, Hc, Hs, Herror, %Hstep⟩
    · iframe Hdst Hc Hs
    icases Herror with ⟨%kinds, Hglobals, %Hresult⟩
    iapply HΦ
    iframe Hs
    isimp only [classResult, ClassReadOwned]
    have hdzero : v.depth' + W64 (0 : Nat) = v.depth' := by word
    isimp only [hdzero, Int.natCast_add, Int.natCast_one]
    iframe Hdst Hc
    isplitl [Hglobals]
    · isimp only [ClassErrorOwned]
      iexists kinds
      iframe Hglobals
      ipureintro; exact Hresult
    ipureintro
    have hstep := Hstep.1
    simp only [Int.natCast_add, Int.natCast_one, Int.add_assoc] at hstep ⊢
    exact ⟨Hresult.2, hstep, hstep, by omega, Hstep.2.1⟩
  | @payload head payload body value enc major valid =>
    cases value with
    | deprecated value =>
      cases value with
      | success value rest =>
        obtain ⟨item, rfl, valid⟩ := valid
        wp_apply wp_class_deprecated_valid validator c dst old tags v xs payload item rest head value dq d
          distinct enc major valid hd hp (by simpa only [List.append_assoc] using hbytes)
          $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
        · iframe Hdst Hc Hs
        iapply HΦ
        iframe Hs
        isimp only [classResult, ClassReadOwned]
        iexists interface.mkOk (go.PointerType DeprecatedCairoClass.ty) #out, (1 + payload.length + item.length)
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned]
        · isimp only [ClassValueOwned]
          iexists out
          iframe Howned
          ipureintro; rfl
        ipureintro
        refine ⟨trivial, by simp; omega, ?_, Hstep.2⟩
        simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
      | failure retained consumed offset dep reason =>
        wp_apply wp_class_deprecated_rejected validator c dst old tags v xs payload body head retained dq
          d consumed offset dep reason distinct enc major valid hd hp hbytes
          $$ [Hdst Hc Hs] as %result ⟨Hdst, Howned, Hc, Hs, Herr, %Hstep⟩
        · iframe Hdst Hc Hs
        ihave %HnonNil := deprecatedClassFailureOwned_nonNil result _ reason $$ Herr
        iapply HΦ
        iframe Hs
        isimp only [classResult, ClassReadOwned]
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned Herr]
        · isimp only [ClassErrorOwned]
          iframe Howned
          iexact Herr
        ipureintro
        refine ⟨HnonNil, ?_, ?_, Hstep.2.2⟩
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.2.1
    | sierra value =>
      cases value with
      | success value rest =>
        obtain ⟨item, rfl, valid⟩ := valid
        wp_apply wp_class_sierra_valid validator c dst old tags v xs payload item rest head value dq d
          distinct enc major valid hd hp (by simpa only [List.append_assoc] using hbytes)
          $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
        · iframe Hdst Hc Hs
        iapply HΦ
        iframe Hs
        isimp only [classResult, ClassReadOwned]
        iexists interface.mkOk (go.PointerType SierraClass.ty) #out, (1 + payload.length + item.length)
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned]
        · isimp only [ClassValueOwned]
          iexists out
          iframe Howned
          ipureintro; rfl
        ipureintro
        refine ⟨trivial, by simp; omega, ?_, Hstep.2⟩
        simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
      | failure retained consumed offset dep reason =>
        wp_apply wp_class_sierra_rejected validator c dst old tags v xs payload body head retained dq
          d consumed offset dep reason distinct enc major valid hd hp hbytes
          $$ [Hdst Hc Hs] as %err ⟨Hdst, Howned, Hc, Hs, Herr, %Hstep⟩
        · iframe Hdst Hc Hs
        iapply HΦ
        iframe Hs
        isimp only [classResult, ClassReadOwned]
        isimp only [Int.natCast_add, Int.natCast_one]
        iframe Hdst Hc
        isplitl [Howned Herr]
        · isimp only [ClassErrorOwned]
          iframe Howned
          iexists err
          iframe Herr
          ipureintro; rfl
        ipureintro
        refine ⟨by simp, ?_, ?_, Hstep.2.2⟩
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.1
        · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hstep.2.1
/-- Complete all-input contract for the production class reader, with an
    independently unique value or failure and exact input/cursor ownership. -/
theorem wp_readClass_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoInterface) (tags : CBORClassTags) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (distinct : ClassTagsDistinct tags)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadClassDefinition)) (Val #c)) (Val #dst)) (Val #tags))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ outcome : ClassResult,
       ⌜ClassSpec d tags wire outcome ∧
         ∀ other, ClassSpec d tags wire other → other = outcome⌝ ∗
       ClassReadOwned c dst old v xs wire result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  obtain ⟨raw, spec⟩ := classDispatch_classify d tags wire hdepth
  wp_apply wp_readClass_spec validator c dst old tags v xs wire dq d raw distinct spec hd hp hbytes
    $$ [Hdst Hc Hs] as %result ⟨Hs, Howned⟩
  · iframe Hdst Hc Hs
  iapply HΦ
  iframe Hs
  iexists classResult raw
  iframe Howned
  ipureintro
  refine ⟨⟨raw, spec, rfl⟩, ?_⟩
  intro other otherSpec
  exact classSpec_unique d tags wire distinct other _ otherSpec ⟨raw, spec, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readClass_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readClass_complete
