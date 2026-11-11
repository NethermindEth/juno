import ProofCheck.SegmentArrayRejected
import ProofCheck.FeltValuesOwnership

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

theorem wp_readSegmentArray_rejected_of_record (validator : Utf8ValidatorContract (GF := GF))
    (d : Nat) (next : d < 32 → SegmentRecordRejectedContract (GF := GF) (d + 1))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (consumed offset extraDepth : Nat) (reason : SegmentArrayFailureReason)
    (failure : SegmentArraySpec d wire (.failure consumed offset extraDepth reason))
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengthsValues)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SegmentArrayFailureOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.depth' + W64 extraDepth) = d + extraDepth ∧ d + extraDepth ≤ 32⌝ }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply cursor_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hbounds := segmentArray_failure_bounds d hdepth consumed offset extraDepth wire reason failure
  have Hdepth := Hbounds.2.2
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have Hprogress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
      sint.Z (v.depth' + W64 extraDepth) = d + extraDepth ∧ d + extraDepth ≤ 32 := by
    constructor; · word
    constructor; · word
    constructor; · word
    constructor; · word
    constructor; · word
    exact Hdepth
  have hz (x : w64) : x + W64 0 = x := by word
  cases failure with
  | outer hn hu rejected =>
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 128) _) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_readSegmentArray_outer_failure c dst old v xs wire dq _ _ hn hu rejected hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    isimp only [Int.natCast_zero, hz] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Herr]
    · unfold SegmentArrayFailureOwned
      iexists kinds
      iframe Hglobals Herr
    · ipureintro; simpa only [Int.natCast_zero, hz] using Hprogress
  | @count _ head payload tail argument count enc major size short =>
    have Hhead := feltValues_head_input v xs payload tail head Hlen.1 hp hbytes
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 128) .eof) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind] at Hkind
    wp_apply wp_readSegmentArray_count c dst old v xs payload tail head argument dq kinds.truncatedKind
      enc major hp.1 (by omega) (by omega) Hhead.2.1 Hhead.2.2
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    isimp only [Int.natCast_zero, hz] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Herr]
    · unfold SegmentArrayFailureOwned
      iexists kinds
      iframe Hglobals Herr
    · ipureintro; simpa only [Int.natCast_zero, hz] using Hprogress
  | @depth _ head payload tail argument count enc major size fit deep =>
    have Hhead := feltValues_head_input v xs payload tail head Hlen.1 hp hbytes
    ihave #Hkind := feltErrorGlobals_lookup kinds .depth $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_readSegmentArray_depth c dst old v xs payload tail head argument dq kinds.depthKind
      enc major hp.1 (by omega) (by omega) (by omega) Hhead.2.1 Hhead.2.2
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, %args, Herr, Hargs⟩
    · iframe Hkind Hdst Hc Hs
    isimp only [Int.natCast_zero, hz] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Herr Hargs]
    · unfold SegmentArrayFailureOwned
      iexists kinds, args
      iframe Hglobals Herr Hargs
    · ipureintro; simpa only [Int.natCast_zero, hz] using Hprogress
  | @element _ head payload tail argument done retained consumed offset childDepth reason count enc major size fit depth rejected =>
    have harg : argument = W64 count := by word
    subst argument
    wp_apply wp_readSegmentArray_array_rejected_of_record validator c dst old v xs payload tail head dq d (next depth) count consumed offset
      done retained childDepth reason rejected enc major hd depth fit hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hprivate, Hc, Hs, Herr, %Hstep⟩
    · iframe Hdst Hc Hs
    have hdepthWord : v.depth' + W64 1 + W64 childDepth = v.depth' + W64 (1 + childDepth) := by word
    isimp only [hdepthWord] at Hc
    isimp only [segmentArrayFailureOwned_element, Int.natCast_add, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one] using Hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegmentArray_rejected_of_record
