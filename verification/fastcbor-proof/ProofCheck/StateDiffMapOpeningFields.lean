import ProofCheck.StateDiffErrors
import ProofCheck.FeltMapOpeningOwned
import ProofCheck.StorageDiffsOpening
import ProofCheck.MigratedClassesOpening

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

/-- An opening rejection in StorageDiffs preserves the complete old record,
    including the selected map contents and all sibling allocations. -/
theorem wp_stateDiff_storage_open_rejected (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : StorageDiffsFailureReason)
    (failure : StorageDiffsSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #(structFieldRef StateDiff go!"StorageDiffs" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StateDiffFieldFailureOwned err v.pos' (.storage reason) ∗
       ⌜StateDiffFieldRejected d .storage wire 0 0 0 (.storage reason)⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_storage_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStorageDiffs_open_rejected c _ previous v xs wire dq d reason failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.storage [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  exact .storage failure

/-- An opening rejection in Nonces preserves the complete old record,
    including the selected map contents and all sibling allocations. -/
theorem wp_stateDiff_nonces_open_rejected (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltMapFailureReason)
    (failure : FeltMapSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"Nonces" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StateDiffFieldFailureOwned err v.pos' (.map reason) ∗
       ⌜StateDiffFieldRejected d .nonces wire 0 0 0 (.map reason)⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_nonces_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_open_rejected c _ previous v xs wire dq d reason failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.nonces [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  exact .nonces failure

/-- An opening rejection in DeployedContracts preserves the complete old record,
    including the selected map contents and all sibling allocations. -/
theorem wp_stateDiff_deployed_open_rejected (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltMapFailureReason)
    (failure : FeltMapSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeployedContracts" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StateDiffFieldFailureOwned err v.pos' (.map reason) ∗
       ⌜StateDiffFieldRejected d .deployed wire 0 0 0 (.map reason)⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_deployed_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_open_rejected c _ previous v xs wire dq d reason failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.deployed [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  exact .deployed failure

/-- An opening rejection in DeclaredV1Classes preserves the complete old record,
    including the selected map contents and all sibling allocations. -/
theorem wp_stateDiff_declaredV1_open_rejected (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltMapFailureReason)
    (failure : FeltMapSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeclaredV1Classes" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StateDiffFieldFailureOwned err v.pos' (.map reason) ∗
       ⌜StateDiffFieldRejected d .declaredV1 wire 0 0 0 (.map reason)⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_declaredV1_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_open_rejected c _ previous v xs wire dq d reason failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.declaredV1 [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  exact .declaredV1 failure

/-- An opening rejection in ReplacedClasses preserves the complete old record,
    including the selected map contents and all sibling allocations. -/
theorem wp_stateDiff_replaced_open_rejected (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltMapFailureReason)
    (failure : FeltMapSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"ReplacedClasses" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StateDiffFieldFailureOwned err v.pos' (.map reason) ∗
       ⌜StateDiffFieldRejected d .replaced wire 0 0 0 (.map reason)⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_replaced_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_open_rejected c _ previous v xs wire dq d reason failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.replaced [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  exact .replaced failure

/-- An opening rejection in MigratedClasses preserves the complete old record,
    including the selected map contents and all sibling allocations. -/
theorem wp_stateDiff_migrated_open_rejected (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : MigratedClassesFailureReason)
    (failure : MigratedClassesSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #(structFieldRef StateDiff go!"MigratedClasses" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StateDiffFieldFailureOwned err v.pos' (.migrated reason) ∗
       ⌜StateDiffFieldRejected d .migrated wire 0 0 0 (.migrated reason)⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_migrated_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readMigratedClasses_open_rejected c _ previous v xs wire dq d reason failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.migrated [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  exact .migrated failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_storage_open_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_nonces_open_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_deployed_open_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_declaredV1_open_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_replaced_open_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_migrated_open_rejected
