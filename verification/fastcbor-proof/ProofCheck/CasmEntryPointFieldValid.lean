import ProofCheck.CasmEntryPointOwnership

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

def casmEntryPointFieldReader : CasmEntryPointField → GoString
  | .offset => cborReadUint64
  | .builtins => cborReadStrings
  | .selector => cborReadFelt

/-- Successful field calls publish exact values and preserve unrelated fields. -/
theorem wp_casmEntryPoint_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : CasmEntryPointValue) (field : CasmEntryPointField) (v : cursor.Cursor)
    (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : CasmEntryPointFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       CasmEntryPointOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (casmEntryPointFieldReader field))) (Val #c)) (Val #(structFieldRef CasmEntryPoint (casmEntryPointFieldName field) dst)))
    {{ RET #interface.nil; CasmEntryPointOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases valid with
  | @offset old item rest value valid =>
    icases casmEntryPoint_offset_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    simp only [casmEntryPointFieldReader, casmEntryPointFieldName]
    have hread := wp_readScalar_valid (GF := GF) false c (structFieldRef CasmEntryPoint go!"Offset" dst)
      old.offset v xs item rest value dq valid hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @builtins old item rest value valid =>
    icases casmEntryPoint_builtins_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [casmEntryPointFieldReader, casmEntryPointFieldName]
    wp_apply wp_readStrings_valid validator c (structFieldRef CasmEntryPoint go!"Builtins" dst) slice v
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
  | @selector old item rest value valid =>
    icases casmEntryPoint_selector_acc dst old $$ Howned with ⟨%ptr, Hfield, Hprevious, Hclose⟩
    simp only [casmEntryPointFieldReader, casmEntryPointFieldName]
    wp_apply wp_readFelt_valid c (structFieldRef CasmEntryPoint go!"Selector" dst) ptr v
      xs item rest value dq d hd depth valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_casmEntryPoint_field_valid
