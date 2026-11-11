import ProofCheck.SierraEntryPointOwnership

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

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

def SierraEntryPointFieldFailureError (err : Loc) (pos : w64) (reason : SierraEntryPointFieldFailure) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .scalar reason => ExpectedFailureError err pos (W8 0) reason (expectedFailureKind reason kinds)
  | .felt reason => FeltFailureError err pos reason (feltFailureKind reason kinds)

def SierraEntryPointFieldOutcome (c dst : Loc) (old : SierraEntryPointValue) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (field : SierraEntryPointField) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : SierraEntryPointValue), SierraEntryPointOwned dst final ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ SierraEntryPointFieldValid d field old item rest final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : SierraEntryPointFieldFailure) (err : Loc), SierraEntryPointOwned dst old ∗ c ↦ v ∗
    SierraEntryPointFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      SierraEntryPointFieldRejected d field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

def sierraEntryPointFieldReaderName : SierraEntryPointField → GoString
  | .index => cborReadUint64
  | .selector => cborReadFelt

/-- Both actual field calls preserve the other field and return the complete
    independently specified value/error outcome, using initialized categories. -/
theorem wp_sierraEntryPoint_field_complete (c dst : Loc) (old : SierraEntryPointValue) (field : SierraEntryPointField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SierraEntryPointOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (sierraEntryPointFieldReaderName field))) (Val #c)) (Val #(structFieldRef SierraEntryPoint (sierraEntryPointFieldName field) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       SierraEntryPointFieldOutcome c dst old v xs wire d field kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  cases field with
  | index =>
    icases sierraEntryPoint_index_acc dst old $$ Howned with ⟨Hfield, Hclose⟩
    have hread := wp_readScalar_complete (GF := GF) false c (structFieldRef SierraEntryPoint go!"Index" dst)
      old.index v xs wire dq kinds hp hbytes
    simp only [Bool.false_eq_true, ite_false] at hread
    simp only [sierraEntryPointFieldReaderName, sierraEntryPointFieldName]
    wp_apply hread $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
    · iframe Hglobals Hfield Hc Hs
    isimp only [ScalarOutcome] at Houtcome
    icases Houtcome with (Hsuccess | Hfailure)
    · icases Hsuccess with ⟨%item, %rest, %value, Hfield, Hc, %Hresult⟩
      ihave Howned := Hclose $$ %value Hfield
      iapply HΦ
      iframe Hs
      iexists kinds
      iframe Hglobals
      unfold SierraEntryPointFieldOutcome
      ileft
      iexists item, rest, {old with index := value}
      iframe Howned Hc
      ipureintro
      exact ⟨Hresult.1, Hresult.2.1, .index Hresult.2.2.1, Hresult.2.2.2⟩
    · icases Hfailure with ⟨%reason, %err, Hfield, Hc, Herr, %Hresult⟩
      ihave Howned := Hclose $$ %old.index Hfield
      iapply HΦ
      iframe Hs
      iexists kinds
      iframe Hglobals
      unfold SierraEntryPointFieldOutcome
      iright
      iexists 0, SierraEntryPointFieldFailure.scalar reason, err
      isimp only [SierraEntryPointFieldFailureError, Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word]
      iframe Howned Hc Herr
      ipureintro
      exact ⟨Hresult.1, .index Hresult.2, by omega, hp.2⟩
  | selector =>
    icases sierraEntryPoint_selector_acc dst old $$ Howned with ⟨%ptr, Hfield, Hselector, Hclose⟩
    simp only [sierraEntryPointFieldReaderName, sierraEntryPointFieldName]
    wp_apply +noauto wp_readFelt_complete c (structFieldRef SierraEntryPoint go!"Selector" dst) ptr v xs wire dq d kinds
      hd hdepth hp hbytes $$ [Hfield Hc Hs] as %result ⟨Hs, Houtcome⟩
    · iframe Hglobals Hfield Hc Hs
    isimp only [FeltOutcome] at Houtcome
    icases Houtcome with (Hsuccess | Hfailure)
    · icases Hsuccess with ⟨%item, %rest, %value, %out, Hfield, Hnew, Hc, %Hresult⟩
      ihave Howned := Hclose $$ %out %value [Hfield Hnew]
      · iframe Hfield Hnew
      iapply HΦ
      iframe Hs
      iexists kinds
      iframe Hglobals
      unfold SierraEntryPointFieldOutcome
      ileft
      iexists item, rest, {old with selector := value}
      iframe Howned Hc
      ipureintro
      exact ⟨Hresult.1, Hresult.2.1, .selector Hresult.2.2.1, Hresult.2.2.2⟩
    · icases Hfailure with ⟨%offset, %reason, %err, Hfield, Hc, Herr, %Hresult⟩
      ihave Howned := Hclose $$ %ptr %old.selector [Hfield Hselector]
      · iframe Hfield Hselector
      iapply HΦ
      iframe Hs
      iexists kinds
      iframe Hglobals
      unfold SierraEntryPointFieldOutcome
      iright
      iexists offset, SierraEntryPointFieldFailure.felt reason, err
      isimp only [SierraEntryPointFieldFailureError]
      iframe Howned Hc Herr
      ipureintro
      exact ⟨Hresult.1, .selector Hresult.2.1, Hresult.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_sierraEntryPoint_field_complete
