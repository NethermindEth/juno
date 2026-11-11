import ProofCheck.StateDiffErrors

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Successful DeclaredV0Classes decoding publishes exactly the child value and
    preserves all sibling fields and their allocations. -/
theorem wp_stateDiff_declaredV0_valid (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Option (List (Option (GoArray w64 4)))) (dq : DFrac) (d : Nat)
    (valid : FeltPointersValid d item value) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeclaredV0Classes" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with declaredV0 := value} ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_declaredV0_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltPointers_valid c _ previous v xs item rest value dq d hd depth valid hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %value [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro; exact Hnext

/-- Every DeclaredV0Classes failure preserves the entire previous record and
    returns an initialized owned diagnostic at the exact input offset. -/
theorem wp_stateDiff_declaredV0_rejected (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (entered : Bool) (reason : FeltFailureReason)
    (failure : FeltPointersFailure d wire consumed offset entered reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeclaredV0Classes" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 (if entered then 1 else 0)} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 offset) (.felts reason) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  icases stateDiff_declaredV0_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
  wp_apply wp_readFeltPointers_failure_bounded c _ previous v xs wire dq d consumed offset entered reason (feltFailureKind reason kinds)
    failure hd depth hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hkind Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.declaredV0 [Hfield Hprevious]
  · iframe Hfield Hprevious
  have hdepth : (if entered then v.depth' + W64 1 else v.depth') = v.depth' + W64 (if entered then 1 else 0) := by
    cases entered <;> simp <;> word
  isimp only [hdepth] at Hc
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexists kinds
    iframe Hglobals Herr
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_declaredV0_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_declaredV0_rejected
