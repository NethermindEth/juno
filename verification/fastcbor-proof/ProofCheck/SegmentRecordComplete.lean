import ProofCheck.SegmentRejected

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

/-- Concrete ownership for the unique record outcome, including the exact
    partial tree and owned allocated diagnostic after rejection. -/
def SegmentRecordReadOwned (c dst : Loc) (v : cursor.Cursor) (xs wire : List w8) (d : Nat)
    (result : GoInterface) : SegmentRecordResult → IProp GF
  | .success value rest => iprop(
      SegmentOwned dst value ∗
      c ↦ {v with pos' := v.pos' + W64 (wire.length - rest.length : Nat)} ∗
      ⌜result = interface.nil ∧
        sint.Z (v.pos' + W64 (wire.length - rest.length : Nat)) = sint.Z v.pos' + (wire.length - rest.length : Nat) ∧
        sint.Z (v.pos' + W64 (wire.length - rest.length : Nat)) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 (wire.length - rest.length : Nat))) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure retained consumed offset extraDepth reason => iprop(∃ err : Loc,
      SegmentOwned dst retained ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      SegmentDiagnosticOwned err (v.pos' + W64 offset) reason ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
        sint.Z (v.depth' + W64 extraDepth) = d + extraDepth ∧ d + extraDepth ≤ 32⌝)

/-- All bytes and arbitrary owned initial trees are covered by the actual
    recursive record reader. Child correctness is proved, not assumed; the
    existing UTF-8 key-validator premise remains explicit. -/
theorem wp_readSegment_record_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : SegmentValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SegmentOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengths)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ outcome : SegmentRecordResult,
       ⌜SegmentRecordSpec d old wire outcome ∧
         ∀ other, SegmentRecordSpec d old wire other → other = outcome⌝ ∗
       SegmentRecordReadOwned c dst v xs wire d result outcome }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  obtain ⟨outcome, spec, unique⟩ := segmentRecordSpec_total_unique d old wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_readSegment_record_valid validator c dst old value v xs item rest dq d depth valid hd hp
      (hbytes.trans split) $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists SegmentRecordResult.success value rest
    isplitr
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    unfold SegmentRecordReadOwned
    have length : wire.length - rest.length = item.length := by rw [split]; simp
    isimp only [length]
    iframe Howned Hc
    ipureintro; exact ⟨trivial, Hstep⟩
  | failure retained consumed offset extraDepth reason =>
    have bounds := segmentRecordRejected_bounds d depth consumed offset extraDepth old retained wire reason spec
    wp_apply wp_readSegment_record_rejected_allocated validator c dst old retained v xs wire dq d consumed offset depth
      extraDepth reason spec hd hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hstep⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists SegmentRecordResult.failure retained consumed offset extraDepth reason
    isplitr
    · ipureintro; exact ⟨spec, unique⟩
    unfold SegmentRecordReadOwned
    iexists err
    iframe Howned Hc Herr
    ipureintro
    exact ⟨rfl, Hstep.1, Hstep.2.1, Hstep.2.2.1, Hstep.2.2.2, by word, bounds.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegment_record_complete
