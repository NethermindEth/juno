import ProofCheck.DeployAccountFieldRejected

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

inductive DeployAccountFieldOutcome where
  | success (value : DeployAccountValue) (rest : List w8)
  | failure (consumed offset extraDepth : Nat) (reason : DeployAccountFieldFailure)

def DeployAccountFieldSpec (d : Nat) (field : DeployAccountField) (old : DeployAccountValue) (wire : List w8) : DeployAccountFieldOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ DeployAccountFieldValid d field old item rest value
  | .failure consumed offset extraDepth reason => DeployAccountFieldRejected d field wire consumed offset extraDepth reason

theorem deployAccountFieldSpec_unique (d : Nat) (field : DeployAccountField) (old : DeployAccountValue) (wire : List w8)
    (a b : DeployAccountFieldOutcome) (first : DeployAccountFieldSpec d field old wire a)
    (second : DeployAccountFieldSpec d field old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := deployAccountField_prefix_unique d field old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure consumed offset extraDepth reason =>
      subst wire
      exact False.elim (deployAccountField_not_rejected _ _ _ _ _ _ _ _ _ _ valid second)
  | failure consumed offset extraDepth reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (deployAccountField_not_rejected _ _ _ _ _ _ _ _ _ _ valid first)
    | failure consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := deployAccountField_rejected_unique _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem deployAccountFieldSpec_total_unique (d : Nat) (field : DeployAccountField) (old : DeployAccountValue) (wire : List w8)
    (depth : d ≤ 32) : ∃ outcome, DeployAccountFieldSpec d field old wire outcome ∧
      ∀ other, DeployAccountFieldSpec d field old wire other → other = outcome := by
  have existsOutcome : ∃ outcome, DeployAccountFieldSpec d field old wire outcome := by
    rcases deployAccountField_classify d field old wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure consumed offset extraDepth reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => deployAccountFieldSpec_unique d field old wire other outcome otherSpec spec⟩

def DeployAccountFieldReadOwned (c dst : Loc) (old : DeployAccountValue) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : DeployAccountFieldOutcome → IProp GF
  | .success value rest => iprop(∃ consumed : Nat,
      DeployAccountOwned dst value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure consumed offset extraDepth reason => iprop(
      DeployAccountOwned dst old ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      DeployAccountFieldError result (v.pos' + W64 offset) reason ∗
      ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every generated field call realizes its independently unique outcome on all
    byte inputs, including nested partial failures, unchanged deployAccount fields and bare DA-mode errors. -/
theorem wp_deployAccount_field_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : DeployAccountValue) (field : DeployAccountField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeployAccountOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (deployAccountFieldReader field))) (Val #c)) (Val #(deployAccountFieldRef (GF := GF) field dst)))
    {{ (result : GoInterface), RET #result; v.buf' ↦*{dq} xs ∗ ∃ outcome : DeployAccountFieldOutcome,
       ⌜DeployAccountFieldSpec d field old wire outcome ∧ ∀ other, DeployAccountFieldSpec d field old wire other → other = outcome⌝ ∗
       DeployAccountFieldReadOwned c dst old v xs wire result outcome }} := by
  iintro %Φ ⟨#HdeployAccounts, #Hschema, Howned, Hc, Hs⟩ HΦ
  obtain ⟨outcome, spec, unique⟩ := deployAccountFieldSpec_total_unique d field old wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_deployAccount_field_valid validator c dst old value field v xs item rest dq d valid hd depth hp (hbytes.trans split)
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists DeployAccountFieldOutcome.success value rest
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [DeployAccountFieldReadOwned]
    iexists item.length
    iframe Howned Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure consumed offset extraDepth reason =>
    wp_apply wp_deployAccount_field_rejected validator c dst old field v xs wire dq d consumed offset extraDepth reason
      spec hd depth hp hbytes $$ [Howned Hc Hs] as %result ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists DeployAccountFieldOutcome.failure consumed offset extraDepth reason
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [DeployAccountFieldReadOwned]
    iframe Howned Hc Herr
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountFieldSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountFieldSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deployAccount_field_complete
