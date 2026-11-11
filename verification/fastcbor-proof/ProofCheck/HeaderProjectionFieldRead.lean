import ProofCheck.HeaderProjectionFelts
import ProofCheck.HeaderProjectionScalars

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

def headerProjectionFieldReader : HeaderProjectionField → GoString
  | .hash | .root => cborReadFelt
  | .count => cborReadUint64
  | .timestamp => cborReadUint64Pointer

def headerProjectionFieldRef (kind : HeaderProjectionKind) (field : HeaderProjectionField) (dst : Loc) : Loc :=
  match kind with
  | .hash => structFieldRef headerHashProjection (headerProjectionFieldName field) dst
  | .root => structFieldRef headerGlobalStateRootProjection (headerProjectionFieldName field) dst
  | .count => structFieldRef headerTransactionCountProjection (headerProjectionFieldName field) dst
  | .timestamp => structFieldRef headerTimestampProjection (headerProjectionFieldName field) dst
  | .hashAndRoot => structFieldRef headerHashAndStateRootProjection (headerProjectionFieldName field) dst

/-- Every selected field of every Header projection realizes its independent
    successful value relation. Concrete field addresses are derived from the
    actual translated layout; all other fields and input ownership survive. -/
theorem wp_headerProjectionField_valid (d : Nat) (kind : HeaderProjectionKind) (field : HeaderProjectionField)
    (c dst : Loc) (old final : HeaderProjectionValue) (v : cursor.Cursor) (xs wire rem : List w8) (dq : DFrac)
    (member : field ∈ headerProjectionFields kind) (valid : HeaderProjectionFieldValid d field old wire final)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (headerProjectionFieldReader field))) (Val #c))
        (Val #(headerProjectionFieldRef (GF := GF) kind field dst)))
    {{ RET #interface.nil; HeaderProjectionOwned kind dst final ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  cases valid with
  | hash valid =>
    cases kind <;> simp [headerProjectionFields] at member
    case hash =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨Howned, Hc, Hs⟩
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      have hread := wp_headerProjectionFelt_valid (GF := GF) .hash c dst h old v xs wire rem _ dq d hd depth valid hp hbytes
      simp only [headerProjectionFeltRef] at hread
      wp_apply hread $$ [Hh Hchildren Hc Hs] as %p ⟨Hh, Hchildren, Hc, Hs, %Hnext⟩
      · iframe Hh Hchildren Hc Hs
      iapply HΦ
      iframe Hc Hs
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists headerProjectionSetFeltPointer .hash h p
        isimp only [headerProjectionSetFeltValue] at Hchildren
        iframe Hh Hchildren
      ipureintro
      exact Hnext
    case hashAndRoot =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨Howned, Hc, Hs⟩
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      have hread := wp_headerProjectionFelt_valid (GF := GF) .pairHash c dst h old v xs wire rem _ dq d hd depth valid hp hbytes
      simp only [headerProjectionFeltRef] at hread
      wp_apply hread $$ [Hh Hchildren Hc Hs] as %p ⟨Hh, Hchildren, Hc, Hs, %Hnext⟩
      · iframe Hh Hchildren Hc Hs
      iapply HΦ
      iframe Hc Hs
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists headerProjectionSetFeltPointer .pairHash h p
        isimp only [headerProjectionSetFeltValue] at Hchildren
        iframe Hh Hchildren
      ipureintro
      exact Hnext
  | root valid =>
    cases kind <;> simp [headerProjectionFields] at member
    case root =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨Howned, Hc, Hs⟩
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      have hread := wp_headerProjectionFelt_valid (GF := GF) .root c dst h old v xs wire rem _ dq d hd depth valid hp hbytes
      simp only [headerProjectionFeltRef] at hread
      wp_apply hread $$ [Hh Hchildren Hc Hs] as %p ⟨Hh, Hchildren, Hc, Hs, %Hnext⟩
      · iframe Hh Hchildren Hc Hs
      iapply HΦ
      iframe Hc Hs
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists headerProjectionSetFeltPointer .root h p
        isimp only [headerProjectionSetFeltValue] at Hchildren
        iframe Hh Hchildren
      ipureintro
      exact Hnext
    case hashAndRoot =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨Howned, Hc, Hs⟩
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      have hread := wp_headerProjectionFelt_valid (GF := GF) .pairRoot c dst h old v xs wire rem _ dq d hd depth valid hp hbytes
      simp only [headerProjectionFeltRef] at hread
      wp_apply hread $$ [Hh Hchildren Hc Hs] as %p ⟨Hh, Hchildren, Hc, Hs, %Hnext⟩
      · iframe Hh Hchildren Hc Hs
      iapply HΦ
      iframe Hc Hs
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists headerProjectionSetFeltPointer .pairRoot h p
        isimp only [headerProjectionSetFeltValue] at Hchildren
        iframe Hh Hchildren
      ipureintro
      exact Hnext
  | count valid =>
    cases kind <;> simp [headerProjectionFields] at member
    case count =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨Howned, Hc, Hs⟩
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      isimp only [HeaderProjectionCells] at Hh
      wp_apply wp_headerProjectionCount_valid c dst h old v xs wire rem _ dq valid hp hbytes
        $$ [Hh Hchildren Hc Hs] with ⟨Hh, Hchildren, Hc, Hs, %Hnext⟩
      · iframe Hh Hchildren Hc Hs
      iapply HΦ
      iframe Hc Hs
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists _
        isimp only [HeaderProjectionCells]
        iframe Hh Hchildren
      ipureintro
      exact Hnext
  | timestamp valid =>
    cases kind <;> simp [headerProjectionFields] at member
    case timestamp =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨Howned, Hc, Hs⟩
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      isimp only [HeaderProjectionCells] at Hh
      wp_apply wp_headerProjectionTimestamp_valid c dst h old v xs wire rem _ dq valid hp hbytes
        $$ [Hh Hchildren Hc Hs] with ⟨%p, Hh, Hchildren, Hc, Hs, %Hnext⟩
      · iframe Hh Hchildren Hc Hs
      iapply HΦ
      iframe Hc Hs
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists {h with Timestamp' := p}
        isimp only [HeaderProjectionCells]
        iframe Hh Hchildren
      ipureintro
      exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionField_valid
