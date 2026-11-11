import ProofCheck.DeclareFieldRejected

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

inductive DeclareFieldOutcome where
  | success (value : DeclareValue) (rest : List w8)
  | failure (consumed offset extraDepth : Nat) (reason : DeclareFieldFailure)

def DeclareFieldSpec (d : Nat) (field : DeclareField) (old : DeclareValue) (wire : List w8) : DeclareFieldOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ DeclareFieldValid d field old item rest value
  | .failure consumed offset extraDepth reason => DeclareFieldRejected d field wire consumed offset extraDepth reason

theorem declareFieldSpec_unique (d : Nat) (field : DeclareField) (old : DeclareValue) (wire : List w8)
    (a b : DeclareFieldOutcome) (first : DeclareFieldSpec d field old wire a)
    (second : DeclareFieldSpec d field old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := declareField_prefix_unique d field old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure consumed offset extraDepth reason =>
      subst wire
      exact False.elim (declareField_not_rejected _ _ _ _ _ _ _ _ _ _ valid second)
  | failure consumed offset extraDepth reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (declareField_not_rejected _ _ _ _ _ _ _ _ _ _ valid first)
    | failure consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := declareField_rejected_unique _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem declareFieldSpec_total_unique (d : Nat) (field : DeclareField) (old : DeclareValue) (wire : List w8)
    (depth : d ≤ 32) : ∃ outcome, DeclareFieldSpec d field old wire outcome ∧
      ∀ other, DeclareFieldSpec d field old wire other → other = outcome := by
  have existsOutcome : ∃ outcome, DeclareFieldSpec d field old wire outcome := by
    rcases declareField_classify d field old wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure consumed offset extraDepth reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => declareFieldSpec_unique d field old wire other outcome otherSpec spec⟩

def DeclareFieldReadOwned (c dst : Loc) (old : DeclareValue) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : DeclareFieldOutcome → IProp GF
  | .success value rest => iprop(∃ consumed : Nat,
      DeclareOwned dst value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure consumed offset extraDepth reason => iprop(
      DeclareOwned dst old ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      DeclareFieldError result (v.pos' + W64 offset) reason ∗
      ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every generated field call realizes its independently unique outcome on all
    byte inputs, including nested partial failures, unchanged declare fields and bare DA-mode errors. -/
theorem wp_declare_field_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : DeclareValue) (field : DeclareField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeclareOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (declareFieldReader field))) (Val #c)) (Val #(structFieldRef DeclareTransaction (declareFieldName field) dst)))
    {{ (result : GoInterface), RET #result; v.buf' ↦*{dq} xs ∗ ∃ outcome : DeclareFieldOutcome,
       ⌜DeclareFieldSpec d field old wire outcome ∧ ∀ other, DeclareFieldSpec d field old wire other → other = outcome⌝ ∗
       DeclareFieldReadOwned c dst old v xs wire result outcome }} := by
  iintro %Φ ⟨#Hdeclares, #Hschema, Howned, Hc, Hs⟩ HΦ
  obtain ⟨outcome, spec, unique⟩ := declareFieldSpec_total_unique d field old wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_declare_field_valid validator c dst old value field v xs item rest dq d valid hd depth hp (hbytes.trans split)
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists DeclareFieldOutcome.success value rest
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [DeclareFieldReadOwned]
    iexists item.length
    iframe Howned Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure consumed offset extraDepth reason =>
    wp_apply wp_declare_field_rejected validator c dst old field v xs wire dq d consumed offset extraDepth reason
      spec hd depth hp hbytes $$ [Howned Hc Hs] as %result ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists DeclareFieldOutcome.failure consumed offset extraDepth reason
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [DeclareFieldReadOwned]
    iframe Howned Hc Herr
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declareFieldSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.declareFieldSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_declare_field_complete
