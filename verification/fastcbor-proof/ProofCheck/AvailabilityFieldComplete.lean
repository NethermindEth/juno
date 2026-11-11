import ProofCheck.AvailabilityOwnership

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

def AvailabilityFieldFailureError (err : Loc) (pos : w64) (reason : AvailabilityFieldFailure) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .scalar reason => ExpectedFailureError err pos (W8 0) reason (expectedFailureKind reason kinds)

def AvailabilityFieldOutcome (c dst : Loc) (old : AvailabilityValue) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (field : AvailabilityField) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : AvailabilityValue), AvailabilityOwned dst final ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ AvailabilityFieldValid d field old item rest final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : AvailabilityFieldFailure) (err : Loc), AvailabilityOwned dst old ∗ c ↦ v ∗
    AvailabilityFieldFailureError err (v.pos' + W64 offset) reason kinds ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      AvailabilityFieldRejected d field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

def availabilityFieldReaderName (_field : AvailabilityField) : GoString := cborReadUint64

/-- Both actual counter calls preserve the other field and return the complete
    independently specified value/error outcome, using initialized categories. -/
theorem wp_availability_field_complete (c dst : Loc) (old : AvailabilityValue) (field : AvailabilityField) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat)
    (_hd : sint.Z v.depth' = d) (_hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       AvailabilityOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (availabilityFieldReaderName field))) (Val #c)) (Val #(structFieldRef DataAvailability (availabilityFieldName field) dst)))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       AvailabilityFieldOutcome c dst old v xs wire d field kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases availability_field_acc dst old field $$ Howned with ⟨Hfield, Hclose⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  have hread := wp_readScalar_complete (GF := GF) false c (structFieldRef DataAvailability (availabilityFieldName field) dst)
    (availabilityGet field old) v xs wire dq kinds hp hbytes
  simp only [Bool.false_eq_true, ite_false] at hread
  simp only [availabilityFieldReaderName]
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
    unfold AvailabilityFieldOutcome
    ileft
    iexists item, rest, availabilitySet field old value
    iframe Howned Hc
    ipureintro
    exact ⟨Hresult.1, Hresult.2.1, .scalar Hresult.2.2.1, Hresult.2.2.2⟩
  · icases Hfailure with ⟨%reason, %err, Hfield, Hc, Herr, %Hresult⟩
    ihave Howned := Hclose $$ %(availabilityGet field old) Hfield
    isimp only [availabilitySet_original] at Howned
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    unfold AvailabilityFieldOutcome
    iright
    iexists 0, AvailabilityFieldFailure.scalar reason, err
    isimp only [AvailabilityFieldFailureError, Int.natCast_zero, show v.pos' + W64 0 = v.pos' from by word]
    iframe Howned Hc Herr
    ipureintro
    exact ⟨Hresult.1, .scalar Hresult.2, by omega, hp.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_availability_field_complete
