import ProofCheck.SegmentRejectedContracts

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

theorem wp_segment_field_rejected_of_array (d : Nat) (arrays : SegmentArrayRejectedContract (GF := GF) d)
    (c dst : Loc) (old : SegmentValue) (field : SegmentField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (consumed offset extraDepth : Nat) (reason : SegmentFieldFailure)
    (failure : SegmentFieldRejected d field wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SegmentOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (segmentFieldReader field))) (Val #c)) (Val #(structFieldRef SegmentLengths (segmentFieldName field) dst)))
    {{ (result : GoInterface), RET #result; SegmentOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SegmentFieldError result (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have bounds := segmentField_rejected_bounds_of_array d depth (segment_failure_bounds_depth (32 - d) d depth rfl).2 _ _ _ _ _ _ failure
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have progress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by
    constructor; · word
    constructor; · word
    constructor <;> word
  cases failure with
  | @length _ wire why failure =>
    icases segment_length_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    simp only [segmentFieldReader, segmentFieldName]
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 0) why) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    have hread := wp_readScalar_failure (GF := GF) false c (structFieldRef SegmentLengths go!"Length" dst) old.length
      v xs wire dq why (expectedFailureKind why kinds) failure hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    wp_apply hread $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hkind Hfield Hc Hs
    ihave Howned := Hclose $$ %old.length Hfield
    isimp only [show segmentSetLength old old.length = old from segmentValue_eta old] at Howned
    isimp only [Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word,
      show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SegmentFieldError
      iexists err
      isplitr
      · ipureintro; rfl
      iexists kinds
      iframe Hglobals Herr
    ipureintro; simpa using progress
  | @children _ wire consumed offset extraDepth why failure =>
    icases segment_children_acc dst old $$ Howned with ⟨%slice, Hfield, Hprevious, Hclose⟩
    simp only [segmentFieldReader, segmentFieldName]
    wp_apply arrays c (structFieldRef SegmentLengths go!"Children" dst) slice
      v xs wire dq _ _ _ why failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hstep⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %slice %old.children [Hfield Hprevious]
    · iframe Hfield Hprevious
    isimp only [show segmentSetChildren old old.children = old from segmentValue_eta old] at Howned
    iapply HΦ
    iframe Howned Hc Hs
    isplitl [Herr]
    · unfold SegmentFieldError
      iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact progress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_segment_field_rejected_of_array
