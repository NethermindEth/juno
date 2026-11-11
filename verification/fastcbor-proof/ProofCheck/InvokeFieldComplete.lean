import ProofCheck.InvokeFieldRejected

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

inductive InvokeFieldOutcome where
  | success (value : InvokeValue) (rest : List w8)
  | failure (consumed offset extraDepth : Nat) (reason : InvokeFieldFailure)

def InvokeFieldSpec (d : Nat) (field : InvokeField) (old : InvokeValue) (wire : List w8) : InvokeFieldOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ InvokeFieldValid d field old item rest value
  | .failure consumed offset extraDepth reason => InvokeFieldRejected d field wire consumed offset extraDepth reason

theorem invokeFieldSpec_unique (d : Nat) (field : InvokeField) (old : InvokeValue) (wire : List w8)
    (a b : InvokeFieldOutcome) (first : InvokeFieldSpec d field old wire a)
    (second : InvokeFieldSpec d field old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := invokeField_prefix_unique d field old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure consumed offset extraDepth reason =>
      subst wire
      exact False.elim (invokeField_not_rejected _ _ _ _ _ _ _ _ _ _ valid second)
  | failure consumed offset extraDepth reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (invokeField_not_rejected _ _ _ _ _ _ _ _ _ _ valid first)
    | failure consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := invokeField_rejected_unique _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem invokeFieldSpec_total_unique (d : Nat) (field : InvokeField) (old : InvokeValue) (wire : List w8)
    (depth : d ≤ 32) : ∃ outcome, InvokeFieldSpec d field old wire outcome ∧
      ∀ other, InvokeFieldSpec d field old wire other → other = outcome := by
  have existsOutcome : ∃ outcome, InvokeFieldSpec d field old wire outcome := by
    rcases invokeField_classify d field old wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure consumed offset extraDepth reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => invokeFieldSpec_unique d field old wire other outcome otherSpec spec⟩

def InvokeFieldReadOwned (c dst : Loc) (old : InvokeValue) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : InvokeFieldOutcome → IProp GF
  | .success value rest => iprop(∃ consumed : Nat,
      InvokeOwned dst value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure consumed offset extraDepth reason => iprop(
      InvokeOwned dst old ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      InvokeFieldError result (v.pos' + W64 offset) reason ∗
      ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every generated field call realizes its independently unique outcome on all
    byte inputs, including nested partial failures, unchanged invoke fields and bare DA-mode errors. -/
theorem wp_invoke_field_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : InvokeValue) (field : InvokeField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       InvokeOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (invokeFieldReader field))) (Val #c)) (Val #(structFieldRef InvokeTransaction (invokeFieldName field) dst)))
    {{ (result : GoInterface), RET #result; v.buf' ↦*{dq} xs ∗ ∃ outcome : InvokeFieldOutcome,
       ⌜InvokeFieldSpec d field old wire outcome ∧ ∀ other, InvokeFieldSpec d field old wire other → other = outcome⌝ ∗
       InvokeFieldReadOwned c dst old v xs wire result outcome }} := by
  iintro %Φ ⟨#Hinvokes, #Hschema, Howned, Hc, Hs⟩ HΦ
  obtain ⟨outcome, spec, unique⟩ := invokeFieldSpec_total_unique d field old wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_invoke_field_valid validator c dst old value field v xs item rest dq d valid hd depth hp (hbytes.trans split)
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists InvokeFieldOutcome.success value rest
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [InvokeFieldReadOwned]
    iexists item.length
    iframe Howned Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure consumed offset extraDepth reason =>
    wp_apply wp_invoke_field_rejected validator c dst old field v xs wire dq d consumed offset extraDepth reason
      spec hd depth hp hbytes $$ [Howned Hc Hs] as %result ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists InvokeFieldOutcome.failure consumed offset extraDepth reason
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [InvokeFieldReadOwned]
    iframe Howned Hc Herr
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeFieldSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeFieldSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_invoke_field_complete
