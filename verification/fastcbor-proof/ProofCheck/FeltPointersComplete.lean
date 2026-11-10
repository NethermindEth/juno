import ProofCheck.FeltPointersClassify

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

/-- A successful nullable felt-pointer slice owns its pointer table and shared
    backing values (or is nil), with exact prefix consumption. Failure preserves
    the old destination, keeps partial allocations unpublished, and retains the
    specified outer-array cursor state plus the owned diagnostic. Full input is framed separately. -/
def FeltPointersOutcome (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : Option (List (Option (GoArray w64 4)))) (p : GoSlice),
    dst ↦ p ∗ FeltPointersResult (GF := GF) p value ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ FeltPointersValid d item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (consumed offset : Nat) (entered : Bool) (reason : FeltFailureReason) (err : Loc), dst ↦ old ∗
    c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
    FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      FeltPointersFailure d wire consumed offset entered reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- The actual nullable felt-pointer slice reader has a specified owned value
    or failure on arbitrary input. Exhaustive classification and production
    success/failure proofs discharge every helper and loop obligation. -/
theorem wp_readFeltPointers_complete (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (kinds : FeltErrorKinds)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ FeltPointersOutcome c dst old v xs wire d kinds result }} := by
  wp_start_folded as ⟨Hglobals, Hdst, Hc, Hs⟩
  rcases feltPointers_classify d wire hdepth with ⟨item, rest, value, split, valid⟩ | ⟨consumed, offset, entered, reason, failure⟩
  · wp_apply wp_readFeltPointers_valid c dst old v xs item rest value dq d hd hdepth valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold FeltPointersOutcome
    ileft
    iexists item, rest, value, p
    iframe Hdst Hvalue Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_readFeltPointers_failure_bounded c dst old v xs wire dq d consumed offset entered reason (feltFailureKind reason kinds)
      failure hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hoffset⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold FeltPointersOutcome
    iright
    iexists consumed, offset, entered, reason, err
    iframe Hdst Hc Herr
    ipureintro
    exact ⟨rfl, failure, Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltPointers_complete
