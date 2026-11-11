import ProofCheck.DeprecatedClassOwnership

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

def deprecatedClassFieldReader : DeprecatedClassField → GoString
  | .abi => cborReadRawMessage
  | .externals => cborReadDeprecatedEntryPointValues
  | .l1Handlers => cborReadDeprecatedEntryPointValues
  | .constructors => cborReadDeprecatedEntryPointValues
  | .program => cborReadString

/-- Successful field calls publish exact values and preserve unrelated fields. -/
theorem wp_deprecatedClass_field_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : DeprecatedClassValue) (field : DeprecatedClassField) (v : cursor.Cursor)
    (xs item rest : List w8) (dq : DFrac) (d : Nat)
    (valid : DeprecatedClassFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       DeprecatedClassOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (deprecatedClassFieldReader field))) (Val #c)) (Val #(structFieldRef DeprecatedCairoClass (deprecatedClassFieldName field) dst)))
    {{ RET #interface.nil; DeprecatedClassOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases valid with
  | @abi old item rest value valid =>
    icases deprecatedClass_abi_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readRawMessage_valid c (structFieldRef DeprecatedCairoClass go!"Abi" dst) slice v
      xs item rest value dq valid hp hbytes
      $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %out %value [Hfield Hnew]
    · iframe Hfield Hnew
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @externals old item rest value valid =>
    icases deprecatedClass_externals_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readDeprecatedEntryValues_valid validator c (structFieldRef DeprecatedCairoClass go!"Externals" dst) slice v
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
  | @l1Handlers old item rest value valid =>
    icases deprecatedClass_l1Handlers_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readDeprecatedEntryValues_valid validator c (structFieldRef DeprecatedCairoClass go!"L1Handlers" dst) slice v
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
  | @constructors old item rest value valid =>
    icases deprecatedClass_constructors_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readDeprecatedEntryValues_valid validator c (structFieldRef DeprecatedCairoClass go!"Constructors" dst) slice v
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
  | @program old item rest value valid =>
    icases deprecatedClass_program_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    simp only [deprecatedClassFieldReader, deprecatedClassFieldName]
    wp_apply wp_readString_valid validator c (structFieldRef DeprecatedCairoClass go!"Program" dst) old.program v
      xs item rest value dq valid hp hbytes $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_deprecatedClass_field_valid
