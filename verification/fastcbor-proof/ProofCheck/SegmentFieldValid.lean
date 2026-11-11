import ProofCheck.SegmentValidContracts

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def segmentFieldReader : SegmentField → GoString
  | .children => cborReadSegmentLengthsValues
  | .length => cborReadUint64

/-- Successful field calls publish exact values and preserve unrelated fields. -/
theorem wp_segment_field_valid_of_array (d : Nat) (arrays : SegmentArrayValidContract (GF := GF) d)
    (c dst : Loc) (old final : SegmentValue) (field : SegmentField) (v : cursor.Cursor)
    (xs item rest : List w8) (dq : DFrac)
    (valid : SegmentFieldValid d field old item rest final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SegmentOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (segmentFieldReader field))) (Val #c)) (Val #(structFieldRef SegmentLengths (segmentFieldName field) dst)))
    {{ RET #interface.nil; SegmentOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases valid with
  | @length _ old item rest value valid =>
    icases segment_length_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    simp only [segmentFieldReader, segmentFieldName]
    have hread := wp_readScalar_valid (GF := GF) false c (structFieldRef SegmentLengths go!"Length" dst)
      old.length v xs item rest value dq valid hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | @children _ old item rest value valid =>
    icases segment_children_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [segmentFieldReader, segmentFieldName]
    wp_apply arrays c (structFieldRef SegmentLengths go!"Children" dst) slice v
      xs (item ++ rest) rest value dq valid hd hp hbytes
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
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_segment_field_valid_of_array
