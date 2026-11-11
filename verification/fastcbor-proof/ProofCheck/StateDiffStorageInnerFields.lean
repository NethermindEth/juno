import ProofCheck.StateDiffErrors
import ProofCheck.StorageDiffsInnerOpening
import ProofCheck.StorageDiffsInnerOpeningSpec
import ProofCheck.StorageDiffsInnerKeyFailure
import ProofCheck.StorageDiffsInnerValueFailure
import ProofCheck.StorageDiffsInnerFailureSpec

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

/-- A first-inner open failure in StorageDiffs preserves the complete
    old record and all children, with the independent nested field diagnostic. -/
theorem wp_stateDiff_storage_first_inner_open_rejected (c dst : Loc) (old : StateDiffValue) (count : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire : List w8)
    (head : w8) (key : FeltMapKey) (dq : DFrac) (d : Nat) (reason : FeltMapFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ (keyItem ++ wire).length)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (failure : FeltMapSpec (d + 1) wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #(structFieldRef StateDiff go!"StorageDiffs" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length)) (.storage (.element (.value reason))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + keyItem.length))) (sint.nat v.buf'.len) xs = wire ∧
         StateDiffFieldRejected d .storage ((head :: payload) ++ keyItem ++ wire)
           (1 + payload.length + keyItem.length) (1 + payload.length + keyItem.length) 1 (.storage (.element (.value reason)))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateDiff_storage_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStorageDiffs_first_inner_open_rejected c _ previous count v xs payload keyItem wire head key dq d reason enc major positive countBound fit valid failure hd depth hp hbytes
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
  have spec := storageDiffs_first_inner_open_rejected_spec d count head payload keyItem wire key reason
    enc major positive countBound fit depth valid failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .storage spec⟩

/-- A first-inner key failure in StorageDiffs preserves the complete
    old record and all children, with the independent nested field diagnostic. -/
theorem wp_stateDiff_storage_first_inner_key_failure (c dst : Loc) (old : StateDiffValue) (count innerCount : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire innerPayload tail : List w8)
    (head innerHead : w8) (key : FeltMapKey) (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ (keyItem ++ wire).length)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (innerEnc : HeadArgument innerHead innerPayload (W64 innerCount))
    (innerMajor : innerHead &&& W8 224 = W8 160) (innerPositive : 0 < innerCount)
    (innerFit : 2 * innerCount ≤ tail.length)
    (split : wire = (innerHead :: innerPayload) ++ tail)
    (failure : LimbsFailure (d + 1 + 1) tail offset reason)
    (hd : sint.Z v.depth' = d) (depth : d + 1 < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #(structFieldRef StateDiff go!"StorageDiffs" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length), depth' := v.depth' + W64 2} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset)) (.storage (.element (.value (.element (.key reason))))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + 1 + innerPayload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + 1 + innerPayload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .storage ((head :: payload) ++ keyItem ++ wire)
           (1 + payload.length + keyItem.length + 1 + innerPayload.length) (1 + payload.length + keyItem.length + 1 + innerPayload.length + offset) 2 (.storage (.element (.value (.element (.key reason)))))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have innerBound : innerCount < 2^63 := by
    have machine : sint.Z v.buf'.len < 2^63 := by word
    have available := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at available
    have splitLength := congrArg List.length split
    simp only [List.length_append, List.length_cons] at splitLength innerFit
    simp only [sint.nat, sint.Z] at Hlen hp available machine
    omega
  icases stateDiff_storage_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStorageDiffs_first_inner_key_failure c _ previous count innerCount v xs payload keyItem wire innerPayload tail head innerHead key dq d offset reason enc major positive countBound fit valid innerEnc innerMajor innerPositive innerFit split failure hd depth hp hbytes
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
  have spec := storageDiffs_first_inner_key_failure_spec d count innerCount offset head innerHead payload keyItem wire innerPayload tail key reason
    enc major positive countBound fit depth valid innerEnc innerMajor innerPositive innerBound innerFit split failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .storage spec⟩

/-- A first-inner value failure in StorageDiffs preserves the complete
    old record and all children, with the independent nested field diagnostic. -/
theorem wp_stateDiff_storage_first_inner_value_failure (c dst : Loc) (old : StateDiffValue) (count innerCount : Nat)
    (v : cursor.Cursor) (xs payload keyItem wire innerPayload tail : List w8)
    (head innerHead : w8) (key : FeltMapKey) (innerKeyItem : List w8) (innerKey : FeltMapKey)
    (dq : DFrac) (d offset : Nat) (reason : FeltFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (countBound : count < 2^63) (fit : 2 * count ≤ (keyItem ++ wire).length)
    (valid : FeltMapKeyValid (d + 1) keyItem key)
    (innerEnc : HeadArgument innerHead innerPayload (W64 innerCount))
    (innerMajor : innerHead &&& W8 224 = W8 160) (innerPositive : 0 < innerCount)
    (innerFit : 2 * innerCount ≤ (innerKeyItem ++ tail).length)
    (split : wire = (innerHead :: innerPayload) ++ innerKeyItem ++ tail)
    (innerValid : FeltMapKeyValid (d + 1 + 1) innerKeyItem innerKey)
    (failure : FeltFailure (d + 1 + 1) tail offset reason)
    (hd : sint.Z v.depth' = d) (depth : d + 1 < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ keyItem ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #(structFieldRef StateDiff go!"StorageDiffs" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateDiffOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length), depth' := v.depth' + W64 2} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFieldFailureOwned err (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length + offset)) (.storage (.element (.value (.element (.value reason))))) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length + offset)) = sint.Z v.pos' + 1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length + offset)) ≤ sint.Z v.buf'.len ∧
         StateDiffFieldRejected d .storage ((head :: payload) ++ keyItem ++ wire)
           (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length) (1 + payload.length + keyItem.length + 1 + innerPayload.length + innerKeyItem.length + offset) 2 (.storage (.element (.value (.element (.value reason)))))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have innerBound : innerCount < 2^63 := by
    have machine : sint.Z v.buf'.len < 2^63 := by word
    have available := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at available
    have splitLength := congrArg List.length split
    simp only [List.length_append, List.length_cons] at splitLength innerFit
    simp only [sint.nat, sint.Z] at Hlen hp available machine
    omega
  icases stateDiff_storage_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStorageDiffs_first_inner_value_failure c _ previous count innerCount v xs payload keyItem wire innerPayload tail head innerHead key innerKeyItem innerKey dq d offset reason enc major positive countBound fit valid innerEnc innerMajor innerPositive innerFit split innerValid failure hd depth hp hbytes
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
  have spec := storageDiffs_first_inner_value_failure_spec d count innerCount offset head innerHead payload keyItem wire innerPayload innerKeyItem tail key innerKey reason
    enc major positive countBound fit depth valid innerEnc innerMajor innerPositive innerBound innerFit split innerValid failure
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .storage spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_storage_first_inner_open_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_storage_first_inner_key_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_storage_first_inner_value_failure
