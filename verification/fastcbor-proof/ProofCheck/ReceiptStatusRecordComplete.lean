import ProofCheck.ReceiptStatusRecordValid
import ProofCheck.ReceiptStatusRecordRejected

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 12800000

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

/-- Concrete ownership and positions for an independently specified outcome.
    Success restores the enclosing depth; a rejected entered map remains open
    until the outer API discards its cursor and clears the destination. -/
def ReceiptStatusRecordOutcome (c dst : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (kinds : FeltErrorKinds) (result : GoInterface) : ReceiptStatusRecordResult → IProp GF
  | .success final rest => iprop(∃ consumed : Nat,
      ReceiptStatusOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest ∧
        consumed + rest.length = wire.length⌝)
  | .failure retained consumed offset entered reason => iprop(
      ReceiptStatusOwned dst retained ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + (if entered then W64 1 else W64 0)} ∗
      ReceiptStatusRecordError result (v.pos' + W64 offset) reason kinds ∗
      ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every independent record outcome is realized by the corresponding actual
    generated projection reader, including the optional tag policy and all
    failures. The existing UTF-8 validator contract is the only helper premise. -/
theorem wp_readReceiptStatus_record_spec (validator : Utf8ValidatorContract (GF := GF))
    (d : Nat) (c dst : Loc) (old : ReceiptStatusValue)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (kinds : FeltErrorKinds)
    (outcome : ReceiptStatusRecordResult) (spec : ReceiptStatusRecordSpec d old wire outcome)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadReceiptExecutionStatusProjection)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ReceiptStatusRecordOutcome c dst v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨#Hglobals, Howned, Hc, Hs⟩
  have hz (x : w64) : x + W64 0 = x := by word
  cases spec with
  | @success tagConsumed tagRest body rest final tag split valid =>
    subst tagRest
    wp_apply wp_readReceiptStatus_record_valid validator d c dst old final v xs wire body rest dq tagConsumed
      tag valid hd hp hbytes $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe
    iapply HΦ
    iframe Hs
    isimp only [ReceiptStatusRecordOutcome]
    iexists tagConsumed + body.length
    isimp only [Int.natCast_add]
    iframe Howned Hc
    ipureintro
    obtain ⟨tagBytes, splitTag, lengthTag⟩ := projectionTag_suffix wire (body ++ rest) tagConsumed .success tag
    refine ⟨trivial, ?_, Hnext.2.1, Hnext.2.2, ?_⟩
    · simpa only [Int.natCast_add, Int.add_assoc] using Hnext.1
    · rw [splitTag]
      simp only [List.length_append, lengthTag]
      omega
  | @malformed tagConsumed tagRest reason tag =>
    cases tag with
    | malformed tagged failure =>
      ihave #Hcategory := feltErrorGlobals_lookup kinds (.expected (W8 192) reason) $$ Hglobals
      isimp only [feltFailureAddr, feltFailureKind] at Hcategory
      wp_apply wp_readReceiptStatus_tag_malformed c dst old v xs wire dq reason (expectedFailureKind reason kinds)
        tagged failure hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr⟩
      · iframe Hcategory Howned Hc Hs
      iapply HΦ
      iframe Hs
      isimp only [ReceiptStatusRecordOutcome, Bool.false_eq_true, ite_false, Int.natCast_zero, Int.add_zero, hz]
      iframe Howned Hc
      isplitl [Herr]
      · isimp only [ReceiptStatusRecordError, ReceiptStatusRecordDiagnostic, receiptStatusRecordFailureKind]
        iexists err
        iframe Herr
        ipureintro; rfl
      ipureintro
      exact ⟨trivial, hp.2, trivial, hp.2⟩
  | @unsupported tagConsumed tagRest tag =>
    wp_apply wp_readReceiptStatus_tag_unsupported c dst old v xs wire tagRest dq tagConsumed kinds tag hp hbytes
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Hglobals Howned Hc Hs
    iapply HΦ
    iframe Hs
    isimp only [ReceiptStatusRecordOutcome, ReceiptStatusRecordError, Bool.false_eq_true, ite_false, hz]
    iframe Howned Hc
    ipureintro
    exact ⟨⟨trivial, Hnext.1⟩, Hnext.2.1, Hnext.2.2.1, Hnext.2.1, Hnext.2.2.1⟩
  | @rejected tagConsumed tagRest final consumed offset entered reason tag failure =>
    wp_apply wp_readReceiptStatus_record_body_rejected validator d c dst old final v xs wire tagRest dq
      tagConsumed consumed offset entered reason kinds tag failure hd hp hbytes
      $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Hglobals Howned Hc Hs
    ihave Herror := receiptStatusRecordError_diagnostic err (v.pos' + W64 (tagConsumed + offset)) reason kinds $$ Herr
    iapply HΦ
    iframe Hs
    isimp only [ReceiptStatusRecordOutcome, Int.natCast_add]
    iframe Howned Hc Herror
    ipureintro
    simpa only [Int.natCast_add, Int.add_assoc] using Hnext

/-- Complete record contract for every byte input and every owned initial value
    at a supported depth. Initialization supplies the actual error categories;
    classification is proved rather than assumed by the caller. -/
theorem wp_readReceiptStatus_record_complete (validator : Utf8ValidatorContract (GF := GF))
    (d : Nat) (c dst : Loc) (old : ReceiptStatusValue)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadReceiptExecutionStatusProjection)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (kinds : FeltErrorKinds) (outcome : ReceiptStatusRecordResult),
       FeltErrorGlobals kinds ∗ ⌜ReceiptStatusRecordSpec d old wire outcome⌝ ∗
       ReceiptStatusRecordOutcome c dst v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  obtain ⟨outcome, spec⟩ := receiptStatusRecord_classify d old wire depth
  wp_apply wp_readReceiptStatus_record_spec validator d c dst old v xs wire dq kinds outcome spec hd hp hbytes
    $$ [Howned Hc Hs] as %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Howned Hc Hs
  iapply HΦ
  iframe Hs
  iexists kinds, outcome
  iframe Hglobals Houtcome
  ipureintro
  exact spec

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptStatus_record_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptStatus_record_complete
