import ProofCheck.StateDiffErrors
import ProofCheck.FeltMapEmpty
import ProofCheck.StorageDiffsEmpty
import ProofCheck.MigratedClassesEmpty
import ProofCheck.StateMapEmptySpec

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

/-- Empty StorageDiffs is allocated, distinct from nil; only that field changes.
    All map-head widths and suffixes are covered with restored depth. -/
theorem wp_stateDiff_storage_empty (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #(structFieldRef StateDiff go!"StorageDiffs" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with storage := some ∅} ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .storage old (head :: payload) rest {old with storage := some ∅}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  icases stateDiff_storage_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStorageDiffs_empty c _ previous v xs payload rest head dq d enc major hd depth hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hprogress⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %(some ∅) [Hfield Hnew]
  · iframe Hfield
    unfold StorageDiffsResult
    iexact Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  have bound : sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by
    simpa only [List.length_cons, Nat.add_comm payload.length 1, Int.natCast_add, Int.natCast_one] using Hnext.2.1
  exact ⟨Hprogress.1, bound, Hprogress.2, .storage (storageDiffs_empty_spec d head payload rest enc major depth)⟩

/-- Empty Nonces is allocated, distinct from nil; only that field changes.
    All map-head widths and suffixes are covered with restored depth. -/
theorem wp_stateDiff_nonces_empty (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"Nonces" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with nonces := some ∅} ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .nonces old (head :: payload) rest {old with nonces := some ∅}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  icases stateDiff_nonces_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_empty c _ previous v xs payload rest head dq d enc major hd depth hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hprogress⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %(some ∅) [Hfield Hnew]
  · iframe Hfield
    unfold FeltMapResult
    iexact Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  have bound : sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by
    simpa only [List.length_cons, Nat.add_comm payload.length 1, Int.natCast_add, Int.natCast_one] using Hnext.2.1
  exact ⟨Hprogress.1, bound, Hprogress.2, .nonces (feltMap_empty_spec d head payload rest enc major depth)⟩

/-- Empty DeployedContracts is allocated, distinct from nil; only that field changes.
    All map-head widths and suffixes are covered with restored depth. -/
theorem wp_stateDiff_deployed_empty (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeployedContracts" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with deployed := some ∅} ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .deployed old (head :: payload) rest {old with deployed := some ∅}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  icases stateDiff_deployed_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_empty c _ previous v xs payload rest head dq d enc major hd depth hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hprogress⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %(some ∅) [Hfield Hnew]
  · iframe Hfield
    unfold FeltMapResult
    iexact Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  have bound : sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by
    simpa only [List.length_cons, Nat.add_comm payload.length 1, Int.natCast_add, Int.natCast_one] using Hnext.2.1
  exact ⟨Hprogress.1, bound, Hprogress.2, .deployed (feltMap_empty_spec d head payload rest enc major depth)⟩

/-- Empty DeclaredV1Classes is allocated, distinct from nil; only that field changes.
    All map-head widths and suffixes are covered with restored depth. -/
theorem wp_stateDiff_declaredV1_empty (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeclaredV1Classes" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with declaredV1 := some ∅} ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .declaredV1 old (head :: payload) rest {old with declaredV1 := some ∅}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  icases stateDiff_declaredV1_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_empty c _ previous v xs payload rest head dq d enc major hd depth hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hprogress⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %(some ∅) [Hfield Hnew]
  · iframe Hfield
    unfold FeltMapResult
    iexact Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  have bound : sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by
    simpa only [List.length_cons, Nat.add_comm payload.length 1, Int.natCast_add, Int.natCast_one] using Hnext.2.1
  exact ⟨Hprogress.1, bound, Hprogress.2, .declaredV1 (feltMap_empty_spec d head payload rest enc major depth)⟩

/-- Empty ReplacedClasses is allocated, distinct from nil; only that field changes.
    All map-head widths and suffixes are covered with restored depth. -/
theorem wp_stateDiff_replaced_empty (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"ReplacedClasses" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with replaced := some ∅} ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .replaced old (head :: payload) rest {old with replaced := some ∅}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  icases stateDiff_replaced_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_empty c _ previous v xs payload rest head dq d enc major hd depth hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hprogress⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %(some ∅) [Hfield Hnew]
  · iframe Hfield
    unfold FeltMapResult
    iexact Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  have bound : sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by
    simpa only [List.length_cons, Nat.add_comm payload.length 1, Int.natCast_add, Int.natCast_one] using Hnext.2.1
  exact ⟨Hprogress.1, bound, Hprogress.2, .replaced (feltMap_empty_spec d head payload rest enc major depth)⟩

/-- Empty MigratedClasses is allocated, distinct from nil; only that field changes.
    All map-head widths and suffixes are covered with restored depth. -/
theorem wp_stateDiff_migrated_empty (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #(structFieldRef StateDiff go!"MigratedClasses" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with migrated := some ∅} ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .migrated old (head :: payload) rest {old with migrated := some ∅}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  icases stateDiff_migrated_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readMigratedClasses_empty c _ previous v xs payload rest head dq d enc major hd depth hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hprogress⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %(some ∅) [Hfield Hnew]
  · iframe Hfield
    unfold MigratedClassesResult
    iexact Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  have bound : sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by
    simpa only [List.length_cons, Nat.add_comm payload.length 1, Int.natCast_add, Int.natCast_one] using Hnext.2.1
  exact ⟨Hprogress.1, bound, Hprogress.2, .migrated (migratedClasses_empty_spec d head payload rest enc major depth)⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_storage_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_nonces_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_deployed_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_declaredV1_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_replaced_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_migrated_empty
