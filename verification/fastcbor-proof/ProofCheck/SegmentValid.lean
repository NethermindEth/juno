import ProofCheck.SegmentRecordValidStep
import ProofCheck.SegmentArrayValidStep

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

/-- The actual mutually recursive readers realize all independently accepted
    encodings. Child correctness is discharged by the decreasing depth budget;
    only the existing key-validator premise remains. -/
theorem segment_valid_contracts_depth (validator : Utf8ValidatorContract (GF := GF)) (fuel : Nat) :
    ∀ d : Nat, d ≤ 32 → 32 - d = fuel →
      SegmentRecordValidContract (GF := GF) d ∧ SegmentArrayValidContract (GF := GF) d := by
  induction fuel with
  | zero =>
    intro d depth remaining
    have impossible : ¬ d < 32 := by omega
    exact ⟨wp_readSegment_record_valid_of_array validator d (fun inside => False.elim (impossible inside)),
      wp_readSegmentArray_valid_of_record d (fun inside => False.elim (impossible inside))⟩
  | succ fuel ih =>
    intro d depth remaining
    have next : d < 32 → SegmentRecordValidContract (GF := GF) (d + 1) ∧ SegmentArrayValidContract (GF := GF) (d + 1) := by
      intro inside
      exact ih (d + 1) (by omega) (by omega)
    exact ⟨wp_readSegment_record_valid_of_array validator d (fun inside => (next inside).2),
      wp_readSegmentArray_valid_of_record d (fun inside => (next inside).1)⟩

theorem wp_readSegment_record_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : SegmentValue) (v : cursor.Cursor)
    (xs item rem : List w8) (dq : DFrac) (d : Nat) (depth : d ≤ 32)
    (valid : SegmentRecordValid d old item rem final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SegmentOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengths)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; SegmentOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} :=
  (segment_valid_contracts_depth validator (32 - d) d depth rfl).1 c dst old final v xs item rem dq valid hd hp hbytes

theorem wp_readSegmentArray_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire rem : List w8) (values : Option (List SegmentValue)) (dq : DFrac) (d : Nat) (depth : d ≤ 32)
    (valid : SegmentArraySpec d wire (.success values rem)) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengthsValues)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil;
       dst ↦ out ∗ SegmentArrayResult out values ∗
       c ↦ {v with pos' := v.pos' + W64 (wire.length - rem.length : Nat)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (wire.length - rem.length : Nat)) = sint.Z v.pos' + (wire.length - rem.length : Nat) ∧
         sint.Z (v.pos' + W64 (wire.length - rem.length : Nat)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (wire.length - rem.length : Nat))) (sint.nat v.buf'.len) xs = rem⌝ }} :=
  (segment_valid_contracts_depth validator (32 - d) d depth rfl).2 c dst old v xs wire rem values dq valid hd hp hbytes


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segment_valid_contracts_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegment_record_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegmentArray_valid
