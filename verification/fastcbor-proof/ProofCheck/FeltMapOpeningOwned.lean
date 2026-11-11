import ProofCheck.FeltMapOpening
import ProofCheck.StateMapOpeningSpec
import ProofCheck.InitializationErrors

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

/-- All independent map-opening rejections use the actual initialized diagnostic
    and preserve the arbitrary reserve and old destination, before map allocation. -/
theorem wp_readFeltMapFrom_open_rejected (c dst old spare : Loc) (reserve : GoSlice) (extra : w64)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltMapFailureReason)
    (failure : FeltMapSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       spare ↦ reserve ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (@! cborReadFeltMapFrom)) (Val #c)) (Val #dst)) (Val #spare)) (Val #extra))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       spare ↦ reserve ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltMapFailureOwned err v.pos' reason }} := by
  iintro %Φ ⟨#Hpkg, Hspare, Hdst, Hc, Hs⟩ HΦ
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds (feltMapDiagnostic reason) $$ Hglobals
  have opening := (feltMap_zero_depth_failure_open d 0 0 wire reason depth failure).2.2
  wp_apply wp_readFeltMapFrom_open_failure c dst old spare reserve extra v xs wire dq d
    (feltMapDiagnostic reason) (feltFailureKind (feltMapDiagnostic reason) kinds) opening hd hp hbytes
    $$ [Hspare Hdst Hc Hs] as %err ⟨Hspare, Hdst, Hc, Hs, Herr⟩
  · iframe Hkind Hspare Hdst Hc Hs
  iapply HΦ
  iframe Hspare Hdst Hc Hs
  unfold FeltMapFailureOwned
  iexists kinds
  iframe Hglobals Herr

/-- All independent map-opening rejections use the actual initialized diagnostic
    and preserve the old destination through the ordinary wrapper, before map allocation. -/
theorem wp_readFeltMap_open_rejected (c dst old : Loc)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltMapFailureReason)
    (failure : FeltMapSpec d wire (.failure 0 0 0 reason))
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltMap)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ FeltMapFailureOwned err v.pos' reason }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_readFeltMapFrom_open_rejected c dst old spare_ptr slice.nil (W64 0) v xs wire dq d reason
    failure hd depth hp hbytes $$ [spare Hdst Hc Hs] as %err ⟨Hspare, Hdst, Hc, Hs, Herr⟩
  · iframe spare Hdst Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltMapFrom_open_rejected

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltMap_open_rejected
