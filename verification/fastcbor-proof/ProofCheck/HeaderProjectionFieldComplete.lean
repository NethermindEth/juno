import ProofCheck.HeaderProjectionFieldRejected
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

/-- The record loop can consume this all-input field contract without assuming
    a successful encoding. Success owns the independent decoded value and exact
    suffix; rejection owns the old value and a diagnostic at the specified offset. -/
def HeaderProjectionFieldOutcome (d : Nat) (kind : HeaderProjectionKind) (field : HeaderProjectionField)
    (c dst : Loc) (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : HeaderProjectionValue),
    HeaderProjectionOwned kind dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ HeaderProjectionFieldValid d field old item final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : HeaderProjectionFieldFailure) (err : Loc),
    HeaderProjectionOwned kind dst old ∗ c ↦ v ∗
    HeaderProjectionFieldError err (v.pos' + W64 offset) reason (headerProjectionFieldFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      HeaderProjectionFieldRejected d field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧ sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- All actual projection field helpers on arbitrary bytes. Initialized cursor
    state supplies the canonical diagnostic categories. No UTF-8 or other
    unproved helper contract is a premise of this result. -/
theorem wp_headerProjectionField_complete (d : Nat) (kind : HeaderProjectionKind) (field : HeaderProjectionField)
    (c dst : Loc) (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (member : field ∈ headerProjectionFields kind) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (headerProjectionFieldReader field))) (Val #c))
        (Val #(headerProjectionFieldRef (GF := GF) kind field dst)))
    {{ (result : GoInterface), RET #result; v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ HeaderProjectionFieldOutcome d kind field c dst old v xs wire kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  rcases headerProjectionField_classify d field old wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
  · wp_apply wp_headerProjectionField_valid d kind field c dst old final v xs item rest dq member valid hd depth hp (hbytes.trans split)
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    isimp only [HeaderProjectionFieldOutcome]
    ileft
    iexists item, rest, final
    iframe Howned Hc
    ipureintro
    exact ⟨trivial, split, valid, Hnext⟩
  · ihave #Hcategory : (headerProjectionFieldFailureAddr (GF := GF) reason ↦□ headerProjectionFieldFailureKind reason kinds : IProp GF) $$ []
    · cases reason with
      | felt reason =>
        isimp only [headerProjectionFieldFailureAddr, headerProjectionFieldFailureKind]
        iapply feltErrorGlobals_lookup kinds reason
        iexact Hglobals
      | scalar reason =>
        ihave #Hcategory := feltErrorGlobals_lookup kinds (.expected (W8 0) reason) $$ Hglobals
        isimp only [feltFailureAddr, feltFailureKind] at Hcategory
        isimp only [headerProjectionFieldFailureAddr, headerProjectionFieldFailureKind]
        iexact Hcategory
    wp_apply wp_headerProjectionField_rejected d offset kind field c dst old v xs wire dq reason
      (headerProjectionFieldFailureKind reason kinds) member failure hd depth hp hbytes
      $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hoffset⟩
    · iframe Hcategory Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    isimp only [HeaderProjectionFieldOutcome]
    iright
    iexists offset, reason, err
    iframe Howned Hc Herr
    ipureintro
    exact ⟨rfl, failure, Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionField_complete
