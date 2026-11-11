import ProofCheck.FeltValuesOwnership
import ProofCheck.SierraEntryValuesRejected
import ProofCheck.SierraEntryValuesSpec

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Exact diagnostics retain categories supplied by actual initialization.
    Inner SierraEntryPoint failures keep their own complete field/key/skip diagnostics. -/
def SierraEntryValuesFailureOwned (err : Loc) (pos : w64) : SierraEntryValuesFailureReason → IProp GF
  | .expected reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      ExpectedFailureError err pos (W8 128) reason (expectedFailureKind reason kinds))
  | .count => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      err ↦ cursor.DecodeError.mk kinds.truncatedKind pos (go!"count exceeds the input") slice.nil)
  | .depth => iprop(∃ (kinds : FeltErrorKinds) (args : GoSlice), FeltErrorGlobals kinds ∗
      err ↦ cursor.DecodeError.mk kinds.depthKind pos (go!"nested more than %d levels") args ∗
      args ↦* [interface.mkOk go.int #(W64 32)])
  | .element reason => SierraEntryPointFailureOwned err pos reason

/-- Every independent rejection is realized by the actual value-slice reader.
    The caller's destination is unchanged. Outer failures roll back; inner
    failures retain array/map depth and consumption. Temporary records remain
    private, as established by wp_readSierraEntryValues_array_rejected. -/
theorem wp_readSierraEntryValues_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d consumed offset extraDepth : Nat) (reason : SierraEntryValuesFailureReason)
    (failure : SierraEntryValuesSpec d wire (.failure consumed offset extraDepth reason))
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSierraEntryPointValues)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SierraEntryValuesFailureOwned err (v.pos' + W64 offset) reason ∗
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
  have Hbounds := sierraEntryValues_failure_bounds d consumed offset extraDepth wire reason failure
  have Hdepth := sierraEntryValues_failure_depth d consumed offset extraDepth wire reason hdepth failure
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
    wp_apply wp_readSierraEntryValues_outer_failure c dst old v xs wire dq _ _ hn hu rejected hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    isimp only [Int.natCast_zero, hz] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Herr]
    · unfold SierraEntryValuesFailureOwned
      iexists kinds
      iframe Hglobals Herr
    · ipureintro; simpa only [Int.natCast_zero, hz] using Hprogress
  | @count head payload tail argument count enc major size short =>
    have Hhead := feltValues_head_input v xs payload tail head Hlen.1 hp hbytes
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 128) .eof) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind] at Hkind
    wp_apply wp_readSierraEntryValues_count c dst old v xs payload tail head argument dq kinds.truncatedKind
      enc major hp.1 (by omega) (by omega) Hhead.2.1 Hhead.2.2
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    isimp only [Int.natCast_zero, hz] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Herr]
    · unfold SierraEntryValuesFailureOwned
      iexists kinds
      iframe Hglobals Herr
    · ipureintro; simpa only [Int.natCast_zero, hz] using Hprogress
  | @depth head payload tail argument count enc major size fit deep =>
    have Hhead := feltValues_head_input v xs payload tail head Hlen.1 hp hbytes
    ihave #Hkind := feltErrorGlobals_lookup kinds .depth $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_readSierraEntryValues_depth c dst old v xs payload tail head argument dq kinds.depthKind
      enc major hp.1 (by omega) (by omega) (by omega) Hhead.2.1 Hhead.2.2
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, %args, Herr, Hargs⟩
    · iframe Hkind Hdst Hc Hs
    isimp only [Int.natCast_zero, hz] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    isplitl [Herr Hargs]
    · unfold SierraEntryValuesFailureOwned
      iexists kinds, args
      iframe Hglobals Herr Hargs
    · ipureintro; simpa only [Int.natCast_zero, hz] using Hprogress
  | @element head payload tail argument done retained consumed offset entered reason count enc major size fit depth rejected =>
    have harg : argument = W64 count := by word
    subst argument
    wp_apply wp_readSierraEntryValues_array_rejected validator c dst old v xs payload tail head dq d count consumed offset
      done retained entered reason rejected enc major hd depth fit hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hprivate, Hc, Hs, Herr, %Hstep⟩
    · iframe Hdst Hc Hs
    have hdepthWord : (if entered then v.depth' + W64 1 + W64 1 else v.depth' + W64 1) =
        v.depth' + W64 (1 + (if entered then 1 else 0 : Nat)) := by cases entered <;> simp <;> word
    isimp only [hdepthWord] at Hc
    isimp only [SierraEntryValuesFailureOwned, Int.natCast_add, Int.natCast_one] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one] using Hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSierraEntryValues_rejected
