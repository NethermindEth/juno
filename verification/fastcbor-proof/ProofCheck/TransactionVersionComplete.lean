import ProofCheck.TransactionVersionRead
import ProofCheck.FeltFailureUnique

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

/-- The version decoder shares the independent limb encoding contract; it
    does not validate a named Starknet transaction version. -/
inductive TransactionVersionResult where
  | success (value : Option (GoArray w64 4)) (rest : List w8)
  | failure (offset : Nat) (reason : FeltFailureReason)

def TransactionVersionSpec (d : Nat) (wire : List w8) : TransactionVersionResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ FeltValid d item value
  | .failure offset reason => FeltFailure d wire offset reason

theorem transactionVersionSpec_unique (d : Nat) (wire : List w8) (a b : TransactionVersionResult)
    (first : TransactionVersionSpec d wire a) (second : TransactionVersionSpec d wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := feltValid_prefix_unique d value value2 item item2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure offset reason =>
      subst wire
      exact False.elim (feltValid_not_failure _ _ _ _ _ _ valid second)
  | failure offset reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (feltValid_not_failure _ _ _ _ _ _ valid first)
    | failure offset2 reason2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ first second
      rfl

theorem transactionVersionSpec_total_unique (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, TransactionVersionSpec d wire outcome ∧
      ∀ other, TransactionVersionSpec d wire other → other = outcome := by
  have existsOutcome : ∃ outcome, TransactionVersionSpec d wire outcome := by
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure offset reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => transactionVersionSpec_unique d wire other outcome otherSpec spec⟩

def TransactionVersionReadOwned (c dst old : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : TransactionVersionResult → IProp GF
  | .success value rest => iprop(∃ (out : Loc) (consumed : Nat),
      dst ↦ out ∗ FeltResult out value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure offset reason => iprop(dst ↦ old ∗ c ↦ v ∗ ∃ err : Loc,
      FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every byte input and old pointer has an independently unique outcome.
    Initialized errors and actual allocation/cast/limb parsing are composed,
    with no UTF-8 or other assumed helper-correctness premise. -/
theorem wp_readTransactionVersion_complete (c dst old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadTransactionVersion)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (kinds : FeltErrorKinds) (outcome : TransactionVersionResult),
       FeltErrorGlobals kinds ∗ ⌜TransactionVersionSpec d wire outcome ∧
         ∀ other, TransactionVersionSpec d wire other → other = outcome⌝ ∗
       TransactionVersionReadOwned c dst old v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, Hglobals⟩
  obtain ⟨outcome, spec, unique⟩ := transactionVersionSpec_total_unique d wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_readTransactionVersion_valid c dst old v xs item rest value dq d hd depth valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, TransactionVersionResult.success value rest
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [TransactionVersionReadOwned]
    iexists out, item.length
    iframe Hdst Hvalue Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure offset reason =>
    ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_readTransactionVersion_rejected c dst old v xs wire dq d offset reason (feltFailureKind reason kinds)
      spec hd depth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, TransactionVersionResult.failure offset reason
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [TransactionVersionReadOwned]
    iframe Hdst Hc
    iexists err
    iframe Herr
    ipureintro; exact ⟨rfl, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionVersionSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionVersionSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readTransactionVersion_complete
