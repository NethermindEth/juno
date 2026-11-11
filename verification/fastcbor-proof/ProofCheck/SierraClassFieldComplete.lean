import ProofCheck.SierraClassFieldRead

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

inductive SierraClassFieldOutcome where
  | success (value : SierraClassValue) (rest : List w8)
  | failure (retained : SierraClassValue) (consumed offset extraDepth : Nat) (reason : SierraClassFieldFailure)

def SierraClassFieldSpec (d : Nat) (field : SierraClassField) (old : SierraClassValue) (wire : List w8) : SierraClassFieldOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ SierraClassFieldValid d field old item rest value
  | .failure retained consumed offset extraDepth reason => SierraClassFieldRejected d field old wire retained consumed offset extraDepth reason

theorem sierraClassFieldSpec_unique (d : Nat) (field : SierraClassField) (old : SierraClassValue) (wire : List w8)
    (a b : SierraClassFieldOutcome) (first : SierraClassFieldSpec d field old wire a)
    (second : SierraClassFieldSpec d field old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := sierraClassField_prefix_unique d field old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset extraDepth reason =>
      subst wire
      exact False.elim (sierraClassField_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid second)
  | failure retained consumed offset extraDepth reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (sierraClassField_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid first)
    | failure retained2 consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := sierraClassField_rejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem sierraClassFieldSpec_total_unique (d : Nat) (field : SierraClassField) (old : SierraClassValue) (wire : List w8)
    (depth : d ≤ 32) : ∃ outcome, SierraClassFieldSpec d field old wire outcome ∧
      ∀ other, SierraClassFieldSpec d field old wire other → other = outcome := by
  have existsOutcome : ∃ outcome, SierraClassFieldSpec d field old wire outcome := by
    rcases sierraClassField_classify d field old wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure retained consumed offset extraDepth reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => sierraClassFieldSpec_unique d field old wire other outcome otherSpec spec⟩

def SierraClassFieldReadOwned (c dst : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : SierraClassFieldOutcome → IProp GF
  | .success value rest => iprop(∃ consumed : Nat,
      SierraClassOwned dst value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure retained consumed offset extraDepth reason => iprop(∃ err : Loc,
      SierraClassOwned dst retained ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      SierraClassFieldFailureOwned err (v.pos' + W64 offset) reason ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every generated field call realizes its independently unique outcome on all
    byte inputs, including nested partial failures and retained embedded fields. -/
theorem wp_sierraClass_field_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : SierraClassValue) (field : SierraClassField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SierraClassOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (sierraClassFieldCall (GF := GF) field c dst)
    {{ (result : GoInterface), RET #result; v.buf' ↦*{dq} xs ∗ ∃ outcome : SierraClassFieldOutcome,
       ⌜SierraClassFieldSpec d field old wire outcome ∧ ∀ other, SierraClassFieldSpec d field old wire other → other = outcome⌝ ∗
       SierraClassFieldReadOwned c dst v xs wire result outcome }} := by
  iintro %Φ ⟨#Hclasses, #Hschema, Howned, Hc, Hs⟩ HΦ
  obtain ⟨outcome, spec, unique⟩ := sierraClassFieldSpec_total_unique d field old wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_sierraClass_field_valid validator c dst old value field v xs item rest dq d valid hd depth hp (hbytes.trans split)
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists SierraClassFieldOutcome.success value rest
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [SierraClassFieldReadOwned]
    iexists item.length
    iframe Howned Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure retained consumed offset extraDepth reason =>
    wp_apply wp_sierraClass_field_rejected validator c dst old retained field v xs wire dq d consumed offset extraDepth reason
      spec hd depth hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists SierraClassFieldOutcome.failure retained consumed offset extraDepth reason
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [SierraClassFieldReadOwned]
    iexists err
    iframe Howned Hc Herr
    ipureintro; exact ⟨rfl, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassFieldSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassFieldSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_sierraClass_field_complete
