import ProofCheck.ExecutionFieldRead

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

inductive ExecutionFieldOutcome where
  | success (value : ExecutionValue) (rest : List w8)
  | failure (retained : ExecutionValue) (consumed offset extraDepth : Nat) (reason : ExecutionFieldFailure)

def ExecutionFieldSpec (d : Nat) (field : ExecutionField) (old : ExecutionValue) (wire : List w8) : ExecutionFieldOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ ExecutionFieldValid d field old item rest value
  | .failure retained consumed offset extraDepth reason => ExecutionFieldRejected d field old wire retained consumed offset extraDepth reason

theorem executionFieldSpec_unique (d : Nat) (field : ExecutionField) (old : ExecutionValue) (wire : List w8)
    (a b : ExecutionFieldOutcome) (first : ExecutionFieldSpec d field old wire a)
    (second : ExecutionFieldSpec d field old wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := executionField_prefix_unique d field old value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure retained consumed offset extraDepth reason =>
      subst wire
      exact False.elim (executionField_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid second)
  | failure retained consumed offset extraDepth reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (executionField_not_rejected _ _ _ _ _ _ _ _ _ _ _ valid first)
    | failure retained2 consumed2 offset2 extraDepth2 reason2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := executionField_rejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ _ first second
      rfl

theorem executionFieldSpec_total_unique (d : Nat) (field : ExecutionField) (old : ExecutionValue) (wire : List w8)
    (depth : d ≤ 32) : ∃ outcome, ExecutionFieldSpec d field old wire outcome ∧
      ∀ other, ExecutionFieldSpec d field old wire other → other = outcome := by
  have existsOutcome : ∃ outcome, ExecutionFieldSpec d field old wire outcome := by
    rcases executionField_classify d field old wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure retained consumed offset extraDepth reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => executionFieldSpec_unique d field old wire other outcome otherSpec spec⟩

def ExecutionFieldReadOwned (c dst availability gas : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : ExecutionFieldOutcome → IProp GF
  | .success value rest => iprop(∃ consumed : Nat,
      ExecutionOwned dst availability gas value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure retained consumed offset extraDepth reason => iprop(∃ err : Loc,
      ExecutionOwned dst availability gas retained ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      ExecutionFieldFailureOwned err (v.pos' + W64 offset) reason ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every generated field call realizes its independently unique outcome on all
    byte inputs, including nested partial failures and retained shared storage. -/
theorem wp_execution_field_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst availability gas : Loc) (old : ExecutionValue) (field : ExecutionField)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ExecutionOwned dst availability gas old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (executionFieldCall (GF := GF) field c dst availability gas)
    {{ (result : GoInterface), RET #result; v.buf' ↦*{dq} xs ∗ ∃ outcome : ExecutionFieldOutcome,
       ⌜ExecutionFieldSpec d field old wire outcome ∧ ∀ other, ExecutionFieldSpec d field old wire other → other = outcome⌝ ∗
       ExecutionFieldReadOwned c dst availability gas v xs wire result outcome }} := by
  iintro %Φ ⟨#Hreceipts, #Hschema, Howned, Hc, Hs⟩ HΦ
  obtain ⟨outcome, spec, unique⟩ := executionFieldSpec_total_unique d field old wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_execution_field_valid validator c dst availability gas old value field v xs item rest dq d valid hd hp (hbytes.trans split)
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists ExecutionFieldOutcome.success value rest
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [ExecutionFieldReadOwned]
    iexists item.length
    iframe Howned Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure retained consumed offset extraDepth reason =>
    wp_apply wp_execution_field_rejected validator c dst availability gas old retained field v xs wire dq d consumed offset extraDepth reason
      spec hd hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists ExecutionFieldOutcome.failure retained consumed offset extraDepth reason
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [ExecutionFieldReadOwned]
    iexists err
    iframe Howned Hc Herr
    ipureintro; exact ⟨rfl, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.executionFieldSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.executionFieldSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_execution_field_complete
