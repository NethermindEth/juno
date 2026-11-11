import ProofCheck.FeeUnitRead

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

/-- The type sentinel is the actual non-nil errorString created at startup. -/
theorem cursor_type_nonNil (kinds : FeltErrorKinds) :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
      FeltErrorGlobals (GF := GF) kinds ⊢ ⌜kinds.typeKind ≠ interface.nil⌝ := by
  iintro ⟨#Hpkg, Hglobals⟩
  ihave Herrors := cursor_initialized_errors $$ Hpkg
  isimp only [CursorInitialized, CursorErrorGlobal] at Herrors
  icases Herrors with ⟨_, _, ⟨%p, #Hcanonical, _⟩, _, _⟩
  isimp only [FeltErrorGlobals] at Hglobals
  icases Hglobals with ⟨_, Htype⟩
  ihave %agree := typedPointsto_agree_wand (globalAddr cursor.ErrType) DFrac.discard DFrac.discard
    (interface.mkOk (go.PointerType errors.errorString.ty) #p) kinds.typeKind $$ Hcanonical Htype
  ipureintro
  rw [← agree]
  simp

/-- Overflow returns a bare sentinel, so it has no allocated diagnostic offset.
    Unsigned parse errors retain their exact diagnostic at the original cursor. -/
def FeeUnitError (result : GoInterface) (pos : w64) (kinds : FeltErrorKinds) : FeeUnitFailureReason → IProp GF
  | .expected reason => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ExpectedFailureError err pos (W8 0) reason (expectedFailureKind reason kinds))
  | .overflow => iprop(⌜result = kinds.typeKind ∧ result ≠ interface.nil⌝)

theorem feeUnitError_nonNil (result : GoInterface) (pos : w64) (kinds : FeltErrorKinds) (reason : FeeUnitFailureReason) :
    FeeUnitError (GF := GF) result pos kinds reason ⊢ ⌜result ≠ interface.nil⌝ := by
  iintro Herr
  cases reason with
  | expected reason =>
    isimp only [FeeUnitError] at Herr
    icases Herr with ⟨%err, %Hresult, _⟩
    ipureintro; rw [Hresult]; simp
  | overflow =>
    isimp only [FeeUnitError] at Herr
    icases Herr with %Hresult
    ipureintro; exact Hresult.2

/-- Every independently rejected FeeUnit preserves the destination and input,
    with exact consumption and the initialized non-nil error representation. -/
theorem wp_readFeeUnit_rejected (c dst : Loc) (old : w8) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (consumed : Nat) (reason : FeeUnitFailureReason) (kinds : FeltErrorKinds)
    (failure : FeeUnitRejected wire consumed reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeeUnit)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result; dst ↦ old ∗
       c ↦ {v with pos' := v.pos' + W64 consumed} ∗ v.buf' ↦*{dq} xs ∗
       FeeUnitError result v.pos' kinds reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hglobals, Hdst, Hc, Hs⟩
  cases failure with
  | expected failure =>
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 0) _) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_readFeeUnit_expected c dst old v xs wire dq _ _ failure hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    have hz : v.pos' + W64 0 = v.pos' := by word
    isimp only [Int.natCast_zero, Int.add_zero, hz, FeeUnitError]
    iframe Hdst Hc Hs
    isplitl [Herr]
    · iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro; exact ⟨trivial, hp.2⟩
  | @overflow head payload rest argument enc major large =>
    ihave #Htype := feltErrorGlobals_lookup kinds (.expected (W8 0) (.typeMismatch (W8 32))) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind] at Htype
    ihave %HnonNil := cursor_type_nonNil kinds $$ [Hglobals]
    · iframe Hglobals
      solve_pkg_init
    wp_apply wp_readFeeUnit_overflow c dst old v xs payload rest head argument dq kinds.typeKind enc major large hp hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hnext⟩
    · iframe Htype Hdst Hc Hs
    iapply HΦ
    isimp only [Int.natCast_add, Int.natCast_one]
    iframe Hdst Hc Hs
    isimp only [FeeUnitError]
    ipureintro
    exact ⟨⟨trivial, HnonNil⟩, by simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hnext.1, Hnext.2.1⟩

def FeeUnitReadOutcome (c dst : Loc) (old : w8) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : FeeUnitResult → IProp GF
  | .success value rest => iprop(∃ consumed : Nat,
      dst ↦ value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure consumed reason => iprop(dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      FeeUnitError result v.pos' kinds reason ∗
      ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len⌝)

/-- Actual FeeUnit decoding for every input and prior byte, with independent
    unique outcomes, initialized categories and no helper-correctness premise. -/
theorem wp_readFeeUnit_complete (c dst : Loc) (old : w8) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeeUnit)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (kinds : FeltErrorKinds) (outcome : FeeUnitResult),
       FeltErrorGlobals kinds ∗ ⌜FeeUnitSpec old wire outcome ∧ ∀ other, FeeUnitSpec old wire other → other = outcome⌝ ∗
       FeeUnitReadOutcome c dst old v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  obtain ⟨outcome, spec, unique⟩ := feeUnitSpec_total_unique old wire
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_readFeeUnit_valid c dst old v xs item rest value dq valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, FeeUnitResult.success value rest
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [FeeUnitReadOutcome]
    iexists item.length
    iframe Hdst Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure consumed reason =>
    wp_apply wp_readFeeUnit_rejected c dst old v xs wire dq consumed reason kinds spec hp hbytes
      $$ [Hdst Hc Hs] as %result ⟨Hdst, Hc, Hs, Herr, %Hnext⟩
    · iframe Hglobals Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, FeeUnitResult.failure consumed reason
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [FeeUnitReadOutcome]
    iframe Hdst Hc Herr
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.cursor_type_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.feeUnitError_nonNil
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeeUnit_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeeUnit_complete
