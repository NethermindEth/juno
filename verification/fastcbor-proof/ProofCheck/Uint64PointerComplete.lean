import ProofCheck.Uint64PointerRead
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

/-- The concrete nil guard precisely matches absence in the abstract value;
    a present zero cannot be mistaken for nil. -/
theorem uint64PointerResult_null_iff (p : Loc) (value : Option w64) :
    Uint64PointerResult (GF := GF) p value ⊢ ⌜p = null ↔ value = none⌝ := by
  cases value with
  | none =>
    isimp only [Uint64PointerResult]
    iintro %hp
    ipureintro
    simp [hp]
  | some value =>
    isimp only [Uint64PointerResult]
    iintro ⟨_, %hn⟩
    ipureintro
    simp [hn]

/-- Caller-visible destination and cursor state for each independent outcome. -/
def Uint64PointerReadOwned (c dst old : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : Uint64PointerOutcome → IProp GF
  | .success value rest => iprop(∃ (item : List w8) (p : Loc), dst ↦ p ∗ Uint64PointerResult p value ∗
      c ↦ {v with pos' := v.pos' + W64 item.length} ∗
      ⌜result = interface.nil ∧ wire = item ++ rest ∧
        sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
        sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure reason => iprop(∃ err : Loc, dst ↦ old ∗ c ↦ v ∗
      ExpectedFailureError err v.pos' (W8 0) reason (expectedFailureKind reason kinds) ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝)

/-- Actual nullable uint64 decoding for every byte input and every prior pointer.
    Initialized cursor globals supply diagnostic categories. An old pointee is
    neither required nor consumed, so caller ownership of it can be framed across
    null clearing, replacement, and failure alike. -/
theorem wp_readUint64Pointer_complete (c dst old : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64Pointer)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (outcome : Uint64PointerOutcome) (kinds : FeltErrorKinds),
       FeltErrorGlobals kinds ∗ ⌜Uint64PointerSpec wire outcome⌝ ∗
       Uint64PointerReadOwned c dst old v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply cursor_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  rcases uint64Pointer_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
  · wp_apply wp_readUint64Pointer_valid c dst old v xs item rest value dq valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] with ⟨%p, Hdst, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists (Uint64PointerOutcome.success value rest), kinds
    iframe Hglobals
    isplitr
    · ipureintro; exact ⟨item, split, valid⟩
    isimp only [Uint64PointerReadOwned]
    iexists item, p
    iframe Hdst Hvalue Hc
    ipureintro; exact ⟨trivial, split, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup (GF := GF) kinds (.expected (W8 0) reason) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_readUint64Pointer_failure c dst old v xs wire dq reason (expectedFailureKind reason kinds)
      failure hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists (Uint64PointerOutcome.failure reason), kinds
    iframe Hglobals
    isplitr
    · ipureintro; exact failure
    isimp only [Uint64PointerReadOwned]
    iexists err
    iframe Hdst Hc Herr
    ipureintro; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.uint64PointerResult_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64Pointer_complete
