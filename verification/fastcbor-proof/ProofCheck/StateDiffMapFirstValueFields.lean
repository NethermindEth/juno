import ProofCheck.StateDiffErrors
import ProofCheck.FeltMapFirstFailure
import ProofCheck.MigratedClassesFirstValueFailure
import ProofCheck.MigratedClassesFirstValueFailureSpec

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

/-- The first value failure in Nonces preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_nonces_first_value_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ (keyItem ++ wire).length) (countBound : count < 2^63)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"Nonces" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) (.map (.element (.value reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .nonces ((head :: payload) ++ keyItem ++ wire) (1 + payload.length + keyItem.length) (1 + payload.length + keyItem.length + offset) 1
           (.map (.element (.value reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_nonces_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_first_value_failure c _ previous count v xs payload keyItem wire head key dq d offset reason enc major positive fit countBound valid failure hd depth hp hbytes
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
  have spec := feltMap_first_value_failure_spec d count offset head payload keyItem wire key reason
    enc major positive countBound fit depth valid failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .nonces spec⟩

/-- The first value failure in DeployedContracts preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_deployed_first_value_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ (keyItem ++ wire).length) (countBound : count < 2^63)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeployedContracts" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) (.map (.element (.value reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .deployed ((head :: payload) ++ keyItem ++ wire) (1 + payload.length + keyItem.length) (1 + payload.length + keyItem.length + offset) 1
           (.map (.element (.value reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_deployed_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_first_value_failure c _ previous count v xs payload keyItem wire head key dq d offset reason enc major positive fit countBound valid failure hd depth hp hbytes
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
  have spec := feltMap_first_value_failure_spec d count offset head payload keyItem wire key reason
    enc major positive countBound fit depth valid failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .deployed spec⟩

/-- The first value failure in DeclaredV1Classes preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_declaredV1_first_value_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ (keyItem ++ wire).length) (countBound : count < 2^63)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeclaredV1Classes" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) (.map (.element (.value reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .declaredV1 ((head :: payload) ++ keyItem ++ wire) (1 + payload.length + keyItem.length) (1 + payload.length + keyItem.length + offset) 1
           (.map (.element (.value reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_declaredV1_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_first_value_failure c _ previous count v xs payload keyItem wire head key dq d offset reason enc major positive fit countBound valid failure hd depth hp hbytes
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
  have spec := feltMap_first_value_failure_spec d count offset head payload keyItem wire key reason
    enc major positive countBound fit depth valid failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .declaredV1 spec⟩

/-- The first value failure in ReplacedClasses preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_replaced_first_value_failure (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ (keyItem ++ wire).length) (countBound : count < 2^63)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"ReplacedClasses" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) (.map (.element (.value reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .replaced ((head :: payload) ++ keyItem ++ wire) (1 + payload.length + keyItem.length) (1 + payload.length + keyItem.length + offset) 1
           (.map (.element (.value reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_replaced_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_first_value_failure c _ previous count v xs payload keyItem wire head key dq d offset reason enc major positive fit countBound valid failure hd depth hp hbytes
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
  have spec := feltMap_first_value_failure_spec d count offset head payload keyItem wire key reason
    enc major positive countBound fit depth valid failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .replaced spec⟩

/-- The first value failure in MigratedClasses preserves the entire old StateDiff,
    retaining exact consumed bytes, diagnostic offset and map depth. -/
theorem wp_stateDiff_migrated_first_value_failure (c dst : Loc) (old : StateDiffValue) (count : Nat) (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ (keyItem ++ wire).length)
    (countBound : count < 2^63)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : LimbsFailure (d + 1) wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #(structFieldRef StateDiff go!"MigratedClasses" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) (.migrated (.element (.value reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .migrated ((head :: payload) ++ keyItem ++ wire) (1 + payload.length + keyItem.length) (1 + payload.length + keyItem.length + offset) 1
           (.migrated (.element (.value reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_migrated_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readMigratedClasses_first_value_failure c _ previous count v xs payload keyItem wire head key dq d offset reason enc major positive fit countBound valid failure hd depth hp hbytes
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
  have spec := migratedClasses_first_value_failure_spec d count offset head payload keyItem wire key reason
    enc major positive countBound fit depth valid failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .migrated spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_nonces_first_value_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_deployed_first_value_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_declaredV1_first_value_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_replaced_first_value_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_migrated_first_value_failure
