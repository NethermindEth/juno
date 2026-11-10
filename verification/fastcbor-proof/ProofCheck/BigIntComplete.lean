import ProofCheck.BigIntFailure
import ProofCheck.CursorInitialize

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

/-- Distinct immutable error messages cannot occupy the same object. -/
theorem cursor_error_objects_distinct (left right : Loc) (a b : GoString) (different : a ≠ b) :
    (left ↦□ errors.errorString.mk a : IProp GF) ⊢ right ↦□ errors.errorString.mk b -∗ ⌜left ≠ right⌝ := by
  iintro #Hleft #Hright
  by_cases same : left = right
  · subst right
    ihave %agree := typedPointsto_agree_wand left DFrac.discard DFrac.discard
      (errors.errorString.mk a) (errors.errorString.mk b) $$ Hleft Hright
    ipureintro
    exact False.elim (different (by cases agree; rfl))
  · ipureintro; exact same

/-- Actual initialization supplies pointer categories and proves that type
    errors are distinct from syntax, unsupported and truncation errors. -/
theorem cursor_initialized_bigInt_globals :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ⊢
      ∃ k : BigIntErrorKinds, BigIntErrorGlobals (GF := GF) k := by
  iintro #Hpkg
  ihave Hcontents := cursor_initialized_errors $$ Hpkg
  isimp only [CursorInitialized, CursorErrorGlobal] at Hcontents
  icases Hcontents with ⟨⟨%truncated, #Htruncated, #Etruncated⟩, ⟨%syntaxError, #Hsyntax, #Esyntax⟩,
    ⟨%typeError, #Htype, #Etype⟩, _, ⟨%unsupported, #Hunsupported, #Eunsupported⟩⟩
  ihave %HtruncatedNe := cursor_error_objects_distinct truncated typeError _ _ (by decide) $$ Etruncated Etype
  ihave %HsyntaxNe := cursor_error_objects_distinct syntaxError typeError _ _ (by decide) $$ Esyntax Etype
  ihave %HunsupportedNe := cursor_error_objects_distinct unsupported typeError _ _ (by decide) $$ Eunsupported Etype
  iexists (BigIntErrorKinds.mk truncated syntaxError typeError unsupported)
  isimp only [BigIntErrorGlobals, bigIntCategory]
  iframe Htruncated Hsyntax Htype Hunsupported
  ipureintro; exact ⟨HtruncatedNe, HsyntaxNe, HunsupportedNe⟩

/-- The observable result of a BigInt call: an independently specified integer
    and exact consumption, or a specified rejection with complete rollback. -/
def BigIntOutcome (c : Loc) (before : cursor.Cursor) (wire : List w8) (xs : List w8)
    (value : Loc) (result : GoInterface) : IProp GF := iprop(
  ∃ k : BigIntErrorKinds, BigIntErrorGlobals k ∗
    ((∃ (item rest : List w8) (number : Int),
      ⌜result = interface.nil ∧ wire = item ++ rest ∧ BigIntValid item number⌝ ∗
      c ↦ {before with pos' := before.pos' + W64 item.length} ∗ BigMagnitudeOwned value number ∗
      ⌜sint.Z (before.pos' + W64 item.length) = sint.Z before.pos' + item.length ∧
        sint.Z (before.pos' + W64 item.length) ≤ sint.Z before.buf'.len ∧
        subslice (sint.nat (before.pos' + W64 item.length)) (sint.nat before.buf'.len) xs = rest⌝) ∨
     (∃ (reason : BigIntFailureReason) (err : Loc),
      ⌜value = null ∧ result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        BigIntRejected wire reason⌝ ∗ c ↦ before ∗ BigIntFailureError err before.pos' reason k)))

/-- The actual public BigInt decoder on every byte input at any valid cursor
    offset. The independent encoding/rejection relations determine the result;
    no correctness premise for head, payload, magnitude or errors.Is remains. -/
theorem wp_bigInt_complete (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (value : Loc) (result : GoInterface), RET (PairV #value #result);
       v.buf' ↦*{dq} xs ∗ BigIntOutcome c v wire xs value result }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ Hpkg
  icases Hglobals with ⟨%k, #Hglobals⟩
  rcases bigInt_classify wire with ⟨item, rest, number, split, valid⟩ | ⟨reason, failure⟩
  · wp_apply wp_bigInt_valid c v xs item rest number dq valid hp (by simpa only [split] using hbytes)
      $$ [Hc Hs] as %value ⟨Hc, Hs, Howned, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    unfold BigIntOutcome
    iexists k
    iframe Hglobals
    ileft
    iexists item, rest, number
    iframe Hc Howned
    ipureintro; exact ⟨⟨rfl, split, valid⟩, Hnext⟩
  · wp_apply wp_bigInt_rejected c v xs wire dq reason k failure hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hglobals Hc Hs
    iapply HΦ
    iframe Hs
    unfold BigIntOutcome
    iexists k
    iframe Hglobals
    iright
    iexists reason, err
    iframe Hc Herr
    ipureintro; exact ⟨rfl, rfl, failure⟩

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.cursor_error_objects_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.cursor_initialized_bigInt_globals
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_complete
