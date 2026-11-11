import ProofCheck.SierraEntriesOwnership

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

def sierraEntriesFieldReader : SierraEntriesField → GoString
  | .constructors => cborReadSierraEntryPointValues
  | .external => cborReadSierraEntryPointValues
  | .l1Handler => cborReadSierraEntryPointValues

/-- Successful field calls publish exact values and preserve unrelated fields. -/
theorem wp_sierraEntries_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : SierraEntriesValue) (field : SierraEntriesField) (v : cursor.Cursor)
    (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : SierraEntriesFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SierraEntriesOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (sierraEntriesFieldReader field))) (Val #c)) (Val #(structFieldRef SierraEntryPointsByType (sierraEntriesFieldName field) dst)))
    {{ RET #interface.nil; SierraEntriesOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases valid with
  | @constructors old item rest value valid =>
    icases sierraEntries_constructors_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [sierraEntriesFieldReader, sierraEntriesFieldName]
    wp_apply wp_readSierraEntryValues_valid validator c (structFieldRef SierraEntryPointsByType go!"Constructor" dst) slice v
      xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @external old item rest value valid =>
    icases sierraEntries_external_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [sierraEntriesFieldReader, sierraEntriesFieldName]
    wp_apply wp_readSierraEntryValues_valid validator c (structFieldRef SierraEntryPointsByType go!"External" dst) slice v
      xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @l1Handler old item rest value valid =>
    icases sierraEntries_l1Handler_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [sierraEntriesFieldReader, sierraEntriesFieldName]
    wp_apply wp_readSierraEntryValues_valid validator c (structFieldRef SierraEntryPointsByType go!"L1Handler" dst) slice v
      xs (item ++ rest) rest value dq d valid hd hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    isimp only [List.length_append, Nat.add_sub_cancel] at Hc
    simp only [List.length_append, Nat.add_sub_cancel] at Hnext
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_sierraEntries_field_valid
