import ProofCheck.SegmentDiagnostic
import ProofCheck.SegmentStorage

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

/- Temporary induction predicates describe actual calls, not assumed helpers.
   The final simultaneous depth induction discharges both contracts. -/
def SegmentRecordValidContract (d : Nat) : Prop :=
  ∀ (c dst : Loc) (old final : SegmentValue) (v : cursor.Cursor)
    (xs item rem : List w8) (dq : DFrac)
    (valid : SegmentRecordValid d old item rem final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem),
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SegmentOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengths)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; SegmentOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }}

def SegmentArrayValidContract (d : Nat) : Prop :=
  ∀ (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire rem : List w8) (values : Option (List SegmentValue)) (dq : DFrac)
    (valid : SegmentArraySpec d wire (.success values rem)) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire),
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengthsValues)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil;
       dst ↦ out ∗ SegmentArrayResult out values ∗
       c ↦ {v with pos' := v.pos' + W64 (wire.length - rem.length : Nat)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (wire.length - rem.length : Nat)) = sint.Z v.pos' + (wire.length - rem.length : Nat) ∧
         sint.Z (v.pos' + W64 (wire.length - rem.length : Nat)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (wire.length - rem.length : Nat))) (sint.nat v.buf'.len) xs = rem⌝ }}


end github_com.NethermindEth.juno.core
end Perennial
