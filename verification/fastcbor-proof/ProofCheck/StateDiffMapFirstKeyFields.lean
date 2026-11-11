import ProofCheck.StateDiffErrors
import ProofCheck.StateMapFirstKeyFailureSpec
import ProofCheck.FeltMapFirstKeyFailure
import ProofCheck.StorageDiffsFirstKeyFailure
import ProofCheck.MigratedClassesFirstKeyFailure

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

/-- The first key failure in StorageDiffs preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_storage_first_key_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload wire : List w8) (head : w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ wire.length)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #(structFieldRef StateDiff go!"StorageDiffs" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) (.storage (.element (.key reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .storage ((head :: payload) ++ wire) (1 + payload.length) (1 + payload.length + offset) 1
           (.storage (.element (.key reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_storage_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStorageDiffs_first_key_failure c _ previous count v xs payload wire head dq d offset reason enc major positive countBound fit failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.storage [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  have spec := storageDiffs_first_key_failure_spec d count offset head payload wire reason
    enc major positive countBound fit depth failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .storage spec⟩

/-- The first key failure in Nonces preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_nonces_first_key_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload wire : List w8) (head : w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ wire.length)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"Nonces" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) (.map (.element (.key reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .nonces ((head :: payload) ++ wire) (1 + payload.length) (1 + payload.length + offset) 1
           (.map (.element (.key reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_nonces_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_first_key_failure c _ previous count v xs payload wire head dq d offset reason enc major positive countBound fit failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.nonces [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  have spec := feltMap_first_key_failure_spec d count offset head payload wire reason
    enc major positive countBound fit depth failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .nonces spec⟩

/-- The first key failure in DeployedContracts preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_deployed_first_key_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload wire : List w8) (head : w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ wire.length)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeployedContracts" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) (.map (.element (.key reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .deployed ((head :: payload) ++ wire) (1 + payload.length) (1 + payload.length + offset) 1
           (.map (.element (.key reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_deployed_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_first_key_failure c _ previous count v xs payload wire head dq d offset reason enc major positive countBound fit failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.deployed [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  have spec := feltMap_first_key_failure_spec d count offset head payload wire reason
    enc major positive countBound fit depth failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .deployed spec⟩

/-- The first key failure in DeclaredV1Classes preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_declaredV1_first_key_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload wire : List w8) (head : w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ wire.length)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeclaredV1Classes" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) (.map (.element (.key reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .declaredV1 ((head :: payload) ++ wire) (1 + payload.length) (1 + payload.length + offset) 1
           (.map (.element (.key reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_declaredV1_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_first_key_failure c _ previous count v xs payload wire head dq d offset reason enc major positive countBound fit failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.declaredV1 [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  have spec := feltMap_first_key_failure_spec d count offset head payload wire reason
    enc major positive countBound fit depth failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .declaredV1 spec⟩

/-- The first key failure in ReplacedClasses preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_replaced_first_key_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload wire : List w8) (head : w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ wire.length)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"ReplacedClasses" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) (.map (.element (.key reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .replaced ((head :: payload) ++ wire) (1 + payload.length) (1 + payload.length + offset) 1
           (.map (.element (.key reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_replaced_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_first_key_failure c _ previous count v xs payload wire head dq d offset reason enc major positive countBound fit failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.replaced [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  have spec := feltMap_first_key_failure_spec d count offset head payload wire reason
    enc major positive countBound fit depth failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .replaced spec⟩

/-- The first key failure in MigratedClasses preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_migrated_first_key_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload wire : List w8) (head : w8) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ wire.length)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #(structFieldRef StateDiff go!"MigratedClasses" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) (.migrated (.element (.key reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .migrated ((head :: payload) ++ wire) (1 + payload.length) (1 + payload.length + offset) 1
           (.migrated (.element (.key reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_migrated_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readMigratedClasses_first_key_failure c _ previous count v xs payload wire head dq d offset reason enc major positive countBound fit failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.migrated [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold StateDiffFieldFailureOwned
    iexact Herr
  ipureintro
  have spec := migratedClasses_first_key_failure_spec d count offset head payload wire reason
    enc major positive countBound fit depth failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .migrated spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_storage_first_key_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_nonces_first_key_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_deployed_first_key_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_declaredV1_first_key_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_replaced_first_key_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_migrated_first_key_failure
