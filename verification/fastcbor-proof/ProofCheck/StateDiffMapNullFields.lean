import ProofCheck.StateDiffErrors
import ProofCheck.FeltMapNull
import ProofCheck.StorageDiffsBoundary
import ProofCheck.MigratedClassesBoundary

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

/-- Null/undefined clears StorageDiffs to nil, preserving every sibling field
    and recording exact suffix/progress and the independent field contract. -/
theorem wp_stateDiff_storage_null (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStorageDiffs)) (Val #c)) (Val #(structFieldRef StateDiff go!"StorageDiffs" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with storage := none} ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .storage old [head] rest {old with storage := none}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  icases stateDiff_storage_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStorageDiffs_null c _ previous v xs head dq hp.1 hi hn
    $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs⟩
  · iframe Hfield Hc Hs
  ihave Hnew : (StorageDiffsResult map.nil none : IProp GF) $$ []
  · unfold StorageDiffsResult
    ipureintro; rfl
  ihave Howned := Hclose $$ %map.nil %none [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .storage (.nullish head rest hn)⟩

/-- Null/undefined clears Nonces to nil, preserving every sibling field
    and recording exact suffix/progress and the independent field contract. -/
theorem wp_stateDiff_nonces_null (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"Nonces" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with nonces := none} ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .nonces old [head] rest {old with nonces := none}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  icases stateDiff_nonces_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_null c _ previous v xs head dq hp.1 hi hn
    $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs⟩
  · iframe Hfield Hc Hs
  ihave Hnew : (FeltMapResult map.nil none : IProp GF) $$ []
  · unfold FeltMapResult
    ipureintro; rfl
  ihave Howned := Hclose $$ %map.nil %none [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .nonces (.nullish head rest hn)⟩

/-- Null/undefined clears DeployedContracts to nil, preserving every sibling field
    and recording exact suffix/progress and the independent field contract. -/
theorem wp_stateDiff_deployed_null (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeployedContracts" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with deployed := none} ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .deployed old [head] rest {old with deployed := none}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  icases stateDiff_deployed_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_null c _ previous v xs head dq hp.1 hi hn
    $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs⟩
  · iframe Hfield Hc Hs
  ihave Hnew : (FeltMapResult map.nil none : IProp GF) $$ []
  · unfold FeltMapResult
    ipureintro; rfl
  ihave Howned := Hclose $$ %map.nil %none [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .deployed (.nullish head rest hn)⟩

/-- Null/undefined clears DeclaredV1Classes to nil, preserving every sibling field
    and recording exact suffix/progress and the independent field contract. -/
theorem wp_stateDiff_declaredV1_null (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"DeclaredV1Classes" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with declaredV1 := none} ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .declaredV1 old [head] rest {old with declaredV1 := none}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  icases stateDiff_declaredV1_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_null c _ previous v xs head dq hp.1 hi hn
    $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs⟩
  · iframe Hfield Hc Hs
  ihave Hnew : (FeltMapResult map.nil none : IProp GF) $$ []
  · unfold FeltMapResult
    ipureintro; rfl
  ihave Howned := Hclose $$ %map.nil %none [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .declaredV1 (.nullish head rest hn)⟩

/-- Null/undefined clears ReplacedClasses to nil, preserving every sibling field
    and recording exact suffix/progress and the independent field contract. -/
theorem wp_stateDiff_replaced_null (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #(structFieldRef StateDiff go!"ReplacedClasses" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with replaced := none} ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .replaced old [head] rest {old with replaced := none}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  icases stateDiff_replaced_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readFeltMap_null c _ previous v xs head dq hp.1 hi hn
    $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs⟩
  · iframe Hfield Hc Hs
  ihave Hnew : (FeltMapResult map.nil none : IProp GF) $$ []
  · unfold FeltMapResult
    ipureintro; rfl
  ihave Howned := Hclose $$ %map.nil %none [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .replaced (.nullish head rest hn)⟩

/-- Null/undefined clears MigratedClasses to nil, preserving every sibling field
    and recording exact suffix/progress and the independent field contract. -/
theorem wp_stateDiff_migrated_null (c dst : Loc) (old : StateDiffValue) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateDiffOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMigratedClasses)) (Val #c)) (Val #(structFieldRef StateDiff go!"MigratedClasses" dst)))
    {{ RET #interface.nil; StateDiffOwned dst {old with migrated := none} ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest ∧
         StateDiffFieldValid d .migrated old [head] rest {old with migrated := none}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  icases stateDiff_migrated_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readMigratedClasses_null c _ previous v xs head dq hp.1 hi hn
    $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs⟩
  · iframe Hfield Hc Hs
  ihave Hnew : (MigratedClassesResult map.nil none : IProp GF) $$ []
  · unfold MigratedClassesResult
    ipureintro; rfl
  ihave Howned := Hclose $$ %map.nil %none [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  simp only [List.length_singleton, Int.natCast_one] at Hnext
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .migrated (.nullish head rest hn)⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_storage_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_nonces_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_deployed_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_declaredV1_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_replaced_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateDiff_migrated_null
